package v112

import (
	"testing"

	casdoorentity "mayfly-go/internal/casdoor/domain/entity"
	octopentity "mayfly-go/internal/octop/domain/entity"
	sysentity "mayfly-go/internal/sys/domain/entity"
	"mayfly-go/migration/migrations/migrationx"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func prepareCasdoorDirectoryMigration(t *testing.T) *gorm.DB {
	t.Helper()
	db := openCronMigDB(t)
	require.NoError(t, db.AutoMigrate(
		&octopentity.CasdoorConnection{},
		&octopentity.CasdoorApplicationSnapshot{},
		&octopentity.CasdoorUserSnapshot{},
		&octopentity.CasdoorAuthRelation{},
	))
	require.NoError(t, findMigration(t, "v1.12.3-casdoor-core-schema")(db))
	return db
}

func TestCasdoorObjectSyncStateMigrationIsIdempotent(t *testing.T) {
	db := openCronMigDB(t)
	migrate := findMigration(t, "v1.12.3-casdoor-object-sync-state")
	require.NoError(t, migrate(db))
	require.NoError(t, migrate(db))
	require.True(t, db.Migrator().HasTable(&casdoorentity.ObjectSyncState{}))
}

func TestIndependentCasdoorDirectoryCopiesStableReferences(t *testing.T) {
	db := prepareCasdoorDirectoryMigration(t)
	connection := &octopentity.CasdoorConnection{
		Name: "directory", ServerURL: "http://casdoor.test", Organization: "org",
		ClientId: "client", ClientSecretEnc: "encrypted-test-value",
	}
	connection.FillBaseInfo(nil)
	connection.SetId(41)
	require.NoError(t, db.Create(connection).Error)
	app := &octopentity.CasdoorApplicationSnapshot{
		ConnectionId: 41, SourceId: "admin/app", Name: "app", Organization: "org",
	}
	app.FillBaseInfo(nil)
	app.SetId(73)
	require.NoError(t, db.Create(app).Error)
	user := &octopentity.CasdoorUserSnapshot{
		ConnectionId: 41, SourceId: "user-stable-id", Username: "test-user", Organization: "org",
	}
	user.FillBaseInfo(nil)
	user.SetId(81)
	require.NoError(t, db.Create(user).Error)
	relation := &octopentity.CasdoorAuthRelation{
		ConnectionId: 41, ApplicationSourceId: "admin/app",
		SubjectType: "user", SubjectSourceId: "user-stable-id", Status: "confirmed",
	}
	relation.FillBaseInfo(nil)
	relation.SetId(92)
	require.NoError(t, db.Create(relation).Error)

	migrate := findMigration(t, "v1.12.3-casdoor-directory-data")
	require.NoError(t, migrate(db))
	require.NoError(t, migrate(db))
	var copied casdoorentity.Connection
	require.NoError(t, db.First(&copied, 41).Error)
	require.Equal(t, "encrypted-test-value", copied.ClientSecretEnc)
	var copiedApp casdoorentity.Application
	require.NoError(t, db.First(&copiedApp, 73).Error)
	require.Equal(t, uint64(41), copiedApp.ConnectionId)
	var copiedUser casdoorentity.User
	require.NoError(t, db.First(&copiedUser, 81).Error)
	require.Equal(t, "user-stable-id", copiedUser.SourceId)
	require.Equal(t, uint64(41), copiedUser.ConnectionId)
	var copiedRelation casdoorentity.AuthRelation
	require.NoError(t, db.First(&copiedRelation, 92).Error)
	require.Equal(t, "admin/app", copiedRelation.ApplicationSourceId)
	require.Equal(t, "user-stable-id", copiedRelation.SubjectSourceId)
	newConnection := &casdoorentity.Connection{
		Name: "new", ServerURL: "http://another.test", Organization: "new-org", ClientId: "new-client",
	}
	newConnection.FillBaseInfo(nil)
	require.NoError(t, db.Create(newConnection).Error)
	require.Greater(t, newConnection.GetId(), uint64(41))
}

func TestIndependentCasdoorDirectoryRejectsConflictingTarget(t *testing.T) {
	db := prepareCasdoorDirectoryMigration(t)
	original := &octopentity.CasdoorConnection{
		Name: "original", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "client",
	}
	original.FillBaseInfo(nil)
	original.SetId(42)
	require.NoError(t, db.Create(original).Error)
	conflict := &casdoorentity.Connection{
		Name: "different", ServerURL: "http://other.test", Organization: "org", ClientId: "client",
	}
	conflict.FillBaseInfo(nil)
	conflict.SetId(42)
	require.NoError(t, db.Create(conflict).Error)
	require.Error(t, findMigration(t, "v1.12.3-casdoor-directory-data")(db))
	var after casdoorentity.Connection
	require.NoError(t, db.First(&after, 42).Error)
	require.Equal(t, "different", after.Name)
}

func TestCasdoorResourcesPreserveReadAndRestrictRemoteWrite(t *testing.T) {
	db := openCronMigDB(t)
	require.NoError(t, db.AutoMigrate(&sysentity.Resource{}, &sysentity.RoleResource{}))
	require.NoError(t, migrationx.InsertResourcesWithRole(db, octopResources))
	require.NoError(t, migrationx.GrantRoleResource(db, 7, 1750000015))
	require.NoError(t, migrationx.GrantRoleResource(db, 7, 1750000016))
	migrate := findMigration(t, "v1.12.3-casdoor-management-resources")
	require.NoError(t, migrate(db))
	require.NoError(t, migrate(db))

	var root sysentity.Resource
	require.NoError(t, db.First(&root, 1750000004).Error)
	require.Equal(t, int64(0), root.Pid)
	require.Equal(t, "/casdoor", root.Code)
	var readId uint64
	require.NoError(t, db.Model(&sysentity.Resource{}).
		Where("code = ? AND is_deleted = 0", "casdoor:read").Pluck("id", &readId).Error)
	require.NotZero(t, readId)
	var grants int64
	require.NoError(t, db.Model(&sysentity.RoleResource{}).
		Where("role_id = ? AND resource_id = ? AND is_deleted = 0", 7, readId).Count(&grants).Error)
	require.EqualValues(t, 1, grants)
	var writeId uint64
	require.NoError(t, db.Model(&sysentity.Resource{}).
		Where("code = ? AND is_deleted = 0", "casdoor:access:manage").Pluck("id", &writeId).Error)
	require.NotZero(t, writeId)
	require.NoError(t, db.Model(&sysentity.RoleResource{}).
		Where("role_id IN ? AND resource_id = ? AND is_deleted = 0", []int{1, 7}, writeId).Count(&grants).Error)
	require.Zero(t, grants, "旧角色不能自动获得远端授权写权限")
}

func TestCasdoorResourcesRejectCodeCollision(t *testing.T) {
	db := openCronMigDB(t)
	require.NoError(t, db.AutoMigrate(&sysentity.Resource{}, &sysentity.RoleResource{}))
	require.NoError(t, migrationx.InsertResourcesWithRole(db, octopResources))
	require.NoError(t, migrationx.InsertResource(db, migrationx.Resource(
		1750099999, 0, "Other001/", 2, "other", "casdoor:read", 1, "",
	)))
	migrate := findMigration(t, "v1.12.3-casdoor-management-resources")
	require.Error(t, migrate(db))
	var root sysentity.Resource
	require.NoError(t, db.First(&root, 1750000004).Error)
	require.Equal(t, int64(1750000001), root.Pid, "冲突时不能部分迁移菜单")
}

func TestCasdoorResourcesAllowSameRouteSegmentUnderOtherParent(t *testing.T) {
	db := openCronMigDB(t)
	require.NoError(t, db.AutoMigrate(&sysentity.Resource{}, &sysentity.RoleResource{}))
	require.NoError(t, migrationx.InsertResourcesWithRole(db, octopResources))
	require.NoError(t, migrationx.InsertResource(db, migrationx.Resource(
		1750099998, 999, "Other001/Users001/", 1, "other.users", "users", 1, "",
	)))
	require.NoError(t, findMigration(t, "v1.12.3-casdoor-management-resources")(db))
}

func TestCasdoorResourcesRejectIDCollision(t *testing.T) {
	db := openCronMigDB(t)
	require.NoError(t, db.AutoMigrate(&sysentity.Resource{}, &sysentity.RoleResource{}))
	require.NoError(t, migrationx.InsertResourcesWithRole(db, octopResources))
	require.NoError(t, migrationx.InsertResource(db, migrationx.Resource(
		1750010002, 999, "Other001/Existing/", 1, "other.existing", "other-existing", 1, "",
	)))
	require.Error(t, findMigration(t, "v1.12.3-casdoor-management-resources")(db))
	var root sysentity.Resource
	require.NoError(t, db.First(&root, 1750000004).Error)
	require.Equal(t, int64(1750000001), root.Pid)
}
