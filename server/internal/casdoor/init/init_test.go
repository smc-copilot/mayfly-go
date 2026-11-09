package init

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/pkg/config"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/global"
	"mayfly-go/pkg/ioc"
	"mayfly-go/pkg/model"
	"mayfly-go/pkg/req"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCompositionBuildsStandaloneServices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	api := BuildDirectoryAPI(db, config.Aes{Key: "1234567890123456"})
	require.NotNil(t, api.ListService)
	require.NotNil(t, api.Connections)
	require.NotNil(t, api.Tester)
	require.NotNil(t, api.Syncer)
	require.NotNil(t, api.Directory)
	require.NotNil(t, api.Operations)
}

func TestStartupRegistersReachableDirectoryEndpoints(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&entity.Connection{}, &entity.Application{}, &entity.User{}, &entity.DirectoryObject{}, &entity.Change{}))
	previousDB, previousConfig := global.Db, config.Conf
	global.Db, config.Conf = db, &config.Config{Aes: config.Aes{Key: "1234567890123456"}}
	t.Cleanup(func() { global.Db, config.Conf = previousDB, previousConfig })
	registerServices()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	for _, api := range ioc.GetBeansByType[req.RouterApi]() {
		confs := api.ReqConfs()
		if confs.Group != "/casdoor" {
			continue
		}
		req.BatchSetGroup(router.Group("/api/casdoor"), confs.Confs)
	}
	for _, path := range []string{"connections", "applications?connectionId=1", "users?connectionId=1", "objects?connectionId=1&kind=role", "operations?connectionId=1"} {
		request := httptest.NewRequest(http.MethodGet, "/api/casdoor/"+path, nil)
		request = request.WithContext(contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 1}))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, path+": "+response.Body.String())
		require.NotContains(t, response.Body.String(), "not found", path)
		var result model.Result
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.EqualValues(t, model.SuccessCode, result.Code, path+": "+response.Body.String())
	}
}
