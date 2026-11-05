package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbapp "mayfly-go/internal/db/application"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/infra/isolate"
	"mayfly-go/internal/octop/infra/pgcollect"
	"mayfly-go/internal/octop/infra/redaction"
	"mayfly-go/pkg/logx"
	"mayfly-go/pkg/taskx"
)

const (
	eventCronKey     = "octop-event-collect"
	retentionCronKey = "octop-event-retention"
)

type Collector interface {
	InitCron()
	CollectAll(ctx context.Context) error
	CollectInstanceTables(ctx context.Context, inst *entity.OctopInstance, onlyTable string) error
}

type collectorAppImpl struct {
	instanceRepo   repository.Instance   `inject:"T"`
	eventRepo      repository.Event      `inject:"T"`
	checkpointRepo repository.Checkpoint `inject:"T"`
	runRepo        repository.CollectorRun `inject:"T"`
	linkRepo       repository.IdentityLink `inject:"T"`
	settingsRepo   repository.Settings   `inject:"T"`
	metricRepo     repository.MetricSample `inject:"T"`
	dbApp          dbapp.Db              `inject:"T"`
	txFn           func(ctx context.Context, fns ...func(context.Context) error) error
}

var _ Collector = (*collectorAppImpl)(nil)

func (c *collectorAppImpl) InitCron() {
	_ = taskx.BindCronTaskWithLock(eventCronKey, "0 */2 * * * *", 2*time.Minute, true, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		if err := c.CollectAll(ctx); err != nil {
			logx.ErrorTrace("octop event collect", err)
		}
	})
	_ = taskx.BindCronTaskWithLock(retentionCronKey, "0 20 3 * * *", time.Hour, true, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		settings, err := c.settingsRepo.GetOrCreate(ctx)
		if err != nil {
			logx.ErrorTrace("octop settings", err)
			return
		}
		days := settings.EventRetainDays
		if days <= 0 {
			days = entity.DefaultEventRetainDays
		}
		before := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
		if _, err := c.eventRepo.DeleteBefore(ctx, before); err != nil {
			logx.ErrorTrace("octop event retention", err)
		}
		if _, err := c.metricRepo.DeleteBefore(ctx, before); err != nil {
			logx.ErrorTrace("octop metric retention", err)
		}
	})
}

func (c *collectorAppImpl) CollectAll(ctx context.Context) error {
	settings, err := c.settingsRepo.GetOrCreate(ctx)
	if err != nil {
		return err
	}
	list, err := c.instanceRepo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	isolate.Run(ctx, list, settings.CollectConcurrency, time.Duration(settings.CollectTimeoutSec)*time.Second, func(ic context.Context, inst *entity.OctopInstance) error {
		return c.CollectInstanceTables(ic, inst, "")
	})
	return nil
}

func (c *collectorAppImpl) CollectInstanceTables(ctx context.Context, inst *entity.OctopInstance, onlyTable string) error {
	run := &entity.CollectorRun{InstanceId: inst.Id, Kind: "event", Status: entity.CollectStatusRunning, StartedAt: time.Now().UTC()}
	if err := c.runRepo.Insert(ctx, run); err != nil {
		return err
	}
	tables := []string{entity.SourceTableAuditLog, entity.SourceTableUsageLog, entity.SourceTableTrajectoryEvents}
	if onlyTable != "" {
		tables = []string{onlyTable}
	}
	var lastErr error
	inserted, skipped := 0, 0
	for _, table := range tables {
		n, s, err := c.collectTable(ctx, inst, table)
		inserted += n
		skipped += s
		if err != nil {
			lastErr = err
		}
	}
	now := time.Now().UTC()
	run.FinishedAt = &now
	run.Inserted = inserted
	run.Skipped = skipped
	if lastErr != nil {
		run.Status = entity.CollectStatusPartial
		run.Error = trimErr(lastErr)
	} else {
		run.Status = entity.CollectStatusOK
	}
	if err := c.runRepo.UpdateById(ctx, run); err != nil {
		return err
	}
	return lastErr
}

