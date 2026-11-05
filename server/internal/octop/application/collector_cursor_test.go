package application

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"mayfly-go/internal/db/dbm/dbi"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/infra/pgcollect"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/model"

	"github.com/stretchr/testify/require"
)

type memQueryer struct {
	rows []map[string]any
}

func (m *memQueryer) QueryContext(_ context.Context, _ string, args ...any) ([]*dbi.QueryColumn, []map[string]any, error) {
	after := "0"
	if len(args) > 0 {
		after = fmt.Sprint(args[0])
	}
	afterN, _ := strconv.ParseInt(after, 10, 64)
	out := make([]map[string]any, 0)
	for _, row := range m.rows {
		id := asInt64(row["id"])
		if id > afterN {
			out = append(out, row)
		}
		if len(out) >= entity.DefaultCollectBatchSize {
			break
		}
	}
	return nil, out, nil
}

type testEventRepo struct{ base.RepoImpl[*entity.OctopEvent] }

func (r *testEventRepo) existing(instanceId uint64, table string, ids []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.SelectByCond(model.NewCond().Eq("source_instance_id", instanceId).Eq("source_table", table).In("source_record_id", ids).Columns("id", "source_record_id"))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SourceRecordId] = struct{}{}
	}
	return out, nil
}

func (r *testEventRepo) pageAllowed(q *entity.EventQuery, allowed []uint64) (*model.PageResult[*entity.OctopEvent], error) {
	qd := model.NewCond()
	if len(allowed) == 0 {
		qd.EqStrict("source_instance_id", uint64(0))
	} else {
		ids := allowed
		if q.InstanceId > 0 {
			ids = intersectUint64(allowed, []uint64{q.InstanceId})
		}
		if len(ids) == 0 {
			qd.EqStrict("source_instance_id", uint64(0))
		} else {
			qd.In("source_instance_id", ids)
		}
	}
	return r.PageByCond(qd, q.PageParam)
}

type testCheckpointRepo struct{ base.RepoImpl[*entity.CollectorCheckpoint] }

func TestCursorResumeSameSecondAndIdempotentRecollect(t *testing.T) {
	setupOctopDB(t)
	same := time.Now().UTC().Format(time.RFC3339)
	src := &memQueryer{rows: make([]map[string]any, 0, 520)}
	for i := 1; i <= 520; i++ {
		src.rows = append(src.rows, map[string]any{"id": i, "user_id": "u-1", "action": "auth.login", "created_at": same})
	}
	reader := pgcollect.NewReader(src, pgcollect.DefaultProfile())
	eventRepo := &testEventRepo{}
	cpRepo := &testCheckpointRepo{}
	inst := &entity.OctopInstance{}
	inst.Id = 9
	ctx := context.Background()
	cp := &entity.CollectorCheckpoint{InstanceId: 9, SourceTable: entity.SourceTableAuditLog, LastRecordId: "0"}
	require.NoError(t, cpRepo.Insert(ctx, cp))

	inserted := 0
	after := "0"
	for {
		batch, err := reader.ReadBatch(ctx, entity.SourceTableAuditLog, after, entity.DefaultCollectBatchSize)
		require.NoError(t, err)
		if len(batch) == 0 {
			break
		}
		ids := make([]string, 0, len(batch))
		for _, row := range batch {
			ids = append(ids, row.RecordID)
		}
		exist, err := eventRepo.existing(inst.Id, entity.SourceTableAuditLog, ids)
		require.NoError(t, err)
		events := make([]*entity.OctopEvent, 0)
		maxID := after
		for _, row := range batch {
			if compareRecordID(row.RecordID, maxID) > 0 {
				maxID = row.RecordID
			}
			if _, ok := exist[row.RecordID]; ok {
				continue
			}
			events = append(events, mapEvent(inst, entity.SourceTableAuditLog, row))
		}
		if len(events) > 0 {
			require.NoError(t, eventRepo.BatchInsert(ctx, events))
		}
		cp.LastRecordId = maxID
		require.NoError(t, cpRepo.UpdateById(ctx, cp))
		inserted += len(events)
		after = maxID
		if len(batch) < entity.DefaultCollectBatchSize {
			break
		}
	}
	require.Equal(t, 520, inserted)
	require.Equal(t, "520", cp.LastRecordId)

	after = "0"
	dup := 0
	for {
		batch, err := reader.ReadBatch(ctx, entity.SourceTableAuditLog, after, entity.DefaultCollectBatchSize)
		require.NoError(t, err)
		if len(batch) == 0 {
			break
		}
		ids := make([]string, 0, len(batch))
		for _, row := range batch {
			ids = append(ids, row.RecordID)
		}
		exist, err := eventRepo.existing(inst.Id, entity.SourceTableAuditLog, ids)
		require.NoError(t, err)
		for _, row := range batch {
			if _, ok := exist[row.RecordID]; !ok {
				dup++
			}
			if compareRecordID(row.RecordID, after) > 0 {
				after = row.RecordID
			}
		}
		if len(batch) < entity.DefaultCollectBatchSize {
			break
		}
	}
	require.Equal(t, 0, dup)

	page, err := eventRepo.pageAllowed(&entity.EventQuery{PageParam: model.PageParam{PageNum: 1, PageSize: 10}}, []uint64{9})
	require.NoError(t, err)
	require.Equal(t, int64(520), page.Total)
	require.Equal(t, entity.EventLogin, page.List[0].Category)
}

