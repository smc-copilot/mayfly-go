package application

import (
	"testing"
	"time"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/infra/pgcollect"
	"mayfly-go/internal/octop/infra/redaction"

	"github.com/stretchr/testify/require"
)

func TestParseSourceTimeUnixFractionalSeconds(t *testing.T) {
	got, ok := parseSourceTime("1793872991.25")
	require.True(t, ok)
	require.True(t, got.Equal(time.Unix(1793872991, 250000000).UTC()))
}

func TestParseSourceTimeUnixSecondsAndMilliseconds(t *testing.T) {
	for _, value := range []string{"1793872991", "1793872991000"} {
		got, ok := parseSourceTime(value)
		require.True(t, ok, value)
		require.True(t, got.Equal(time.Unix(1793872991, 0).UTC()), value)
	}
}

func TestMapOctopAuditActorAndTrajectoryKind(t *testing.T) {
	inst := &entity.OctopInstance{}
	audit := mapEvent(inst, entity.SourceTableAuditLog, pgcollect.Row{RecordID: "1", Fields: redaction.Extract(map[string]any{"id": 1, "ts": int64(1793872991), "actor": "alice", "action": "auth.login"})})
	require.Equal(t, "alice", audit.OctopUsername)
	require.Equal(t, entity.EventLogin, audit.Category)
	require.NotNil(t, audit.SourceTime)
	trajectory := mapEvent(inst, entity.SourceTableTrajectoryEvents, pgcollect.Row{RecordID: "2", Fields: redaction.Extract(map[string]any{"id": 2, "ts": 1793872991.25, "kind": "tool.call", "agent_id": "agent-1"})})
	require.Equal(t, "tool.call", trajectory.Action)
	require.Equal(t, entity.EventTrajectory, trajectory.Category)
	require.NotNil(t, trajectory.SourceTime)
	require.NotEqual(t, int8(1), trajectory.Uncollected)
}

func TestMapOctopUsagePreservesCallDetails(t *testing.T) {
	row := pgcollect.Row{RecordID: "5", Fields: redaction.Extract(map[string]any{"id": 5, "ts": int64(1793872991), "user_id": int64(2), "model": "model-a", "input_tokens": int64(12), "output_tokens": int64(8)})}
	ev := mapEvent(&entity.OctopInstance{}, entity.SourceTableUsageLog, row)
	require.Equal(t, entity.EventCall, ev.Category)
	require.Equal(t, "model-a", ev.Model)
	require.Equal(t, int64(12), ev.InputTokens)
	require.NotEqual(t, int8(1), ev.Uncollected)
}
