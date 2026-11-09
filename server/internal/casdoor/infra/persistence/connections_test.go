package persistence

import (
	"context"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"
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
