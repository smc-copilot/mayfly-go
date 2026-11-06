package casdoor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListApplicationsSendsOrganizationAndOwner(t *testing.T) {
	var got urlQuery
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = parseQuery(r)
		require.Equal(t, apiOrganizationApplications, r.URL.Path)
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "cid", user)
		require.Equal(t, "csecret", pass)
		writeList(w, []map[string]any{
			{"name": "portal", "displayName": "Portal", "organization": "acme", "owner": "built-in", "disableSignin": false, "redirectUris": []string{"https://octop.example.com/api/auth/oidc/callback"}, "clientSecret": "app-secret-must-not-store"},
			{"name": "ops", "displayName": "Ops", "organization": "acme", "owner": "built-in", "disableSignin": true},
			{"name": "other", "organization": "other-org", "owner": "built-in"},
		}, 3)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "acme", AppOwner: "built-in", ClientID: "cid", ClientSecret: "csecret"})
	apps, total, err := c.ListApplications(context.Background(), 1, 100)
	require.NoError(t, err)
	require.Equal(t, "acme", got.Get("organization"))
	require.Equal(t, "built-in", got.Get("owner"))
	require.NotEqual(t, "acme", got.Get("owner"))
	require.Equal(t, "1", got.Get("p"))
	require.Equal(t, "100", got.Get("pageSize"))
	require.Equal(t, 3, total)
	require.Len(t, apps, 3)
	require.Equal(t, "built-in", apps[0].Owner)
	require.True(t, apps[0].EnabledKnown)
	require.True(t, apps[0].Enabled)
	require.True(t, apps[1].EnabledKnown)
	require.False(t, apps[1].Enabled)
	require.False(t, apps[2].EnabledKnown)
	require.Equal(t, []string{"https://octop.example.com/api/auth/oidc/callback"}, apps[0].RedirectURIs)
	raw, err := json.Marshal(apps[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), "app-secret-must-not-store")
	require.NotContains(t, string(raw), "clientSecret")
}

func TestListApplicationsDoesNotPutOrgInOwner(t *testing.T) {
	var got urlQuery
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = parseQuery(r)
		if r.URL.Query().Get("organization") == "" {
			writeStatus(w, 200, "error", "Missing parameter: organization")
			return
		}
		writeList(w, []map[string]any{{"name": "a", "organization": "org-x", "owner": "admin"}}, 1)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "org-x", AppOwner: "", ClientID: "id", ClientSecret: "s"})
	_, _, err := c.ListApplications(context.Background(), 1, 10)
	require.NoError(t, err)
	require.Equal(t, "org-x", got.Get("organization"))
	require.Equal(t, "", got.Get("owner"))
	require.True(t, got.Has("owner"))
}

func TestListApplicationsMissingOrganizationClassified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, 200, "error", "Missing parameter: organization")
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "", AppOwner: "admin", ClientID: "id", ClientSecret: "super-secret-value"})
	_, _, err := c.ListApplications(context.Background(), 1, 10)
	require.Error(t, err)
	require.True(t, IsKind(err, KindInvalidOrg))
	require.Contains(t, err.Error(), apiOrganizationApplications)
	require.NotContains(t, err.Error(), "super-secret-value")
}

func TestListApplicationsPaginates(t *testing.T) {
	pages := map[string][]map[string]any{
		"1": {{"name": "a", "owner": "admin", "organization": "org"}, {"name": "b", "owner": "admin", "organization": "org"}},
		"2": {{"name": "c", "owner": "admin", "organization": "org"}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeList(w, pages[r.URL.Query().Get("p")], 3)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "org", AppOwner: "admin", ClientID: "id", ClientSecret: "s"})
	page1, total, err := c.ListApplications(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, page1, 2)
	page2, _, err := c.ListApplications(context.Background(), 2, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "c", page2[0].Name)
	require.False(t, ShouldStopPaging(2, 3, 1, 2, 2))
	require.True(t, ShouldStopPaging(3, 3, 2, 2, 1))
	require.False(t, ShouldStopPaging(1, 3, 1, 100, 1), "short first page must continue when total says more remain")
	require.True(t, ShouldStopPaging(3, 3, 1, 100, 3))
}

func TestListUsersPrefersGetUsers(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		require.Equal(t, "acme", r.URL.Query().Get("owner"))
		if r.URL.Path == apiUsersLegacy {
			t.Errorf("must not call legacy path when get-users exists")
		}
		writeList(w, []map[string]any{
			{"name": "alice", "owner": "acme", "email": "a@x", "isForbidden": false, "isDeleted": false},
			{"name": "bob", "owner": "acme", "isForbidden": true},
			{"name": "carol", "owner": "acme"},
		}, 3)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "acme", ClientID: "id", ClientSecret: "s"})
	users, total, err := c.ListUsers(context.Background(), 2, 50)
	require.NoError(t, err)
	require.Equal(t, []string{apiUsers}, paths)
	require.Equal(t, 3, total)
	require.True(t, users[0].EnabledKnown)
	require.True(t, users[0].Enabled)
	require.True(t, users[1].EnabledKnown)
	require.False(t, users[1].Enabled)
	require.False(t, users[2].EnabledKnown)
	require.Equal(t, "acme/alice", users[0].SourceID)
}

