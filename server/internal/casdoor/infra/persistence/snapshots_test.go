package persistence

import (
	"context"
	"testing"
	"time"

	"mayfly-go/internal/casdoor/domain/entity"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func snapshotTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&entity.Application{}, &entity.User{}))
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}

func seedApp(t *testing.T, db *gorm.DB, connectionID uint64, source, name string) uint64 {
	t.Helper()
	app := &entity.Application{ConnectionId: connectionID, SourceId: source, Name: name, Present: 1, Enabled: 1}
	app.FillBaseInfo(nil)
	require.NoError(t, db.Create(app).Error)
	return app.Id
}

func TestReplaceApplicationsPreservesIDsAndSeparatesConnections(t *testing.T) {
	db := snapshotTestDB(t)
	id := seedApp(t, db, 1, "admin/app", "old")
	missingID := seedApp(t, db, 1, "admin/missing", "missing")
	otherID := seedApp(t, db, 2, "admin/other", "other")
	repo := NewSnapshotRepository(db)
	err := repo.ReplaceApplications(context.Background(), 1, []entity.Application{
		{ConnectionId: 1, SourceId: "admin/app", Name: "updated", Enabled: 0},
	}, time.Now())
	require.NoError(t, err)
	var updated, missing, other entity.Application
	require.NoError(t, db.First(&updated, id).Error)
	require.Equal(t, "updated", updated.Name)
	require.Zero(t, updated.Enabled, "未知状态不能被 GORM 跳过")
	require.EqualValues(t, 1, updated.Present)
	require.NoError(t, db.First(&missing, missingID).Error)
	require.EqualValues(t, -1, missing.Present)
	require.EqualValues(t, 1, missing.Enabled, "源端缺失不等同于禁用")
	require.NoError(t, db.First(&other, otherID).Error)
	require.EqualValues(t, 1, other.Present)
}

func TestPartialApplicationSnapshotDoesNotMarkUnseenApplicationsAbsent(t *testing.T) {
	db := snapshotTestDB(t)
	id := seedApp(t, db, 1, "admin/unseen", "unseen")
	repo := NewSnapshotRepository(db)
	err := repo.MergeApplications(context.Background(), 1, []entity.Application{{
		ConnectionId: 1, SourceId: "octop/app", Name: "app",
	}}, time.Now())
	require.NoError(t, err)
	var existing entity.Application
	require.NoError(t, db.First(&existing, id).Error)
	require.EqualValues(t, 1, existing.Present)
}

func TestReplaceApplicationsRollsBackPartialFailure(t *testing.T) {
	db := snapshotTestDB(t)
	id := seedApp(t, db, 1, "admin/app", "original")
	require.NoError(t, db.Exec("CREATE TRIGGER fail_bad_app BEFORE INSERT ON t_casdoor_application WHEN NEW.name = 'bad' BEGIN SELECT RAISE(FAIL, 'fixture error'); END").Error)
	repo := NewSnapshotRepository(db)
	err := repo.ReplaceApplications(context.Background(), 1, []entity.Application{
		{ConnectionId: 1, SourceId: "admin/app", Name: "modified"},
		{ConnectionId: 1, SourceId: "admin/bad", Name: "bad"},
	}, time.Now())
	require.Error(t, err)
	var app entity.Application
	require.NoError(t, db.First(&app, id).Error)
	require.Equal(t, "original", app.Name)
	require.EqualValues(t, 1, app.Present)
}

func TestReplaceUsersRejectsCrossConnection(t *testing.T) {
	db := snapshotTestDB(t)
	repo := NewSnapshotRepository(db)
	err := repo.ReplaceUsers(context.Background(), 1, []entity.User{
		{ConnectionId: 2, SourceId: "id", Username: "test"},
	}, time.Now())
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&entity.User{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestEmptySnapshotRejectsZeroConnection(t *testing.T) {
	db := snapshotTestDB(t)
	repo := NewSnapshotRepository(db)
	require.Error(t, repo.ReplaceApplications(context.Background(), 0, nil, time.Now()))
	require.Error(t, repo.ReplaceUsers(context.Background(), 0, nil, time.Now()))
}

func TestReplaceUsersPreservesStableIDAndClearsUnknownStatus(t *testing.T) {
	db := snapshotTestDB(t)
	existing := &entity.User{ConnectionId: 1, SourceId: "uuid-1", ObjectId: "org/old", Username: "old", Email: "old@example.test", Enabled: 1, Present: 1}
	existing.FillBaseInfo(nil)
	require.NoError(t, db.Create(existing).Error)
	repo := NewSnapshotRepository(db)
	require.NoError(t, repo.ReplaceUsers(context.Background(), 1, []entity.User{{
		ConnectionId: 1, SourceId: "uuid-1", ObjectId: "org/new", Username: "new", Enabled: 0,
	}}, time.Now()))
	var updated entity.User
	require.NoError(t, db.First(&updated, existing.Id).Error)
	require.Equal(t, "org/new", updated.ObjectId)
	require.Empty(t, updated.Email)
	require.Zero(t, updated.Enabled)
}
