package octopapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mayfly-go/internal/octop/domain/entity"

	"github.com/stretchr/testify/require"
)

func TestListUsersOKAndBearer(t *testing.T) {
	var gotAuth string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 7, "username": "alice", "display_name": "Alice", "email": "a@ex.com", "disabled": false, "password_hash": "should-not-matter"},
		})
	}))
	defer srv.Close()

	res := ListUsers(context.Background(), srv.URL, "jwt-token-value", 0)
	require.Equal(t, entity.UserListOK, res.Status)
	require.Equal(t, "/api/users", gotPath)
	require.Equal(t, "Bearer jwt-token-value", gotAuth)
	require.Len(t, res.Rows, 1)
	require.Equal(t, "alice", res.Rows[0]["username"])
}

func TestListUsersUnconfigured(t *testing.T) {
	res := ListUsers(context.Background(), "http://127.0.0.1:9", "", 0)
	require.Equal(t, entity.UserListUnconfigured, res.Status)
	res = ListUsers(context.Background(), "", "tok", 0)
	require.Equal(t, entity.UserListUnconfigured, res.Status)
}

func TestListUsersForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"missing users permission"}`))
	}))
	defer srv.Close()
	res := ListUsers(context.Background(), srv.URL, "tok-abc", 0)
	require.Equal(t, entity.UserListForbidden, res.Status)
	require.NotContains(t, res.Message, "tok-abc")
}

func TestListUsersUnreachableAndUnexpected(t *testing.T) {
	res := ListUsers(context.Background(), "http://127.0.0.1:1", "secret-token-xyz", 0)
	require.Equal(t, entity.UserListUnreachable, res.Status)
	require.NotContains(t, res.Message, "secret-token-xyz")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"users":[]}`))
	}))
	defer srv.Close()
	res = ListUsers(context.Background(), srv.URL, "tok", 0)
	require.Equal(t, entity.UserListUnexpected, res.Status)
	require.True(t, strings.Contains(res.Message, "non-array"))
}

func TestListUsersDoesNotUseQueryToken(t *testing.T) {
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode([]any{})
	}))
	defer srv.Close()
	_ = ListUsers(context.Background(), srv.URL, "jwt-should-not-be-query", 0)
	require.Empty(t, rawQuery)
}