func (c *collectorAppImpl) collectTable(ctx context.Context, inst *entity.OctopInstance, table string) (inserted, skipped int, err error) {
	cp, err := c.ensureCheckpoint(ctx, inst.Id, table)
	if err != nil {
		return 0, 0, err
	}
	if cp.BackoffUntil != nil && time.Now().Before(*cp.BackoffUntil) {
		return 0, 0, nil
	}
	if inst.DbId == 0 || inst.DbName == "" {
		return 0, 0, c.failCheckpoint(ctx, cp, pgcollect.CoverageUnsupported, "未关联 PostgreSQL")
	}
	conn, err := c.dbApp.GetDbConn(ctx, inst.DbId, inst.DbName)
	if err != nil {
		return 0, 0, c.failCheckpoint(ctx, cp, pgcollect.CoverageFail, err.Error())
	}
	reader := pgcollect.NewReader(conn, pgcollect.DefaultProfile())
	coverage, retention, perr := reader.Probe(ctx, table)
	if perr != nil {
		return 0, 0, c.failCheckpoint(ctx, cp, coverage, perr.Error())
	}
	cp.Coverage = coverage
	if t, err := time.Parse(time.RFC3339, retention); err == nil {
		cp.RetentionStart = &t
	} else if retention != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", retention); err == nil {
			utc := t.UTC()
			cp.RetentionStart = &utc
		}
	}

	after := cp.LastRecordId
	for {
		batch, rerr := reader.ReadBatch(ctx, table, after, entity.DefaultCollectBatchSize)
		if rerr != nil {
			return inserted, skipped, c.failCheckpoint(ctx, cp, pgcollect.CoverageFail, rerr.Error())
		}
		if len(batch) == 0 {
			break
		}
		ids := make([]string, 0, len(batch))
		for _, row := range batch {
			ids = append(ids, row.RecordID)
		}
		exist, eerr := c.eventRepo.ExistingRecordIDs(ctx, inst.Id, table, ids)
		if eerr != nil {
			return inserted, skipped, eerr
		}
		events := make([]*entity.OctopEvent, 0, len(batch))
		maxID := after
		for _, row := range batch {
			if compareRecordID(row.RecordID, maxID) > 0 {
				maxID = row.RecordID
			}
			if _, ok := exist[row.RecordID]; ok {
				skipped++
				continue
			}
			ev := mapEvent(inst, table, row)
			c.fillIdentity(ev)
			events = append(events, ev)
		}
		if err := c.persistBatch(ctx, cp, events, maxID); err != nil {
			return inserted, skipped, err
		}
		inserted += len(events)
		after = maxID
		if len(batch) < entity.DefaultCollectBatchSize {
			break
		}
	}
	now := time.Now().UTC()
	cp.LastSuccessAt = &now
	cp.LastError = ""
	cp.FailCount = 0
	cp.BackoffUntil = nil
	cp.Coverage = pgcollect.CoverageOK
	return inserted, skipped, c.checkpointRepo.UpdateById(ctx, cp)
}

func (c *collectorAppImpl) persistBatch(ctx context.Context, cp *entity.CollectorCheckpoint, events []*entity.OctopEvent, maxID string) error {
	fn := c.txFn
	if fn == nil {
		fn = GetInstanceApp().Tx
	}
	return fn(ctx, func(txCtx context.Context) error {
		if len(events) > 0 {
			if err := c.eventRepo.BatchInsert(txCtx, events); err != nil {
				return err
			}
		}
		cp.LastRecordId = maxID
		return c.checkpointRepo.UpdateById(txCtx, cp)
	})
}

func (c *collectorAppImpl) ensureCheckpoint(ctx context.Context, instanceId uint64, table string) (*entity.CollectorCheckpoint, error) {
	cp, err := c.checkpointRepo.GetByTable(instanceId, table)
	if err != nil {
		return nil, err
	}
	if cp != nil {
		return cp, nil
	}
	cp = &entity.CollectorCheckpoint{InstanceId: instanceId, SourceTable: table, LastRecordId: "0"}
	if err := c.checkpointRepo.Insert(ctx, cp); err != nil {
		return nil, err
	}
	return cp, nil
}

func (c *collectorAppImpl) failCheckpoint(ctx context.Context, cp *entity.CollectorCheckpoint, coverage, msg string) error {
	now := time.Now().UTC()
	cp.Coverage = coverage
	cp.LastError = trimStr(msg, 250)
	cp.FailCount++
	backoff := time.Duration(entity.DefaultBackoffSec*(1<<min(cp.FailCount, 3))) * time.Second
	if backoff > time.Duration(entity.DefaultMaxBackoffSec)*time.Second {
		backoff = time.Duration(entity.DefaultMaxBackoffSec) * time.Second
	}
	until := now.Add(backoff)
	cp.BackoffUntil = &until
	if err := c.checkpointRepo.UpdateById(ctx, cp); err != nil {
		return err
	}
	return fmt.Errorf("collect %s: %s", cp.SourceTable, msg)
}

