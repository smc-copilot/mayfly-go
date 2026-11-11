package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProbeDoesNotAcceptPublicHealthAsAuthentication(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/health" {
			_, err := io.WriteString(w, `{"status":"OK","version":"4.70.1"}`)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := NewClient(Config{BaseURL: srv.URL, PublicKey: "bad", SecretKey: "bad"}).Probe(context.Background())
	require.ErrorIs(t, err, ErrForbidden)
}

func TestV4TraceReadUsesObservations(t *testing.T) {
	var legacyCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body string
		switch r.URL.Path {
		case "/api/public/health":
			body = `{"status":"OK","version":"4.70.1"}`
		case "/api/public/projects":
			body = `{"data":[{"id":"project-1","name":"Octop 8091"}]}`
		case "/api/public/v2/observations":
			if r.URL.Query().Get("sessionId") == "thread-1" {
				require.Equal(t, "3", r.URL.Query().Get("userId"))
				require.Equal(t, "true", r.URL.Query().Get("isRootObservation"))
				require.NotEmpty(t, r.URL.Query().Get("fromStartTime"))
				require.NotEmpty(t, r.URL.Query().Get("toStartTime"))
				require.NotContains(t, r.URL.Query().Get("fields"), "io")
				body = `{"data":[{"id":"root-1","projectId":"project-1","traceId":"trace-1","sessionId":"thread-1","userId":"3","isRootObservation":true,"name":"root","traceName":"chat turn","startTime":"2026-11-10T08:00:00Z","endTime":"2026-11-10T08:00:01Z","level":"DEFAULT"}],"meta":{}}`
			} else {
				body = `{"data":[],"meta":{}}`
			}
		default:
			legacyCalled = true
			http.NotFound(w, r)
			return
		}
		_, err := io.WriteString(w, body)
		require.NoError(t, err)
	}))
	defer srv.Close()
	from, to := fixtureWindow()
	result, err := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"}).ListTraces(context.Background(), ListTracesQuery{SessionId: "thread-1", UserId: "3", From: &from, To: &to})
	require.NoError(t, err)
	require.False(t, legacyCalled)
	require.Len(t, result.Items, 1)
	require.Equal(t, "trace-1", result.Items[0].TraceId)
	require.Equal(t, "chat turn", result.Items[0].Name)
	require.EqualValues(t, -1, result.Total, "observation count is not trace count")
}

func fixtureWindow() (time.Time, time.Time) {
	return time.Date(2026, 11, 10, 7, 0, 0, 0, time.UTC), time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
}

func v4Fixture(t *testing.T, read http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch {
		case r.URL.Path == "/api/public/health":
			body = `{"status":"OK","version":"4.70.1"}`
		case r.URL.Path == "/api/public/projects":
			body = `{"data":[{"id":"project-1","name":"Octop 8091"}]}`
		case r.URL.Path == "/api/public/v2/observations" && r.URL.Query().Get("sessionId") == "":
			body = `{"data":[],"meta":{}}`
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

func v4Row() map[string]any {
	return map[string]any{"id": "node-1", "projectId": "project-1", "traceId": "trace-1", "sessionId": "thread-1", "userId": "3", "startTime": "2026-11-10T08:00:00Z", "type": "GENERATION", "usageDetails": map[string]int64{"input": 12, "output": 7}}
}

func TestV4CursorContinuesEmptyAndShortPages(t *testing.T) {
	from, to := fixtureWindow()
	calls := 0
	srv := v4Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/public/v2/observations", r.URL.Path)
		require.Equal(t, from.Format(time.RFC3339Nano), r.URL.Query().Get("fromStartTime"))
		require.Equal(t, to.Format(time.RFC3339Nano), r.URL.Query().Get("toStartTime"))
		require.NotContains(t, r.URL.Query().Get("fields"), "io")
		var data []map[string]any
		cursor := ""
		switch calls {
		case 0:
			require.Empty(t, r.URL.Query().Get("cursor"))
			data = []map[string]any{}
			cursor = "opaque-one"
		case 1:
			require.Equal(t, "opaque-one", r.URL.Query().Get("cursor"))
			data = []map[string]any{v4Row(), v4Row()}
			cursor = "opaque-two"
		case 2:
			require.Equal(t, "opaque-two", r.URL.Query().Get("cursor"))
			data = []map[string]any{v4Row()}
		default:
			t.Errorf("unexpected request %d", calls)
		}
		calls++
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": map[string]any{"cursor": cursor}}))
	})
	c := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	q := ListTracesQuery{SessionId: "thread-1", UserId: "3", From: &from, To: &to, Limit: 20}
	first, err := c.ListTraces(context.Background(), q)
	require.NoError(t, err)
	require.Empty(t, first.Items)
	require.True(t, first.HasMore)
	require.EqualValues(t, -1, first.Total)
	q.Cursor = first.NextCursor
	second, err := c.ListTraces(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.True(t, second.HasMore)
	q.Cursor = second.NextCursor
	last, err := c.ListTraces(context.Background(), q)
	require.NoError(t, err)
	require.False(t, last.HasMore)
	require.Empty(t, last.NextCursor)
	require.Equal(t, 3, calls)
}

func TestV4ObservationsRejectMissingOrOutOfScopeFields(t *testing.T) {
	for _, field := range []string{"id", "projectId", "traceId", "sessionId", "userId", "startTime"} {
		t.Run(field, func(t *testing.T) {
			from, to := fixtureWindow()
			srv := v4Fixture(t, func(w http.ResponseWriter, r *http.Request) {
				row := v4Row()
				if field == "startTime" {
					row[field] = "2026-11-09T08:00:00Z"
				} else {
					row[field] = ""
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{row}, "meta": map[string]any{}}))
			})
			_, err := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"}).ListObservations(context.Background(), ListObservationsQuery{SessionId: "thread-1", UserId: "3", TraceId: "trace-1", From: &from, To: &to})
			require.ErrorIs(t, err, ErrAssociation)
		})
	}
}

