package langfuse

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentRedactsWholePlaintextCredentialHeaders(t *testing.T) {
	input := "request details\nAuthorization: Basic dXNlcjpwYXNz\nCookie: session=first-secret; other=second-secret\nSet-Cookie: sid=third-secret; Path=/\nProxy-Authorization: Digest digest-secret\nend details"
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	redacted, truncated := stringifyTruncate(redactContent(encoded), MaxContentFieldBytes)
	require.False(t, truncated)
	for _, secret := range []string{"dXNlcjpwYXNz", "first-secret", "second-secret", "third-secret", "digest-secret"} {
		require.NotContains(t, redacted, secret)
	}
	require.Contains(t, redacted, "request details")
	require.Contains(t, redacted, "end details")
}

func TestContentRedactsStructuredProxyAndCookieHeaders(t *testing.T) {
	input := json.RawMessage(`[{"headers":{"Proxy-Authorization":"Basic dXNlcjpwYXNz","Set-Cookie":["sid=first-secret; Path=/","other=second-secret"],"Content-Type":"application/json"},"text":"safe"}]`)
	redacted, truncated := stringifyTruncate(redactContent(input), MaxContentFieldBytes)
	require.False(t, truncated)
	for _, secret := range []string{"dXNlcjpwYXNz", "first-secret", "second-secret"} {
		require.NotContains(t, redacted, secret)
	}
	require.Contains(t, redacted, "application/json")
	require.Contains(t, redacted, "safe")
}
