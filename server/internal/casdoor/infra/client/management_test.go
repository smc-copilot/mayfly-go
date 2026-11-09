package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceRoleUsersSendsOnlyUsersField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/update-role", r.URL.Path)
		require.Equal(t, "org/role", r.URL.Query().Get("id"))
		require.Equal(t, "users", r.URL.Query().Get("columns"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Len(t, body, 1)
		require.Contains(t, body, "users")
		fmt.Fprint(w, `{"status":"ok","data":"Affected"}`)
	}))
	defer server.Close()
	err := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"}).ReplaceRoleUsers(context.Background(), "org/role", []string{"org/user"})
	require.NoError(t, err)
}

func TestRoleUpdateRejectsUnaffectedAndCrossOrganization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok","data":"Unaffected"}`)
	}))
	defer server.Close()
	c := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"})
	require.Error(t, c.ReplaceRoleUsers(context.Background(), "org/role", []string{"org/user"}))
	require.Error(t, c.ReplaceRoleUsers(context.Background(), "other/role", []string{"org/user"}))
}

func TestRoleUpdateTreatsServerFailureAsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failure", http.StatusInternalServerError) }))
	defer server.Close()
	err := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"}).ReplaceRoleUsers(context.Background(), "org/role", []string{"org/user"})
	require.ErrorAs(t, err, new(WriteUnknownError))
}

func TestRoleUpdateDoesNotFollowRedirectAndMarksOutcomeUnknown(t *testing.T) {
	var followed bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	err := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"}).ReplaceRoleUsers(context.Background(), "org/role", []string{"org/user"})
	require.ErrorAs(t, err, new(WriteUnknownError))
	require.False(t, followed)
}

func TestGetRoleReadsWhitelistedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/get-role", r.URL.Path)
		require.Equal(t, "org/role", r.URL.Query().Get("id"))
		fmt.Fprint(w, `{"status":"ok","data":{"owner":"org","name":"role","users":["org/user"],"roles":["org/nested"],"unknown":"ignored"}}`)
	}))
	defer server.Close()
	role, err := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"}).GetRole(context.Background(), "org/role")
	require.NoError(t, err)
	require.NotNil(t, role)
	require.Equal(t, []string{"org/user"}, role.Users)
	require.Equal(t, []string{"org/nested"}, role.Roles)
}

func TestPatchUserLimitsEditableFieldsAndKeepsExplicitEmptyEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/update-user", r.URL.Path)
		require.Equal(t, "org/alice", r.URL.Query().Get("id"))
		require.Equal(t, "displayName,email", r.URL.Query().Get("columns"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, map[string]any{"displayName": "Alice", "email": ""}, body)
		fmt.Fprint(w, `{"status":"ok","data":"Affected"}`)
	}))
	defer server.Close()
	display, email := "Alice", ""
	err := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"}).PatchUser(context.Background(), "org/alice", UserPatch{DisplayName: &display, Email: &email})
	require.NoError(t, err)
}

func TestGetUserDoesNotExposeSensitiveFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok","data":{"owner":"org","name":"alice","displayName":"Alice","password":"secret-value"}}`)
	}))
	defer server.Close()
	user, err := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"}).GetUser(context.Background(), "org/alice")
	require.NoError(t, err)
	serialized, err := json.Marshal(user)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "secret-value")
	require.Equal(t, "alice", user.Name)
}

func TestPatchApplicationIncludesIdentityWithoutSendingSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/update-application", r.URL.Path)
		require.Equal(t, "admin/app", r.URL.Query().Get("id"))
		require.Equal(t, "displayName", r.URL.Query().Get("columns"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "org", body["organization"])
		require.Equal(t, "app", body["name"])
		require.Equal(t, "client", body["clientId"])
		require.NotContains(t, body, "clientSecret")
		fmt.Fprint(w, `{"status":"ok","data":"Affected"}`)
	}))
	defer server.Close()
	display := "New name"
	err := NewManagementClient(Config{ServerURL: server.URL, Organization: "org"}).PatchApplication(context.Background(), ApplicationIdentity{Owner: "admin", Name: "app", Organization: "org", ClientID: "client"}, ApplicationPatch{DisplayName: &display})
	require.NoError(t, err)
}
