package pgcollect

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"mayfly-go/internal/db/dbm/dbi"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/infra/redaction"
)

const (
	CoverageOK           = "ok"
	CoverageUnsupported  = "unsupported"
	CoverageNoPermission = "no_permission"
	CoverageFail         = "fail"
)

// Row 源表一行（已按白名单提取）
type Row struct {
	RecordID string
	Fields   map[string]any
}

type Queryer interface {
	QueryContext(ctx context.Context, querySQL string, args ...any) ([]*dbi.QueryColumn, []map[string]any, error)
}

// Profile 默认对齐公开 Octop schema；部署版不一致时由实例 extra 覆盖列名。
type Profile struct {
	AuditTable      string
	UsageTable      string
	TrajectoryTable string
	IDColumn        string
	TimeColumn      string
}

func DefaultProfile() Profile {
	return Profile{
		AuditTable:      entity.SourceTableAuditLog,
		UsageTable:      entity.SourceTableUsageLog,
		TrajectoryTable: entity.SourceTableTrajectoryEvents,
		IDColumn:        "id",
		TimeColumn:      "created_at",
	}
}

type Reader struct {
	q       Queryer
	profile Profile
}

func NewReader(q Queryer, profile Profile) *Reader {
	if profile.IDColumn == "" {
		profile.IDColumn = "id"
	}
	if profile.TimeColumn == "" {
		profile.TimeColumn = "created_at"
	}
	return &Reader{q: q, profile: profile}
}

func (r *Reader) TableName(sourceTable string) string {
	switch sourceTable {
	case entity.SourceTableAuditLog:
		return first(r.profile.AuditTable, entity.SourceTableAuditLog)
	case entity.SourceTableUsageLog:
		return first(r.profile.UsageTable, entity.SourceTableUsageLog)
	case entity.SourceTableTrajectoryEvents:
		return first(r.profile.TrajectoryTable, entity.SourceTableTrajectoryEvents)
	default:
		return sourceTable
	}
}

// Probe 探测源表是否可读。失败时区分不支持/无权限/采集失败，绝不把错误当成 0 行。
func (r *Reader) Probe(ctx context.Context, sourceTable string) (coverage string, retentionStart string, err error) {
	table := quoteIdent(r.TableName(sourceTable))
	idCol := quoteIdent(r.profile.IDColumn)
	timeCol := quoteIdent(r.profile.TimeColumn)
	sql := fmt.Sprintf("SELECT MIN(%s) AS min_id, MIN(%s) AS min_time FROM %s", idCol, timeCol, table)
	_, rows, qerr := r.q.QueryContext(ctx, sql)
	// Octop deployments use ts, while older schemas expose created_at.
	// Only retry a missing time column; permission and connection errors must survive.
	if qerr != nil && r.profile.TimeColumn == "created_at" &&
		strings.Contains(strings.ToLower(qerr.Error()), "created_at") &&
		(strings.Contains(strings.ToLower(qerr.Error()), "does not exist") || strings.Contains(strings.ToLower(qerr.Error()), "undefined_column")) {
		sql = fmt.Sprintf("SELECT MIN(%s) AS min_id, MIN(%s) AS min_time FROM %s", idCol, quoteIdent("ts"), table)
		_, rows, qerr = r.q.QueryContext(ctx, sql)
	}
	if qerr != nil {
		return classifyErr(qerr), "", qerr
	}
	if len(rows) > 0 {
		if t := stringify(rows[0]["min_time"]); t != "" {
			retentionStart = t
		}
	}
	return CoverageOK, retentionStart, nil
}

func (r *Reader) ReadBatch(ctx context.Context, sourceTable, afterID string, limit int) ([]Row, error) {
	if limit <= 0 || limit > entity.DefaultCollectBatchSize {
		limit = entity.DefaultCollectBatchSize
	}
	table := quoteIdent(r.TableName(sourceTable))
	idCol := quoteIdent(r.profile.IDColumn)
	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s > $1 ORDER BY %s ASC LIMIT %d", table, idCol, idCol, limit)
	after := afterID
	if after == "" {
		after = "0"
	}
	_, rows, err := r.q.QueryContext(ctx, sql, after)
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(rows))
	for _, raw := range rows {
		fields := redaction.Extract(raw)
		id := first(
			stringify(raw[r.profile.IDColumn]),
			stringify(raw["id"]),
			redaction.String(fields, "id", "event_id"),
		)
		if id == "" {
			continue
		}
		out = append(out, Row{RecordID: id, Fields: fields})
	}
	return out, nil
}

func classifyErr(err error) string {
	if err == nil {
		return CoverageOK
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "does not exist"),
		strings.Contains(msg, "undefined_table"),
		strings.Contains(msg, "unknown table"),
		strings.Contains(msg, "unsupported"):
		return CoverageUnsupported
	case strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "not permission"),
		strings.Contains(msg, "access denied"),
		strings.Contains(msg, "insufficientprivilege"):
		return CoverageNoPermission
	default:
		return CoverageFail
	}
}

func quoteIdent(name string) string {
	n := strings.ReplaceAll(name, `"`, ``)
	parts := strings.Split(n, ".")
	for i, p := range parts {
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, ".")
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func first(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
