package persistence

import (
	"context"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"

	"github.com/stretchr/testify/require"
)

func TestScopeRepositoryFiltersAccountConnectionAndDeleted(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.ConnectionScope{}))
	scope := &entity.ConnectionScope{AccountId: 2, ConnectionId: 10, Scope: "read"}
	scope.FillBaseInfo(nil)
	require.NoError(t, db.Create(scope).Error)
	repo := NewScopeRepository(db)
	got, err := repo.GetScope(context.Background(), 2, 10)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "read", got.Scope)
	got, err = repo.GetScope(context.Background(), 2, 11)
	require.NoError(t, err)
	require.Nil(t, got)
	require.NoError(t, db.Model(&entity.ConnectionScope{}).Where("id = ?", scope.Id).Update("is_deleted", 1).Error)
	got, err = repo.GetScope(context.Background(), 2, 10)
	require.NoError(t, err)
	require.Nil(t, got)
}
