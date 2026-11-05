package application

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/errorx"
)

type Event interface {
	Query(ctx context.Context, q *entity.EventQuery) (*dto.EventPage, error)
	Summary(ctx context.Context, q *entity.EventQuery, groupBy string) ([]*repository.EventSummary, error)
	Export(ctx context.Context, q *entity.EventQuery) ([]byte, string, error)
	Recollect(ctx context.Context, instanceId uint64, sourceTable string) error
}

type eventAppImpl struct {
	eventRepo      repository.Event      `inject:"T"`
	checkpointRepo repository.Checkpoint `inject:"T"`
	instanceApp    Instance              `inject:"T"`
	collector      Collector             `inject:"T"`
}

var _ Event = (*eventAppImpl)(nil)

func (a *eventAppImpl) Query(ctx context.Context, q *entity.EventQuery) (*dto.EventPage, error) {
	allowed, err := a.instanceApp.VisibleInstanceIDs(ctx)
	if err != nil {
		return nil, err
	}
	if q.InstanceId > 0 && !containsID(allowed, q.InstanceId) {
		return nil, errorx.NewBizI(ctx, imsg.ErrEventNoPermission)
	}
	page, err := a.eventRepo.PageByQuery(q, allowed)
	if err != nil {
		return nil, err
	}
	gaps, err := a.gaps(ctx, q, allowed)
	if err != nil {
		return nil, err
	}
	return &dto.EventPage{
		Total: page.Total, List: page.List, PageNum: page.PageNum, PageSize: page.PageSize, Gaps: gaps,
	}, nil
}

func (a *eventAppImpl) Summary(ctx context.Context, q *entity.EventQuery, groupBy string) ([]*repository.EventSummary, error) {
	allowed, err := a.instanceApp.VisibleInstanceIDs(ctx)
	if err != nil {
		return nil, err
	}
	if q.InstanceId > 0 && !containsID(allowed, q.InstanceId) {
		return nil, errorx.NewBizI(ctx, imsg.ErrEventNoPermission)
	}
	return a.eventRepo.Summarize(q, allowed, groupBy)
}

func (a *eventAppImpl) Export(ctx context.Context, q *entity.EventQuery) ([]byte, string, error) {
	allowed, err := a.instanceApp.VisibleInstanceIDs(ctx)
	if err != nil {
		return nil, "", err
	}
	if q.InstanceId > 0 && !containsID(allowed, q.InstanceId) {
		return nil, "", errorx.NewBizI(ctx, imsg.ErrExportNoPermission)
	}
	rows, err := a.eventRepo.ListForExport(q, allowed, 2000)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"source_instance_id", "source_table", "source_record_id", "category", "action", "status",
		"octop_user_id", "octop_username", "casdoor_user_id", "identity_status",
		"agent_id", "agent_name", "thread_id", "event_id", "tool_name", "model",
		"input_tokens", "output_tokens", "duration_ms", "source_time", "collected_at", "source",
	})
	for _, row := range rows {
		_ = w.Write([]string{
			fmt.Sprint(row.SourceInstanceId), row.SourceTable, row.SourceRecordId, row.Category, row.Action, row.Status,
			row.OctopUserId, row.OctopUsername, row.CasdoorUserId, row.IdentityStatus,
			row.AgentId, row.AgentName, row.ThreadId, row.EventId, row.ToolName, row.Model,
			fmt.Sprint(row.InputTokens), fmt.Sprint(row.OutputTokens), fmt.Sprint(row.DurationMs),
			formatTime(row.SourceTime), row.CollectedAt.UTC().Format(time.RFC3339), row.Source,
		})
	}
	w.Flush()
	name := fmt.Sprintf("octop-events-%s.csv", time.Now().UTC().Format("20060102T150405Z"))
	return buf.Bytes(), name, w.Error()
}

func (a *eventAppImpl) Recollect(ctx context.Context, instanceId uint64, sourceTable string) error {
	inst, err := a.instanceApp.GetById(instanceId)
	if err != nil {
		return err
	}
	if err := a.instanceApp.AssertInstanceAccess(ctx, inst); err != nil {
		return err
	}
	return a.collector.CollectInstanceTables(ctx, inst, sourceTable)
}

func (a *eventAppImpl) gaps(ctx context.Context, q *entity.EventQuery, allowed []uint64) ([]*dto.EventGap, error) {
	_ = ctx
	ids := allowed
	if q.InstanceId > 0 {
		ids = []uint64{q.InstanceId}
	}
	out := make([]*dto.EventGap, 0)
	tables := []string{entity.SourceTableAuditLog, entity.SourceTableUsageLog, entity.SourceTableTrajectoryEvents}
	for _, id := range ids {
		for _, table := range tables {
			cp, err := a.checkpointRepo.GetByTable(id, table)
			if err != nil {
				return nil, err
			}
			if cp == nil {
				out = append(out, &dto.EventGap{InstanceId: id, SourceTable: table, Coverage: entity.GapUnsupported, Reason: "尚未采集"})
				continue
			}
			gap := &dto.EventGap{
				InstanceId: id, SourceTable: table, LastRecordId: cp.LastRecordId,
				LastSuccessAt: cp.LastSuccessAt, RetentionStart: cp.RetentionStart,
			}
			switch cp.Coverage {
			case "ok":
				if cp.LastError != "" {
					gap.Coverage = entity.GapCollectFail
					gap.Reason = cp.LastError
				} else {
					gap.Coverage = entity.GapNone
					gap.Reason = "该时段确无事件或已对齐游标"
				}
			case "unsupported", "no_permission":
				gap.Coverage = entity.GapUnsupported
				gap.Reason = firstNonEmpty(cp.LastError, "该类别不支持或无只读权限")
			default:
				gap.Coverage = entity.GapCollectFail
				gap.Reason = firstNonEmpty(cp.LastError, "采集失败")
			}
			if cp.RetentionStart != nil && q.StartTime > 0 && time.UnixMilli(q.StartTime).UTC().Before(*cp.RetentionStart) {
				gap.Coverage = entity.GapSourcePurged
				gap.Reason = "源端历史已清理，只能采集源端尚存的数据"
			}
			out = append(out, gap)
		}
	}
	return out, nil
}

func containsID(ids []uint64, id uint64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