func TestV4ContentFilterCarriesFullScopeAndRedactsSecrets(t *testing.T) {
	from, to := fixtureWindow()
	srv := v4Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/public/v2/observations", r.URL.Path)
		require.Contains(t, r.URL.Query().Get("fields"), "io")
		require.NotEqual(t, "true", r.URL.Query().Get("parseIoAsJson"))
		var conditions []struct {
			Column, Operator string
			Value            any
		}
		require.NoError(t, json.Unmarshal([]byte(r.URL.Query().Get("filter")), &conditions))
		values := map[string]any{}
		for _, condition := range conditions {
			values[condition.Column+condition.Operator] = condition.Value
		}
		require.Equal(t, map[string]any{"id=": "node-1", "traceId=": "trace-1", "sessionId=": "thread-1", "userId=": "3", "startTime>=": from.Format(time.RFC3339Nano), "startTime<": to.Format(time.RFC3339Nano)}, values)
		row := v4Row()
		row["input"] = `{"api_key":"key-value","nested":{"Authorization":"Bearer token-value"},"text":"safe"}`
		row["output"] = "password=hidden Bearer sensitive"
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{row}, "meta": map[string]any{}}))
	})
	obs, content, err := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"}).GetObservationInScope(context.Background(), "node-1", ListObservationsQuery{SessionId: "thread-1", UserId: "3", TraceId: "trace-1", From: &from, To: &to})
	require.NoError(t, err)
	require.Equal(t, "node-1", obs.ObservationId)
	for _, secret := range []string{"key-value", "token-value", "hidden", "sensitive"} {
		require.NotContains(t, content.Input+content.Output, secret)
	}
	require.Contains(t, content.Input, "safe")
	require.Contains(t, content.Output, "[redacted]")
}

func TestV4ObservationPaginationAndUsage(t *testing.T) {
	from, to := fixtureWindow()
	srv := v4Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{v4Row(), v4Row()}, "meta": map[string]any{"cursor": "next"}}))
	})
	c := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	q := ListObservationsQuery{SessionId: "thread-1", UserId: "3", TraceId: "trace-1", From: &from, To: &to}
	page, err := c.ListObservations(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.EqualValues(t, 12, *page.Items[0].InputTokens)
	require.EqualValues(t, 7, *page.Items[0].OutputTokens)
	require.EqualValues(t, -1, page.Total)
	require.True(t, page.HasMore)
	require.Equal(t, "next", page.NextCursor)
	q.Cursor = page.NextCursor
	_, err = c.ListObservations(context.Background(), q)
	require.ErrorIs(t, err, ErrUnsupported, "unchanged cursor must not loop forever")
}

func TestVerificationChecksRealProjectAndReadPermission(t *testing.T) {
	for _, tc := range []struct {
		name, version, project string
		authStatus, readStatus int
		want                   error
	}{
		{"auth denied", "4.70.1", "project-1", 401, 200, ErrForbidden},
		{"name is not ID", "4.70.1", "Octop 8091", 200, 200, ErrProject},
		{"read denied", "4.70.1", "project-1", 200, 403, ErrForbidden},
		{"v4 no legacy fallback", "4.70.1", "project-1", 200, 404, ErrUnsupported},
		{"unknown version", "5.0.0", "project-1", 200, 200, ErrUnsupported},
		{"missing version", "", "project-1", 200, 200, ErrUnsupported},
		{"valid empty project", "4.70.1", "", 200, 200, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/public/health":
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"status": "OK", "version": tc.version}))
				case "/api/public/projects":
					w.WriteHeader(tc.authStatus)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "project-1", "name": "Octop 8091"}}}))
				case "/api/public/v2/observations":
					w.WriteHeader(tc.readStatus)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "meta": map[string]any{}}))
				default:
					t.Errorf("unexpected protocol fallback: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			info, err := NewClient(Config{BaseURL: srv.URL, PublicKey: "pk", SecretKey: "sk", ProjectId: tc.project}).Verify(context.Background())
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.Equal(t, "failed", info.Checks[len(info.Checks)-1].State)
			} else {
				require.NoError(t, err)
				require.Len(t, info.Checks, 4)
				require.Equal(t, "project-1", info.ProjectId)
				for _, check := range info.Checks {
					require.Equal(t, "ready", check.State, fmt.Sprint(check))
				}
			}
		})
	}
}