func TestEventQueryCrossInstanceIsolation(t *testing.T) {
	setupOctopDB(t)
	repo := &testEventRepo{}
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repo.BatchInsert(ctx, []*entity.OctopEvent{
		{SourceInstanceId: 1, SourceTable: "audit_log", SourceRecordId: "1", Category: entity.EventLogin, CollectedAt: now, SourceTime: &now},
		{SourceInstanceId: 2, SourceTable: "audit_log", SourceRecordId: "2", Category: entity.EventLogin, CollectedAt: now, SourceTime: &now},
	}))

	page, err := repo.pageAllowed(&entity.EventQuery{PageParam: model.PageParam{PageNum: 1, PageSize: 20}, InstanceId: 2}, []uint64{1})
	require.NoError(t, err)
	require.Equal(t, int64(0), page.Total)

	page, err = repo.pageAllowed(&entity.EventQuery{PageParam: model.PageParam{PageNum: 1, PageSize: 20}}, []uint64{1})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, uint64(1), page.List[0].SourceInstanceId)

	page, err = repo.pageAllowed(&entity.EventQuery{PageParam: model.PageParam{PageNum: 1, PageSize: 20}}, nil)
	require.NoError(t, err)
	require.Equal(t, int64(0), page.Total)
}

func TestStatusUnknownOnCollectorFailure(t *testing.T) {
	status, reason := aggregateHealth(&entity.OctopInstance{}, probeFacts{collectFailed: true, collectError: "凭据失效"})
	require.Equal(t, entity.HealthUnknown, status)
	require.Equal(t, "凭据失效", reason)

	status, reason = aggregateHealth(&entity.OctopInstance{HttpFailThreshold: 3}, probeFacts{octopProcess: entity.ProbeFail})
	require.Equal(t, entity.HealthCritical, status)
	require.Contains(t, reason, "进程停止")
}

func intersectUint64(allowed, requested []uint64) []uint64 {
	allow := map[uint64]struct{}{}
	for _, id := range allowed {
		allow[id] = struct{}{}
	}
	out := make([]uint64, 0, len(requested))
	for _, id := range requested {
		if _, ok := allow[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func TestSustainedThreshold(t *testing.T) {
	require.False(t, sustainedOver([]float64{90, 90}, 85, 5*time.Minute, 2*time.Minute))
	require.True(t, sustainedOver([]float64{90, 91, 92}, 85, 5*time.Minute, 2*time.Minute))
	require.False(t, sustainedOver([]float64{90, 70, 92}, 85, 5*time.Minute, 2*time.Minute))
}
