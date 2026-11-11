package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeAndAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		body := `{"status":"OK","version":"2.92.0"}`
		if r.URL.Path == "/api/public/projects" {
			body = `{"data":[{"id":"project-1","name":"test"}]}`
		}
		if r.URL.Path == "/api/public/traces" {
			body = `{"data":[],"meta":{"totalItems":0,"totalPages":0}}`
		}
		_, err := w.Write([]byte(body))
		require.NoError(t, err)
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	ver, err := c.Probe(context.Background())
	require.NoError(t, err)
	require.Equal(t, "2.92.0", ver)
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk:sk"))
	require.Equal(t, want, gotAuth)
}

func TestListTracesFiltersAndRecheckFields(t *testing.T) {
	srv := classicFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/public/traces", r.URL.Path)
		q := r.URL.Query()
		require.Equal(t, "thread-1", q.Get("sessionId"))
		require.Equal(t, "42", q.Get("userId"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"data":[{"id":"t1","name":"run","sessionId":"thread-1","userId":"42","level":"DEFAULT","latency":1.5}],
			"meta":{"page":1,"limit":20,"totalItems":1,"totalPages":1}
		}`)
	})
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	res, err := c.ListTraces(context.Background(), ListTracesQuery{SessionId: "thread-1", UserId: "42", Page: 1, Limit: 20})
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	require.Equal(t, "t1", res.Items[0].TraceId)
	require.Equal(t, "thread-1", res.Items[0].SessionId)
	require.Equal(t, "42", res.Items[0].UserId)
	require.NotNil(t, res.Items[0].LatencyMs)
	require.Equal(t, int64(1500), *res.Items[0].LatencyMs)
	require.False(t, res.HasMore)
}

func TestListObservationsAndContentTruncate(t *testing.T) {
	big := strings.Repeat("x", MaxContentFieldBytes+100)
	srv := classicFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/public/observations" && r.URL.Query().Get("traceId") == "t1":
			_, _ = io.WriteString(w, `{"data":[{"id":"o1","traceId":"t1","type":"GENERATION","name":"llm","input":"hi","output":"yo"}],"meta":{"page":1,"limit":50,"totalItems":1,"totalPages":1}}`)
		case r.URL.Path == "/api/public/observations/o1":
			_, _ = io.WriteString(w, `{"id":"o1","traceId":"t1","type":"GENERATION","input":`+jsonString(big)+`,"output":"ok"}`)
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	list, err := c.ListObservations(context.Background(), ListObservationsQuery{TraceId: "t1"})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.True(t, list.Items[0].HasContent)

	_, content, err := c.GetObservation(context.Background(), "o1")
	require.NoError(t, err)
	require.True(t, content.Truncated)
	require.LessOrEqual(t, len(content.Input), MaxContentFieldBytes)
	require.Equal(t, "ok", content.Output)
}

func TestProbeForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	_, err := c.Probe(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrForbidden)
}

func TestInvalidConfig(t *testing.T) {
	c := NewClient(Config{BaseURL: "not-a-url", PublicKey: "pk", SecretKey: "sk"})
	_, err := c.Probe(context.Background())
	require.ErrorIs(t, err, ErrConfig)
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func classicFixture(t *testing.T, read http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body string
		switch {
		case r.URL.Path == "/api/public/health":
			body = `{"status":"OK","version":"3.1.0"}`
		case r.URL.Path == "/api/public/projects":
			body = `{"data":[{"id":"project-1","name":"test"}]}`
		case r.URL.Path == "/api/public/traces" && r.URL.Query().Get("sessionId") == "":
			body = `{"data":[],"meta":{"totalItems":0,"totalPages":0}}`
		default:
			read(w, r)
			return
		}
		_, err := io.WriteString(w, body)
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClassicDirectTraceDoesNotDependOnFirstPage(t *testing.T) {
	srv := classicFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/public/traces/last-trace", r.URL.Path)
		_, err := io.WriteString(w, `{"id":"last-trace","sessionId":"thread-1","userId":"42"}`)
		require.NoError(t, err)
	})
	result, err := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"}).ListTraces(context.Background(), ListTracesQuery{SessionId: "thread-1", UserId: "42", TraceId: "last-trace"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, "last-trace", result.Items[0].TraceId)
}