func TestListUsersFallsBackToLegacyPath(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == apiUsers {
			http.NotFound(w, r)
			return
		}
		writeList(w, []map[string]any{{"name": "alice", "owner": "acme"}}, 1)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "acme", ClientID: "id", ClientSecret: "s"})
	users, _, err := c.ListUsers(context.Background(), 1, 10)
	require.NoError(t, err)
	require.Equal(t, []string{apiUsers, apiUsersLegacy}, paths)
	require.Equal(t, "alice", users[0].Username)
}

func TestListUsersLegacyMissingIsRouteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "acme", ClientID: "id", ClientSecret: "super-secret-value"})
	_, _, err := c.ListUsers(context.Background(), 1, 10)
	require.Error(t, err)
	require.True(t, IsKind(err, KindRouteMissing))
	require.NotContains(t, err.Error(), "super-secret-value")
}

func TestForbiddenAndTimeoutClassified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"status":"error","msg":"Unauthorized operation"}`)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "acme", AppOwner: "admin", ClientID: "id", ClientSecret: "super-secret"})
	_, _, err := c.ListApplications(context.Background(), 1, 1)
	require.True(t, IsKind(err, KindForbidden))
	require.Contains(t, err.Error(), "403")
	require.NotContains(t, err.Error(), "super-secret")
	require.NotContains(t, strings.ToLower(err.Error()), "authorization")
}

func TestGetApplicationUsersStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, apiApplication, r.URL.Path)
		id := r.URL.Query().Get("id")
		switch id {
		case "built-in/portal":
			writeJSON(w, map[string]any{"status": "ok", "data": map[string]any{"name": "portal", "users": []any{"u-1", "u-2"}}})
		case "built-in/empty":
			writeJSON(w, map[string]any{"status": "ok", "data": map[string]any{"name": "empty", "users": []any{}}})
		case "built-in/nofield":
			writeJSON(w, map[string]any{"status": "ok", "data": map[string]any{"name": "nofield", "owner": "built-in"}})
		case "built-in/denied":
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, map[string]any{"status": "error", "msg": "no permission"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, Organization: "acme", AppOwner: "built-in", ClientID: "id", ClientSecret: "super-secret-value"}).(*httpClient)

	ok := c.GetApplicationUsers(context.Background(), "built-in", "portal")
	require.Equal(t, "confirmed", ok.Status)
	require.Equal(t, []string{"u-1", "u-2"}, ok.UserIDs)

	empty := c.GetApplicationUsers(context.Background(), "built-in", "empty")
	require.Equal(t, "confirmed", empty.Status)
	require.Empty(t, empty.UserIDs)

	missing := c.GetApplicationUsers(context.Background(), "built-in", "nofield")
	require.Equal(t, "unsupported", missing.Status)
	require.False(t, missing.Failed)

	denied := c.GetApplicationUsers(context.Background(), "built-in", "denied")
	require.Equal(t, "unverified", denied.Status)
	require.True(t, denied.Failed)
	require.NotContains(t, denied.Reason, "super-secret-value")

	noOwner := c.GetApplicationUsers(context.Background(), "", "portal")
	require.Equal(t, "unverified", noOwner.Status)
	require.Contains(t, noOwner.Reason, "owner")
}

func TestTestServiceReachableWithoutSecret(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := r.BasicAuth(); ok {
			sawAuth = true
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(Config{ServerURL: srv.URL, ClientID: "id", ClientSecret: "s"})
	require.NoError(t, c.TestService(context.Background()))
	require.False(t, sawAuth)
}

func TestAPIErrorSanitizesSecret(t *testing.T) {
	err := classifyTransport("/api/get-users", &netError{msg: "Get http://x:secret@host/api/get-users?client_secret=abc: timeout", timeout: true})
	require.True(t, IsKind(err, KindTimeout))
	require.NotContains(t, err.Error(), "abc")
	require.NotContains(t, err.Error(), "secret@")
}

type urlQuery interface {
	Get(string) string
	Has(string) bool
}

func parseQuery(r *http.Request) urlQuery { return r.URL.Query() }

func writeList(w http.ResponseWriter, items []map[string]any, total int) {
	writeJSON(w, map[string]any{"status": "ok", "data": items, "data2": total})
}

func writeStatus(w http.ResponseWriter, httpStatus int, status, msg string) {
	w.WriteHeader(httpStatus)
	writeJSON(w, map[string]any{"status": status, "msg": msg})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

type netError struct {
	msg     string
	timeout bool
}

func (e *netError) Error() string   { return e.msg }
func (e *netError) Timeout() bool   { return e.timeout }
func (e *netError) Temporary() bool { return true }