func (c *collectorAppImpl) fillIdentity(ev *entity.OctopEvent) {
	ev.IdentityStatus = entity.IdentityUnlinked
	if ev.OctopUserId == "" {
		return
	}
	link, err := c.linkRepo.GetByOctopUser(ev.SourceInstanceId, ev.OctopUserId)
	if err != nil || link == nil {
		return
	}
	ev.IdentityStatus = entity.IdentityLinked
	ev.CasdoorUserId = link.CasdoorUserSourceId
}

func mapEvent(inst *entity.OctopInstance, table string, row pgcollect.Row) *entity.OctopEvent {
	now := time.Now().UTC()
	ev := &entity.OctopEvent{
		SourceInstanceId: inst.Id,
		SourceTable:      table,
		SourceRecordId:   row.RecordID,
		Source:           "octop",
		CollectedAt:      now,
		IdentityStatus:   entity.IdentityUnlinked,
	}
	f := row.Fields
	ev.OctopUserId = redaction.String(f, "user_id", "userid")
	ev.OctopUsername = redaction.String(f, "username", "user_name")
	ev.AgentId = redaction.String(f, "agent_id", "agentid")
	ev.AgentName = redaction.String(f, "agent_name", "agentname")
	ev.ThreadId = redaction.String(f, "thread_id", "threadid")
	ev.TurnId = redaction.String(f, "turn_id", "turnid")
	ev.EventId = redaction.String(f, "event_id", "eventid")
	ev.Action = redaction.String(f, "action", "event_type", "step_type")
	ev.Target = redaction.String(f, "target")
	ev.ToolName = redaction.String(f, "tool_name", "toolname")
	ev.Model = redaction.String(f, "model")
	ev.Status = redaction.String(f, "status")
	if ev.Status == "" {
		if redaction.String(f, "success") == "true" || redaction.String(f, "success") == "1" {
			ev.Status = "ok"
		}
	}
	ev.InputTokens = redaction.Int64(f, "input_tokens", "inputtokens")
	ev.OutputTokens = redaction.Int64(f, "output_tokens", "outputtokens")
	ev.DurationMs = redaction.Int64(f, "duration_ms", "durationms", "duration", "latency_ms")
	ev.Category = categoryOf(table, ev.Action)
	if ts := redaction.String(f, "created_at", "createdat", "timestamp", "ts"); ts != "" {
		if t, ok := parseSourceTime(ts); ok {
			ev.SourceTime = &t
		} else {
			ev.SourceTimeAbnormal = 1
			ev.Diagnostic = "源时间异常"
		}
	} else {
		ev.SourceTimeAbnormal = 1
	}
	if ev.Action == "" && ev.ToolName == "" && ev.Status == "" {
		ev.Uncollected = 1
		ev.Diagnostic = redaction.Uncollected
	}
	return ev
}

func categoryOf(table, action string) string {
	switch table {
	case entity.SourceTableUsageLog:
		return entity.EventCall
	case entity.SourceTableTrajectoryEvents:
		return entity.EventTrajectory
	default:
		al := strings.ToLower(action)
		if strings.Contains(al, "login") || strings.Contains(al, "logout") || strings.Contains(al, "auth.") {
			return entity.EventLogin
		}
		return entity.EventOperation
	}
}

func parseSourceTime(s string) (time.Time, bool) {
	layouts := []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), true
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1e12 {
			return time.UnixMilli(n).UTC(), true
		}
		if n > 1e9 {
			return time.Unix(n, 0).UTC(), true
		}
	}
	return time.Time{}, false
}

func compareRecordID(a, b string) int {
	ai, aerr := strconv.ParseUint(a, 10, 64)
	bi, berr := strconv.ParseUint(b, 10, 64)
	if aerr == nil && berr == nil {
		switch {
		case ai > bi:
			return 1
		case ai < bi:
			return -1
		default:
			return 0
		}
	}
	return strings.Compare(a, b)
}

func trimStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
