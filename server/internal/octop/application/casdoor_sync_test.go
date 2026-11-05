package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/infra/casdoor"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/global"
	"mayfly-go/pkg/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeCasdoor struct {
	apps     []casdoor.Application
	users    []casdoor.User
	appUsers map[string][]string
	appErr   error
	userErr  error
	pagesize int
}

func (f *fakeCasdoor) TestApps(context.Context) error  { return f.appErr }
func (f *fakeCasdoor) TestUsers(context.Context) error { return f.userErr }

func (f *fakeCasdoor) ListApplications(_ context.Context, page, pageSize int) ([]casdoor.Application, int, error) {
	if f.appErr != nil {
		return nil, 0, f.appErr
	}
	if f.pagesize > 0 {
		pageSize = f.pagesize
	}
	return pageSlice(f.apps, page, pageSize)
}

func (f *fakeCasdoor) ListUsers(_ context.Context, page, pageSize int) ([]casdoor.User, int, error) {
	if f.userErr != nil {
		return nil, 0, f.userErr
	}
	if f.pagesize > 0 {
		pageSize = f.pagesize
	}
	return pageSlice(f.users, page, pageSize)
}

func (f *fakeCasdoor) GetApplicationUsers(_ context.Context, appName string) ([]string, bool, error) {
	if f.appUsers == nil {
		return nil, false, nil
	}
	ids, ok := f.appUsers[appName]
	return ids, ok, nil
}

func pageSlice[T any](all []T, page, pageSize int) ([]T, int, error) {
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= len(all) {
		return []T{}, len(all), nil
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], len(all), nil
}

func setupOctopDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&entity.CasdoorApplicationSnapshot{},
		&entity.CasdoorUserSnapshot{},
		&entity.CasdoorAuthRelation{},
		&entity.OctopEvent{},
		&entity.CollectorCheckpoint{},
		&entity.OctopInstance{},
		&entity.OctopIdentityLink{},
	))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	prev := global.Db
	global.Db = db
	t.Cleanup(func() { global.Db = prev })
	return db
}

func TestSyncDirectoryPaginationAndRevoke(t *testing.T) {
	setupOctopDB(t)
	apps := newCasdoorAppRepoForTest()
	users := newCasdoorUserRepoForTest()
	auths := newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 1

	client := &fakeCasdoor{
		apps: []casdoor.Application{
			{SourceID: "app-a", Name: "app-a", Organization: "org", Enabled: true},
			{SourceID: "app-b", Name: "app-b", Organization: "org", Enabled: true},
		},
		users: []casdoor.User{
			{SourceID: "u-1", Username: "alice", Organization: "org", Enabled: true},
			{SourceID: "u-2", Username: "bob", Organization: "org", Enabled: true},
			{SourceID: "u-3", Username: "carol", Organization: "org", Enabled: true},
		},
		appUsers: map[string][]string{"app-a": {"u-1", "u-2"}},
		pagesize: 1,
	}

	ctx := context.Background()
	require.NoError(t, SyncDirectory(ctx, client, conn, apps, users, auths))

	page, err := apps.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)

	userPage, err := users.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(3), userPage.Total)

	rel, err := auths.GetByKey(conn.Id, "app-a", "user", "u-1")
	require.NoError(t, err)
	require.Equal(t, entity.AuthConfirmed, rel.Status)

	// 撤权：u-2 不再出现，u-1 仍在
	client.users = []casdoor.User{
		{SourceID: "u-1", Username: "alice", Organization: "org", Enabled: false},
	}
	client.appUsers = map[string][]string{"app-a": {"u-1"}}
	require.NoError(t, SyncDirectory(ctx, client, conn, apps, users, auths))

	gone, err := users.GetBySource(conn.Id, "u-2")
	require.NoError(t, err)
	require.Equal(t, int8(-1), gone.Enabled)
	require.Equal(t, int8(-1), gone.Present)

	revoked, err := auths.GetByKey(conn.Id, "app-a", "user", "u-2")
	require.NoError(t, err)
	require.Equal(t, entity.AuthRevoked, revoked.Status)

	kept, err := users.GetBySource(conn.Id, "u-1")
	require.NoError(t, err)
	require.Equal(t, int8(-1), kept.Enabled) // 源端禁用
	require.Equal(t, int8(1), kept.Present)
}

func TestSyncDirectoryFailureKeepsSnapshot(t *testing.T) {
	setupOctopDB(t)
	apps := newCasdoorAppRepoForTest()
	users := newCasdoorUserRepoForTest()
	auths := newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 2
	ctx := context.Background()
	ok := &fakeCasdoor{
		apps:  []casdoor.Application{{SourceID: "app-a", Name: "app-a", Organization: "org", Enabled: true}},
		users: []casdoor.User{{SourceID: "u-1", Username: "alice", Organization: "org", Enabled: true}},
	}
	require.NoError(t, SyncDirectory(ctx, ok, conn, apps, users, auths))

	fail := &fakeCasdoor{appErr: errors.New("casdoor down")}
	err := SyncDirectory(ctx, fail, conn, apps, users, auths)
	require.Error(t, err)

	row, err := apps.GetBySource(conn.Id, "app-a")
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, int8(1), row.Present)
}

