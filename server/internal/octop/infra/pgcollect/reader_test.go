package pgcollect

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"mayfly-go/internal/db/dbm/dbi"
	"mayfly-go/internal/octop/domain/entity"
)

type schemaQueryer struct {
	queries    []string
	ts         bool
	permission bool
}

func (q *schemaQueryer) QueryContext(_ context.Context, sql string, args ...any) ([]*dbi.QueryColumn, []map[string]any, error) {
	q.queries = append(q.queries, sql)
	if q.permission {
		return nil, nil, errors.New("permission denied")
	}
	if strings.Contains(sql, "MIN(") {
		if strings.Contains(sql, `"created_at"`) && q.ts {
			return nil, nil, errors.New(`pq: column "created_at" does not exist`)
		}
		return nil, []map[string]any{{"min_id": int64(1), "min_time": 1793872991.25}}, nil
	}
	if !strings.Contains(sql, "$1") {
		return nil, nil, errors.New("PostgreSQL requires a numbered parameter")
	}
	return nil, []map[string]any{{"id": int64(4), "ts": 1793872991.25, "action": "login"}}, nil
}

func TestProbeSupportsOctopTimestampColumnAndPrecision(t *testing.T) {
	q := &schemaQueryer{ts: true}
	coverage, retention, err := NewReader(q, DefaultProfile()).Probe(context.Background(), entity.SourceTableAuditLog)
	require.NoError(t, err)
	require.Equal(t, CoverageOK, coverage)
	require.Equal(t, "1793872991.25", retention)
	require.Len(t, q.queries, 2)
}
func TestProbeDoesNotMaskPermissionError(t *testing.T) {
	q := &schemaQueryer{permission: true}
	coverage, _, err := NewReader(q, DefaultProfile()).Probe(context.Background(), entity.SourceTableAuditLog)
	require.Error(t, err)
	require.Equal(t, CoverageNoPermission, coverage)
	require.Len(t, q.queries, 1)
}
func TestReadBatchUsesPostgresParameter(t *testing.T) {
	q := &schemaQueryer{}
	rows, err := NewReader(q, DefaultProfile()).ReadBatch(context.Background(), entity.SourceTableUsageLog, "0", 10)
	require.NoError(t, err)
	require.Equal(t, "4", rows[0].RecordID)
}

func TestProbeLegacyCreatedAt(t *testing.T) {
	q := &schemaQueryer{}
	coverage, _, err := NewReader(q, DefaultProfile()).Probe(context.Background(), entity.SourceTableUsageLog)
	require.NoError(t, err)
	require.Equal(t, CoverageOK, coverage)
	require.Len(t, q.queries, 1)
}
