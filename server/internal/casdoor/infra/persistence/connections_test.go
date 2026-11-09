package persistence

import (
	"context"
	"testing"
	"time"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/model"

	"github.com/stretchr/testify/require"
)

func TestConnectionRepositoryUpdatesZeroValuesAndPreservesSync(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.Connection{}))
	repo := NewConnectionRepository(db)
	conn := &entity.Connection{
		Name: "test", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "client",
		ClientSecretEnc: "encrypted", CredentialConfigured: 1, ManagementEnabled: true, LastSyncStatus: "success",
	}
	require.NoError(t, repo.SaveConnection(context.Background(), conn))
	require.NotZero(t, conn.Id)
	conn.ManagementEnabled = false
	conn.TLSVerify = -1
	conn.LastSyncStatus = "stale-client-value"
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 9, Username: "operator"})
	require.NoError(t, repo.SaveConnection(ctx, conn))
	got, err := repo.GetConnection(context.Background(), conn.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, got.ManagementEnabled)
	require.EqualValues(t, -1, got.TLSVerify)
	require.Equal(t, "success", got.LastSyncStatus)
	require.EqualValues(t, 9, got.ModifierId)
	require.NoError(t, db.Model(&entity.Connection{}).Where("id = ?", conn.Id).Update("is_deleted", 1).Error)
	got, err = repo.GetConnection(context.Background(), conn.Id)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSyncStatusPreservesLastGoodDataOnFailureAndClearsErrorOnSuccess(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.Connection{}))
	conn := entity.Connection{Name: "test", ServerURL: "http://one", Organization: "org", ClientId: "client", AppSyncCount: 12}
	conn.FillBaseInfo(nil)
	require.NoError(t, db.Create(&conn).Error)
	repo := NewSyncStateRepository(db)
	now := time.Now().UTC()
	require.NoError(t, repo.RecordSyncState(context.Background(), conn.Id, repository.SyncState{
		Applications: repository.SyncCategory{Status: "failed", AttemptAt: now, ReasonCode: "sync_failed"},
		Users:        repository.SyncCategory{Status: "success", AttemptAt: now, SuccessAt: &now, Count: 4},
	}))
	var got entity.Connection
	require.NoError(t, db.First(&got, conn.Id).Error)
	require.Equal(t, 12, got.AppSyncCount)
	require.Equal(t, "sync_failed", got.AppSyncError)
	require.Equal(t, 4, got.UserSyncCount)
	require.NoError(t, repo.RecordSyncState(context.Background(), conn.Id, repository.SyncState{
		Applications: repository.SyncCategory{Status: "success", AttemptAt: now, SuccessAt: &now, Count: 2},
		Users:        repository.SyncCategory{Status: "success", AttemptAt: now, SuccessAt: &now, Count: 4},
	}))
	require.NoError(t, db.First(&got, conn.Id).Error)
	require.Equal(t, 2, got.AppSyncCount)
	require.Empty(t, got.AppSyncError)
}

func TestListVisibleConnectionsEnforcesAccountScope(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.Connection{}, &entity.ConnectionScope{}))
	first := entity.Connection{Name: "one", ServerURL: "http://one", Organization: "org", ClientId: "one"}
	second := entity.Connection{Name: "two", ServerURL: "http://two", Organization: "org", ClientId: "two"}
	first.FillBaseInfo(nil)
	second.FillBaseInfo(nil)
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	scope := entity.ConnectionScope{AccountId: 9, ConnectionId: first.Id, Scope: "read"}
	scope.FillBaseInfo(nil)
	require.NoError(t, db.Create(&scope).Error)
	repo := NewConnectionQuery(db)
	items, err := repo.ListVisibleConnections(context.Background(), 9)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, first.Id, items[0].Id)
	items, err = repo.ListVisibleConnections(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, items, 2)
	_, err = repo.ListVisibleConnections(context.Background(), 0)
	require.Error(t, err)
}
