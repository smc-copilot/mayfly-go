package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServiceRoutesIncludeOrganizationContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	confs := (&ServiceAPI{}).ReqConfs()
	for _, conf := range confs.Confs {
		conf.Group(router.Group("/api" + confs.Group))
	}
	routes := map[string]bool{}
	for _, r := range router.Routes() {
		routes[r.Method+" "+r.Path] = true
	}
	for _, path := range []string{"GET /api/casdoor/services", "POST /api/casdoor/services/test", "GET /api/casdoor/services/:serviceId/organizations", "GET /api/casdoor/services/:serviceId/organizations/:organizationId/users", "GET /api/casdoor/services/:serviceId/organizations/:organizationId/applications"} {
		require.True(t, routes[path], path)
	}
}
