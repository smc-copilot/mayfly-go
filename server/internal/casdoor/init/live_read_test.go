package init

import (
	"context"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"mayfly-go/internal/casdoor/application"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/pkg/config"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/model"
)

// Explicit opt-in: remote requests are reads/token issuance only; all Mayfly writes use an in-memory fixture.
func TestLiveServiceReadOnly(t *testing.T) {
	endpoint := os.Getenv("CASDOOR_LIVE_URL")
	if endpoint == "" {
		t.Skip("live read-only fixture not configured")
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&entity.Service{}, &entity.ManagementCredential{}, &entity.ServiceOrganization{}, &entity.ServiceScope{}, &entity.Connection{}, &entity.Application{}, &entity.User{}, &entity.DirectoryObject{}, &entity.ObjectSyncState{}, &entity.Change{}))
	manager := BuildServiceAPI(db, config.Aes{Key: "1234567890123456"}).Manager
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 1})
	id, err := manager.Save(ctx, application.ServiceInput{Name: "live-read-only", ServerURL: endpoint, ClientID: os.Getenv("CASDOOR_LIVE_CLIENT_ID"), ClientSecret: os.Getenv("CASDOOR_LIVE_CLIENT_SECRET")})
	require.NoError(t, err)
	orgs, err := manager.Organizations(ctx, id)
	require.NoError(t, err)
	t.Logf("organizations discovered: %d", len(orgs))
	for _, org := range orgs {
		result, err := manager.Sync(ctx, id, org.Id)
		require.NoError(t, err)
		t.Logf("organization %d: applications=%s/%d users=%s/%d", org.Id, result.Apps.Status, result.Apps.Count, result.Users.Status, result.Users.Count)
		require.Equal(t, "success", result.Apps.Status)
		require.Equal(t, "success", result.Users.Status)
		for kind, state := range result.Objects {
			t.Logf("organization %d: %s=%s/%d", org.Id, kind, state.Status, state.Count)
			require.Equal(t, "success", state.Status)
		}
	}
}
