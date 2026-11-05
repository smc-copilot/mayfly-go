package redaction

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractDropsPayloadAndSecrets(t *testing.T) {
	row := map[string]any{
		"id":           12,
		"user_id":      "u-1",
		"action":       "auth.login",
		"payload_json": `{"content":"hello world","tool_name":"search","args":{"q":"secret"},"event_id":"e-9"}`,
		"password":     "p@ss",
		"token":        "tok",
		"prompt":       "system prompt",
	}
	got := Extract(row)
	require.Equal(t, int(12), asInt(got["id"]))
	require.Equal(t, "u-1", got["user_id"])
	require.Equal(t, "auth.login", got["action"])
	require.Equal(t, "search", got["tool_name"])
	require.Equal(t, "e-9", got["event_id"])
	_, hasPayload := got["payload_json"]
	require.False(t, hasPayload)
	_, hasArgs := got["args"]
	require.False(t, hasArgs)
	_, hasPwd := got["password"]
	require.False(t, hasPwd)
	_, hasToken := got["token"]
	require.False(t, hasToken)
	_, hasPrompt := got["prompt"]
	require.False(t, hasPrompt)
	_, hasContent := got["content"]
	require.False(t, hasContent)
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}
