package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"mayfly-go/internal/casdoor/application"
)

func TestDirectoryRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	confs := (&DirectoryAPI{}).ReqConfs()
	group := engine.Group("/api" + confs.Group)
	for _, conf := range confs.Confs {
		conf.Group(group)
	}
	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		http.MethodGet + " /api/casdoor/connections",
		http.MethodPost + " /api/casdoor/connections",
		http.MethodPost + " /api/casdoor/connections/test",
		http.MethodPost + " /api/casdoor/connections/:id/sync",
		http.MethodGet + " /api/casdoor/applications",
		http.MethodGet + " /api/casdoor/users",
		http.MethodGet + " /api/casdoor/objects",
		http.MethodGet + " /api/casdoor/operations",
	} {
		require.True(t, routes[route], route)
	}
}

func TestSyncResponseDoesNotExposeRawErrors(t *testing.T) {
	result := category(application.DirectoryCategoryResult{Status: "failed", Err: errors.New("fixture-secret")})
	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(data), "fixture-secret")
	require.Contains(t, string(data), "sync_failed")
}