func TestSyncDirectoryDoesNotMergeSameName(t *testing.T) {
	setupOctopDB(t)
	apps := newCasdoorAppRepoForTest()
	users := newCasdoorUserRepoForTest()
	auths := newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 3
	client := &fakeCasdoor{
		users: []casdoor.User{
			{SourceID: "id-1", Username: "alice", Organization: "org", Enabled: true},
			{SourceID: "id-2", Username: "alice", Organization: "org", Enabled: true},
		},
	}
	require.NoError(t, SyncDirectory(context.Background(), client, conn, apps, users, auths))
	page, err := users.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
}

func TestSyncDirectorySkipsOtherOrg(t *testing.T) {
	setupOctopDB(t)
	apps := newCasdoorAppRepoForTest()
	users := newCasdoorUserRepoForTest()
	auths := newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 4
	client := &fakeCasdoor{
		users: []casdoor.User{
			{SourceID: "u-1", Username: "alice", Organization: "org", Enabled: true},
			{SourceID: "u-x", Username: "eve", Organization: "other", Enabled: true},
		},
	}
	require.NoError(t, SyncDirectory(context.Background(), client, conn, apps, users, auths))
	page, err := users.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, "u-1", page.List[0].SourceId)
}

func newCasdoorAppRepoForTest() repository.CasdoorApp {
	return persistenceAppRepo()
}

func persistenceAppRepo() repository.CasdoorApp {
	return newTestAppRepo()
}

type testAppRepo struct{ base.RepoImpl[*entity.CasdoorApplicationSnapshot] }

func newTestAppRepo() repository.CasdoorApp { return &testAppRepo{} }

func (r *testAppRepo) GetBySource(connectionId uint64, sourceId string) (*entity.CasdoorApplicationSnapshot, error) {
	var row entity.CasdoorApplicationSnapshot
	err := r.GetByCond(model.NewCond().Eq("connection_id", connectionId).Eq("source_id", sourceId).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *testAppRepo) MarkMissingAbsent(ctx context.Context, connectionId uint64, present []string, now time.Time) error {
	rows, err := r.SelectByCond(model.NewCond().Eq("connection_id", connectionId).Eq("present", 1))
	if err != nil {
		return err
	}
	keep := map[string]struct{}{}
	for _, id := range present {
		keep[id] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := keep[row.SourceId]; ok {
			continue
		}
		row.Present = -1
		row.Enabled = -1
		row.LastSyncAt = &now
		if err := r.UpdateById(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

type testUserRepo struct{ base.RepoImpl[*entity.CasdoorUserSnapshot] }

func newCasdoorUserRepoForTest() repository.CasdoorUser { return &testUserRepo{} }

func (r *testUserRepo) GetBySource(connectionId uint64, sourceId string) (*entity.CasdoorUserSnapshot, error) {
	var row entity.CasdoorUserSnapshot
	err := r.GetByCond(model.NewCond().Eq("connection_id", connectionId).Eq("source_id", sourceId).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *testUserRepo) MarkMissingDisabled(ctx context.Context, connectionId uint64, present []string, now time.Time) error {
	rows, err := r.SelectByCond(model.NewCond().Eq("connection_id", connectionId).Eq("present", 1))
	if err != nil {
		return err
	}
	keep := map[string]struct{}{}
	for _, id := range present {
		keep[id] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := keep[row.SourceId]; ok {
			continue
		}
		row.Present = -1
		row.Enabled = -1
		row.LastSyncAt = &now
		if err := r.UpdateById(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

type testAuthRepo struct{ base.RepoImpl[*entity.CasdoorAuthRelation] }

func newCasdoorAuthRepoForTest() repository.CasdoorAuth { return &testAuthRepo{} }

func (r *testAuthRepo) GetByKey(connectionId uint64, app, typ, sub string) (*entity.CasdoorAuthRelation, error) {
	var row entity.CasdoorAuthRelation
	err := r.GetByCond(model.NewCond().Eq("connection_id", connectionId).Eq("application_source_id", app).Eq("subject_type", typ).Eq("subject_source_id", sub).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *testAuthRepo) MarkMissingRevoked(ctx context.Context, connectionId uint64, keepKeys []repository.AuthKey, now time.Time) error {
	rows, err := r.SelectByCond(model.NewCond().Eq("connection_id", connectionId).Eq("status", entity.AuthConfirmed))
	if err != nil {
		return err
	}
	keep := map[string]struct{}{}
	for _, k := range keepKeys {
		keep[k.AppSourceId+"|"+k.SubjectType+"|"+k.SubjectSourceId] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := keep[row.ApplicationSourceId+"|"+row.SubjectType+"|"+row.SubjectSourceId]; ok {
			continue
		}
		row.Status = entity.AuthRevoked
		row.LastSyncAt = &now
		if err := r.UpdateById(ctx, row); err != nil {
			return err
		}
	}
	return nil
}
