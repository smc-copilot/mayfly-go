package persistence

import (
	"context"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"

	"github.com/stretchr/testify/require"
)

func TestDirectoryQueryScopesAndPaginatesApplications(t *testing.T) {
	db := snapshotTestDB(t)
	for _, item := range []entity.Application{
		{ConnectionId: 1, SourceId: "a", Name: "alpha"},
		{ConnectionId: 1, SourceId: "b", Name: "beta"},
		{ConnectionId: 2, SourceId: "c", Name: "alpha"},
		{ConnectionId: 1, SourceId: "d", Name: "absent", Present: -1},
	} {
		item.FillBaseInfo(nil)
		require.NoError(t, db.Create(&item).Error)
	}
	query := NewDirectoryQuery(db)
	rows, total, err := query.ListApplications(context.Background(), 1, "", 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, rows, 1)
	rows, total, err = query.ListApplications(context.Background(), 1, "alpha", 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "a", rows[0].SourceId)
	_, _, err = query.ListApplications(context.Background(), 0, "", 1, 10)
	require.Error(t, err)
}

func TestDirectoryQueryScopesUsers(t *testing.T) {
	db := snapshotTestDB(t)
	for _, item := range []entity.User{
		{ConnectionId: 1, SourceId: "u1", Username: "alice"},
		{ConnectionId: 2, SourceId: "u2", Username: "alice"},
	} {
		item.FillBaseInfo(nil)
		require.NoError(t, db.Create(&item).Error)
	}
	rows, total, err := NewDirectoryQuery(db).ListUsers(context.Background(), 1, "alice", 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, "u1", rows[0].SourceId)
}

func TestDirectoryQueryScopesObjectsAndRejectsUnknownKinds(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.DirectoryObject{}))
	for _, item := range []entity.DirectoryObject{
		{ConnectionId: 1, SourceId: "role-1", Kind: "role"},
		{ConnectionId: 2, SourceId: "role-2", Kind: "role"},
		{ConnectionId: 1, SourceId: "group-1", Kind: "group"},
	} {
		item.FillBaseInfo(nil)
		require.NoError(t, db.Create(&item).Error)
	}
	rows, total, err := NewDirectoryQuery(db).ListObjects(context.Background(), 1, "role", 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, "role-1", rows[0].SourceId)
	_, _, err = NewDirectoryQuery(db).ListObjects(context.Background(), 1, "other", 1, 10)
	require.Error(t, err)
}
