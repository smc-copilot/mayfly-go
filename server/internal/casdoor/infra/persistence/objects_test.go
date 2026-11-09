package persistence

import (
	"context"
	"testing"
	"time"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"

	"github.com/stretchr/testify/require"
)

func TestReplaceObjectsIsolatesKindAndConnection(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.DirectoryObject{}))
	for _, item := range []entity.DirectoryObject{
		{ConnectionId: 1, Kind: "role", SourceId: "org/old", Present: 1},
		{ConnectionId: 1, Kind: "group", SourceId: "org/group", Present: 1},
		{ConnectionId: 2, Kind: "role", SourceId: "other/role", Present: 1},
	} {
		item.FillBaseInfo(nil)
		require.NoError(t, db.Create(&item).Error)
	}
	repo := NewObjectSnapshotRepository(db)
	require.NoError(t, repo.ReplaceObjects(context.Background(), 1, "role", []entity.DirectoryObject{{ConnectionId: 1, Kind: "role", SourceId: "org/new", Projection: `{}`}}, time.Now()))
	var rows []entity.DirectoryObject
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 4)
	require.EqualValues(t, -1, rows[0].Present)
	require.EqualValues(t, 1, rows[1].Present)
	require.EqualValues(t, 1, rows[2].Present)
	require.EqualValues(t, 1, rows[3].Present)
	require.Error(t, repo.ReplaceObjects(context.Background(), 1, "role", []entity.DirectoryObject{{ConnectionId: 2, Kind: "role", SourceId: "bad"}}, time.Now()))
}

func TestObjectSyncStateRetainsLastSuccessOnFailure(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.ObjectSyncState{}))
	repo := NewObjectSyncStateRepository(db)
	now := time.Now().UTC()
	require.NoError(t, repo.RecordObjectSyncState(context.Background(), 1, "role", repository.SyncCategory{Status: "success", AttemptAt: now, SuccessAt: &now, Count: 3}))
	require.NoError(t, repo.RecordObjectSyncState(context.Background(), 1, "role", repository.SyncCategory{Status: "failed", AttemptAt: now.Add(time.Minute), ReasonCode: "sync_failed"}))
	var state entity.ObjectSyncState
	require.NoError(t, db.Where("connection_id = ? AND kind = ?", 1, "role").First(&state).Error)
	require.Equal(t, "stale", state.Status)
	require.Equal(t, 3, state.Count)
	require.NotNil(t, state.SuccessAt)
	require.Equal(t, "sync_failed", state.ReasonCode)
}
