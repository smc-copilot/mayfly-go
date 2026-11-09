package init

import (
	"context"
	"fmt"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"mayfly-go/internal/casdoor/application"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/infra/persistence"
	"mayfly-go/internal/pkg/config"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/model"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestServiceLifecycleDiscoversAndSynchronizesIsolatedOrganizationContexts(t *testing.T) {
	var rejectToken atomic.Bool
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/login/oauth/access_token" {
			if rejectToken.Load() {
				http.Error(w, "rejected", http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"access_token":"fixture-token","token_type":"Bearer","expires_in":3600}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			http.Error(w, "bad auth", 401)
			return
		}
		if r.URL.Query().Get("p") == "2" {
			fmt.Fprint(w, `{"status":"ok","data":[]}`)
			return
		}
		switch r.URL.Path {
		case "/api/get-applications":
			fmt.Fprint(w, `{"status":"ok","data":[{"owner":"admin","name":"management","organization":"built-in","clientId":"management"}]}`)
		case "/api/get-organizations":
			fmt.Fprint(w, `{"status":"ok","data":[{"owner":"admin","name":"built-in"},{"owner":"admin","name":"org"}]}`)
		case "/api/get-organization-applications":
			require.Empty(t, r.URL.Query().Get("owner"))
			org := r.URL.Query().Get("organization")
			fmt.Fprintf(w, `{"status":"ok","data":[{"owner":"admin","name":"%s-app","organization":"%s"}]}`, org, org)
		case "/api/get-users":
			org := r.URL.Query().Get("owner")
			fmt.Fprintf(w, `{"status":"ok","data":[{"uuid":"%s-user","owner":"%s","name":"user"}]}`, org, org)
		default:
			fmt.Fprint(w, `{"status":"ok","data":[]}`)
		}
	}))
	defer remote.Close()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&entity.Service{}, &entity.ManagementCredential{}, &entity.ServiceOrganization{}, &entity.ServiceScope{}, &entity.Connection{}, &entity.Application{}, &entity.User{}, &entity.DirectoryObject{}, &entity.ObjectSyncState{}, &entity.Change{}, &entity.ConnectionScope{}))
	old := entity.Connection{Name: "legacy", ServerURL: remote.URL, Organization: "org", AppOwner: "octop", ClientId: "legacy-app", ClientSecretEnc: "preserve-me"}
	old.FillBaseInfo(nil)
	require.NoError(t, db.Create(&old).Error)
	user := entity.User{ConnectionId: old.Id, SourceId: "org-user", Username: "old", Organization: "org"}
	user.FillBaseInfo(nil)
	require.NoError(t, db.Create(&user).Error)
	manager := BuildServiceAPI(db, config.Aes{Key: "1234567890123456"}).Manager
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 1})
	id, err := manager.Save(ctx, application.ServiceInput{Name: "Service", ServerURL: remote.URL, ClientID: "management", ClientSecret: "secret", LegacyConnectionID: old.Id})
	require.NoError(t, err)
	orgs, err := manager.Organizations(ctx, id)
	require.NoError(t, err)
	require.Len(t, orgs, 2)
	var target, builtin entity.ServiceOrganization
	for _, org := range orgs {
		if org.Name == "org" {
			target = org
		} else {
			builtin = org
		}
	}
	result, err := manager.Sync(ctx, id, target.Id)
	require.NoError(t, err)
	require.Equal(t, "success", result.Apps.Status)
	require.Equal(t, "success", result.Users.Status)
	users, total, err := manager.Users(ctx, id, target.Id, "", 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, user.Id, users[0].Id)
	_, total, err = manager.Users(ctx, id, builtin.Id, "", 1, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	_, _, err = manager.Users(ctx, id+1, target.Id, "", 1, 10)
	require.Error(t, err)
	_, _, err = manager.Users(context.Background(), id, target.Id, "", 1, 10)
	require.Error(t, err)
	previous, err := persistence.NewConnectionQuery(db).ListVisibleConnections(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, previous)
	require.NoError(t, db.First(&old, old.Id).Error)
	require.Equal(t, "preserve-me", old.ClientSecretEnc)
	rows, err := manager.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 2, rows[0].OrganizationCount)
	ordinary := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 7})
	rows, err = manager.List(ordinary)
	require.NoError(t, err)
	require.Empty(t, rows)
	rejectToken.Store(true)
	manager = BuildServiceAPI(db, config.Aes{Key: "1234567890123456"}).Manager
	_, err = manager.Sync(ctx, id, target.Id)
	require.Error(t, err)
	var failed entity.Connection
	require.NoError(t, db.First(&failed, target.ConnectionID).Error)
	require.Equal(t, "stale", failed.AppSyncStatus)
	require.Equal(t, "stale", failed.UserSyncStatus)
	require.NotNil(t, failed.UserSuccessAt)
	var service entity.Service
	require.NoError(t, db.First(&service, id).Error)
	require.Equal(t, "failed", service.Status)
	orgs, err = manager.Organizations(ctx, id)
	require.NoError(t, err)
	for _, org := range orgs {
		if org.Id == target.Id {
			require.Equal(t, "stale", org.SyncStates["users"].Status)
			require.NotNil(t, org.SyncStates["users"].SuccessAt)
			require.Equal(t, 1, org.SyncStates["users"].Count)
		}
	}
}
