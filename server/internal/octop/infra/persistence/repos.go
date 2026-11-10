package persistence

import (
	"context"
	"strings"
	"time"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/model"
)

type instanceRepoImpl struct {
	base.RepoImpl[*entity.OctopInstance]
}

func newInstanceRepo() repository.Instance {
	return &instanceRepoImpl{}
}

func (r *instanceRepoImpl) ListEnabled(ctx context.Context) ([]*entity.OctopInstance, error) {
	_ = ctx
	return r.SelectByCond(model.NewCond().Eq("collect_enabled", 1))
}

type casdoorConnRepoImpl struct {
	base.RepoImpl[*entity.CasdoorConnection]
}

func newCasdoorConnectionRepo() repository.CasdoorConnection {
	return &casdoorConnRepoImpl{}
}

type casdoorAppRepoImpl struct {
	base.RepoImpl[*entity.CasdoorApplicationSnapshot]
}

func newCasdoorAppRepo() repository.CasdoorApp {
	return &casdoorAppRepoImpl{}
}

func (r *casdoorAppRepoImpl) GetBySource(connectionId uint64, sourceId string) (*entity.CasdoorApplicationSnapshot, error) {
	var row entity.CasdoorApplicationSnapshot
	err := r.GetByCond(model.NewCond().Eq("connection_id", connectionId).Eq("source_id", sourceId).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *casdoorAppRepoImpl) MarkMissingAbsent(ctx context.Context, connectionId uint64, presentSourceIds []string, now time.Time) error {
	rows, err := r.SelectByCond(model.NewCond().Eq("connection_id", connectionId).Eq("present", 1))
	if err != nil {
		return err
	}
	keep := toSet(presentSourceIds)
	for _, row := range rows {
		if _, ok := keep[row.SourceId]; ok {
			continue
		}
		row.Present = -1
		row.Enabled = -1
		row.LastSyncAt = &now
		if err := r.UpdateById(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

type casdoorUserRepoImpl struct {
	base.RepoImpl[*entity.CasdoorUserSnapshot]
}

func newCasdoorUserRepo() repository.CasdoorUser {
	return &casdoorUserRepoImpl{}
}

func (r *casdoorUserRepoImpl) GetBySource(connectionId uint64, sourceId string) (*entity.CasdoorUserSnapshot, error) {
	var row entity.CasdoorUserSnapshot
	err := r.GetByCond(model.NewCond().Eq("connection_id", connectionId).Eq("source_id", sourceId).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *casdoorUserRepoImpl) MarkMissingDisabled(ctx context.Context, connectionId uint64, presentSourceIds []string, now time.Time) error {
	rows, err := r.SelectByCond(model.NewCond().Eq("connection_id", connectionId).Eq("present", 1))
	if err != nil {
		return err
	}
	keep := toSet(presentSourceIds)
	for _, row := range rows {
		if _, ok := keep[row.SourceId]; ok {
			continue
		}
		row.Present = -1
		row.Enabled = -1
		row.LastSyncAt = &now
		if err := r.UpdateById(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

type casdoorAuthRepoImpl struct {
	base.RepoImpl[*entity.CasdoorAuthRelation]
}

func newCasdoorAuthRepo() repository.CasdoorAuth {
	return &casdoorAuthRepoImpl{}
}

func (r *casdoorAuthRepoImpl) GetByKey(connectionId uint64, appSourceId, subjectType, subjectSourceId string) (*entity.CasdoorAuthRelation, error) {
	var row entity.CasdoorAuthRelation
	err := r.GetByCond(model.NewCond().
		Eq("connection_id", connectionId).
		Eq("application_source_id", appSourceId).
		Eq("subject_type", subjectType).
		Eq("subject_source_id", subjectSourceId).
		Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *casdoorAuthRepoImpl) MarkMissingRevoked(ctx context.Context, connectionId uint64, verifiedAppIDs []string, keepKeys []repository.AuthKey, now time.Time) error {
	if len(verifiedAppIDs) == 0 {
		return nil
	}
	rows, err := r.SelectByCond(model.NewCond().
		Eq("connection_id", connectionId).
		Eq("status", entity.AuthConfirmed).
		In("application_source_id", verifiedAppIDs))
	if err != nil {
		return err
	}
	keep := make(map[string]struct{}, len(keepKeys))
	for _, k := range keepKeys {
		keep[k.AppSourceId+"|"+k.SubjectType+"|"+k.SubjectSourceId] = struct{}{}
	}
	verified := toSet(verifiedAppIDs)
	for _, row := range rows {
		if _, ok := verified[row.ApplicationSourceId]; !ok {
			continue
		}
		key := row.ApplicationSourceId + "|" + row.SubjectType + "|" + row.SubjectSourceId
		if _, ok := keep[key]; ok {
			continue
		}
		row.Status = entity.AuthRevoked
		row.LastSyncAt = &now
		if err := r.UpdateById(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

type identityLinkRepoImpl struct {
	base.RepoImpl[*entity.OctopIdentityLink]
}

func newIdentityLinkRepo() repository.IdentityLink {
	return &identityLinkRepoImpl{}
}

func (r *identityLinkRepoImpl) GetByOctopUser(instanceId uint64, octopUserId string) (*entity.OctopIdentityLink, error) {
	var row entity.OctopIdentityLink
	err := r.GetByCond(model.NewCond().Eq("octop_instance_id", instanceId).Eq("octop_user_id", octopUserId).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *identityLinkRepoImpl) GetByCasdoorUser(connectionId uint64, casdoorSourceId string, instanceId uint64) (*entity.OctopIdentityLink, error) {
	var row entity.OctopIdentityLink
	err := r.GetByCond(model.NewCond().
		Eq("connection_id", connectionId).
		Eq("casdoor_user_source_id", casdoorSourceId).
		Eq("octop_instance_id", instanceId).
		Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

type eventRepoImpl struct {
	base.RepoImpl[*entity.OctopEvent]
}

func newEventRepo() repository.Event {
	return &eventRepoImpl{}
}

func (r *eventRepoImpl) ExistingRecordIDs(ctx context.Context, instanceId uint64, sourceTable string, recordIDs []string) (map[string]struct{}, error) {
	_ = ctx
	out := make(map[string]struct{}, len(recordIDs))
	if len(recordIDs) == 0 {
		return out, nil
	}
	rows, err := r.SelectByCond(model.NewCond().
		Eq("source_instance_id", instanceId).
		Eq("source_table", sourceTable).
		In("source_record_id", recordIDs).
		Columns("id", "source_record_id"))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SourceRecordId] = struct{}{}
	}
	return out, nil
}

func eventCond(q *entity.EventQuery, allowed []uint64) *model.QueryCond {
	qd := model.NewCond()
	if len(allowed) == 0 {
		qd.EqStrict("source_instance_id", uint64(0))
		return qd
	}
	ids := allowed
	if q.InstanceId > 0 {
		ids = intersectIDs(allowed, []uint64{q.InstanceId})
	} else if len(q.InstanceIds) > 0 {
		ids = intersectIDs(allowed, q.InstanceIds)
	}
	if len(ids) == 0 {
		qd.EqStrict("source_instance_id", uint64(0))
		return qd
	}
	qd.In("source_instance_id", ids)
	qd.Eq("category", q.Category)
	qd.Eq("action", q.Action)
	qd.Eq("status", q.Status)
	qd.Eq("octop_user_id", q.OctopUserId)
	qd.Eq("casdoor_user_id", q.CasdoorUserId)
	qd.Eq("agent_id", q.AgentId)
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		qd.And("(octop_username LIKE ? OR agent_name LIKE ? OR action LIKE ? OR target LIKE ? OR tool_name LIKE ?)", like, like, like, like, like)
	}
	if q.StartTime > 0 {
		qd.Ge("source_time", time.UnixMilli(q.StartTime).UTC())
	}
	if q.EndTime > 0 {
		qd.Le("source_time", time.UnixMilli(q.EndTime).UTC())
	}
	return qd
}

func (r *eventRepoImpl) PageByQuery(q *entity.EventQuery, allowed []uint64) (*model.PageResult[*entity.OctopEvent], error) {
	return r.PageByCond(eventCond(q, allowed).OrderByDesc("source_time"), q.PageParam)
}

func (r *eventRepoImpl) ListForExport(q *entity.EventQuery, allowed []uint64, limit int) ([]*entity.OctopEvent, error) {
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}
	page, err := r.PageByCond(eventCond(q, allowed).OrderByDesc("source_time"), model.PageParam{PageNum: 1, PageSize: limit})
	if err != nil {
		return nil, err
	}
	return page.List, nil
}

func (r *eventRepoImpl) Summarize(q *entity.EventQuery, allowed []uint64, groupBy string) ([]*repository.EventSummary, error) {
	col := "octop_user_id"
	nameCol := "octop_username"
	if groupBy == "agent" {
		col = "agent_id"
		nameCol = "agent_name"
	}
	ids := allowed
	if q.InstanceId > 0 {
		ids = intersectIDs(allowed, []uint64{q.InstanceId})
	}
	if len(ids) == 0 {
		return []*repository.EventSummary{}, nil
	}
	args := make([]any, 0, 8)
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(col)
	b.WriteString(" AS group_key, MAX(")
	b.WriteString(nameCol)
	b.WriteString(") AS display_name, COUNT(*) AS total, SUM(CASE WHEN status IN ('fail','failed','error') THEN 1 ELSE 0 END) AS fail_count, COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens, MAX(source_time) AS last_activity FROM ")
	b.WriteString((&entity.OctopEvent{}).TableName())
	b.WriteString(" WHERE source_instance_id IN (")
	for i, id := range ids {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("?")
		args = append(args, id)
	}
	b.WriteString(")")
	if q.Category != "" {
		b.WriteString(" AND category = ?")
		args = append(args, q.Category)
	}
	if q.StartTime > 0 {
		b.WriteString(" AND source_time >= ?")
		args = append(args, time.UnixMilli(q.StartTime).UTC())
	}
	if q.EndTime > 0 {
		b.WriteString(" AND source_time <= ?")
		args = append(args, time.UnixMilli(q.EndTime).UTC())
	}
	b.WriteString(" AND ")
	b.WriteString(col)
	b.WriteString(" <> '' GROUP BY ")
	b.WriteString(col)
	b.WriteString(" ORDER BY last_activity DESC")

	type row struct {
		GroupKey     string     `gorm:"column:group_key"`
		DisplayName  string     `gorm:"column:display_name"`
		Total        int64      `gorm:"column:total"`
		FailCount    int64      `gorm:"column:fail_count"`
		InputTokens  int64      `gorm:"column:input_tokens"`
		OutputTokens int64      `gorm:"column:output_tokens"`
		LastActivity *time.Time `gorm:"column:last_activity"`
	}
	var rows []row
	if err := r.SelectBySQL(context.Background(), b.String(), &rows, args...); err != nil {
		return nil, err
	}
	out := make([]*repository.EventSummary, 0, len(rows))
	for i := range rows {
		if i >= 200 {
			break
		}
		item := rows[i]
		out = append(out, &repository.EventSummary{
			GroupKey:     item.GroupKey,
			DisplayName:  item.DisplayName,
			Total:        item.Total,
			FailCount:    item.FailCount,
			InputTokens:  item.InputTokens,
			OutputTokens: item.OutputTokens,
			LastActivity: item.LastActivity,
		})
	}
	return out, nil
}

func (r *eventRepoImpl) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	if err := r.DeleteByCond(ctx, model.NewCond().Lt("collected_at", before)); err != nil {
		return 0, err
	}
	return 0, nil
}

type checkpointRepoImpl struct {
	base.RepoImpl[*entity.CollectorCheckpoint]
}

func newCheckpointRepo() repository.Checkpoint {
	return &checkpointRepoImpl{}
}

func (r *checkpointRepoImpl) GetByTable(instanceId uint64, sourceTable string) (*entity.CollectorCheckpoint, error) {
	var row entity.CollectorCheckpoint
	err := r.GetByCond(model.NewCond().Eq("instance_id", instanceId).Eq("source_table", sourceTable).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

type collectorRunRepoImpl struct {
	base.RepoImpl[*entity.CollectorRun]
}

func newCollectorRunRepo() repository.CollectorRun {
	return &collectorRunRepoImpl{}
}

func (r *collectorRunRepoImpl) RecentByInstance(instanceId uint64, kind string, limit int) ([]*entity.CollectorRun, error) {
	if limit <= 0 {
		limit = 20
	}
	qd := model.NewCond().Eq("instance_id", instanceId).OrderByDesc("started_at")
	if kind != "" {
		qd.Eq("kind", kind)
	}
	page, err := r.PageByCond(qd, model.PageParam{PageNum: 1, PageSize: limit})
	if err != nil {
		return nil, err
	}
	return page.List, nil
}

type metricSampleRepoImpl struct {
	base.RepoImpl[*entity.OctopMetricSample]
}

func newMetricSampleRepo() repository.MetricSample {
	return &metricSampleRepoImpl{}
}

func (r *metricSampleRepoImpl) GetRange(instanceId uint64, start, end time.Time) ([]*entity.OctopMetricSample, error) {
	return r.SelectByCond(model.NewCond().
		Eq("instance_id", instanceId).
		Ge("collected_at", start).
		Le("collected_at", end).
		OrderByAsc("collected_at"))
}

func (r *metricSampleRepoImpl) Latest(instanceId uint64) (*entity.OctopMetricSample, error) {
	page, err := r.PageByCond(model.NewCond().Eq("instance_id", instanceId).OrderByDesc("collected_at"), model.PageParam{PageNum: 1, PageSize: 1})
	if err != nil {
		return nil, err
	}
	if len(page.List) == 0 {
		return nil, nil
	}
	return page.List[0], nil
}

func (r *metricSampleRepoImpl) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	if err := r.DeleteByCond(ctx, model.NewCond().Lt("collected_at", before)); err != nil {
		return 0, err
	}
	return 0, nil
}

type settingsRepoImpl struct {
	base.RepoImpl[*entity.OctopSettings]
}

func newSettingsRepo() repository.Settings {
	return &settingsRepoImpl{}
}

func (r *settingsRepoImpl) GetOrCreate(ctx context.Context) (*entity.OctopSettings, error) {
	page, err := r.PageByCond(model.NewCond(), model.PageParam{PageNum: 1, PageSize: 1})
	if err != nil {
		return nil, err
	}
	if len(page.List) > 0 {
		return page.List[0], nil
	}
	row := &entity.OctopSettings{
		EventRetainDays:    entity.DefaultEventRetainDays,
		MetricSampleSec:    entity.DefaultSampleIntervalSec,
		CollectConcurrency: entity.DefaultCollectConcurrency,
		CollectTimeoutSec:  entity.DefaultCollectTimeoutSec,
	}
	if err := r.Insert(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

type readConnectionRepoImpl struct {
	base.RepoImpl[*entity.OctopReadConnection]
}

func newReadConnectionRepo() repository.ReadConnection {
	return &readConnectionRepoImpl{}
}

func (r *readConnectionRepoImpl) GetByInstanceType(ctx context.Context, instanceId uint64, connType string) (*entity.OctopReadConnection, error) {
	_ = ctx
	var row entity.OctopReadConnection
	err := r.GetByCond(model.NewCond().Eq("instance_id", instanceId).Eq("type", connType).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *readConnectionRepoImpl) ListByInstance(ctx context.Context, instanceId uint64) ([]*entity.OctopReadConnection, error) {
	_ = ctx
	return r.SelectByCond(model.NewCond().Eq("instance_id", instanceId))
}

func toSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func intersectIDs(allowed, requested []uint64) []uint64 {
	allow := make(map[uint64]struct{}, len(allowed))
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
