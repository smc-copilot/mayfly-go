package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/infra/casdoor"
	"mayfly-go/internal/pkg/config"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/global"
	"mayfly-go/pkg/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeCasdoor struct {
	apps         []casdoor.Application
	users        []casdoor.User
	appUsers     map[string]casdoor.RelationRead
	appErr       error
	userErr      error
	svcErr       error
	pagesize     int
	failAppPage  int
	failUserPage int
}

func (f *fakeCasdoor) TestService(context.Context) error { return f.svcErr }
func (f *fakeCasdoor) TestApps(context.Context) error    { return f.appErr }
func (f *fakeCasdoor) TestUsers(context.Context) error   { return f.userErr }

func (f *fakeCasdoor) ListApplications(_ context.Context, page, pageSize int) ([]casdoor.Application, int, error) {
	if f.appErr != nil {
		return nil, 0, f.appErr
	}
	if f.pagesize > 0 {
		pageSize = f.pagesize
	}
	if f.failAppPage > 0 && page >= f.failAppPage {
		return nil, 0, errors.New("casdoor GET /api/get-organization-applications: http 500 (page 2 failed)")
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
	if f.failUserPage > 0 && page >= f.failUserPage {
		return nil, 0, errors.New("casdoor GET /api/get-users: http 500 (page 2 failed)")
	}
	return pageSlice(f.users, page, pageSize)
}

func (f *fakeCasdoor) GetApplicationUsers(_ context.Context, _, appName string) casdoor.RelationRead {
	if f.appUsers == nil {
		return casdoor.RelationRead{Status: entity.AuthUnsupported, Reason: "no users field"}
	}
	read, ok := f.appUsers[appName]
	if !ok {
		return casdoor.RelationRead{Status: entity.AuthUnverified, Reason: "application relation not readable"}
	}
	return read
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
	if config.Conf == nil {
		config.Conf = &config.Config{}
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&entity.CasdoorConnection{},
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

func enabledApp(id, name, org string) casdoor.Application {
	return casdoor.Application{SourceID: id, Name: name, Organization: org, Owner: "built-in", Enabled: true, EnabledKnown: true}
}

func enabledUser(id, name, org string) casdoor.User {
	return casdoor.User{SourceID: id, Username: name, Organization: org, Enabled: true, EnabledKnown: true}
}

func TestSyncDirectoryPaginationAndRevoke(t *testing.T) {
	setupOctopDB(t)
	apps, users, auths := newCasdoorAppRepoForTest(), newCasdoorUserRepoForTest(), newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org", AppOwner: "built-in"}
	conn.Id = 1

	client := &fakeCasdoor{
		apps: []casdoor.Application{enabledApp("app-a", "app-a", "org"), enabledApp("app-b", "app-b", "org")},
		users: []casdoor.User{
			enabledUser("u-1", "alice", "org"),
			enabledUser("u-2", "bob", "org"),
			enabledUser("u-3", "carol", "org"),
		},
		appUsers: map[string]casdoor.RelationRead{
			"app-a": {Status: entity.AuthConfirmed, UserIDs: []string{"u-1", "u-2"}},
		},
		pagesize: 1,
	}

	ctx := context.Background()
	res := SyncDirectory(ctx, client, conn, apps, users, auths)
	require.Equal(t, entity.SyncStatusSuccess, res.Apps.Status)
	require.Equal(t, entity.SyncStatusSuccess, res.Users.Status)
	require.Equal(t, 2, res.Apps.Count)
	require.Equal(t, 3, res.Users.Count)

	page, err := apps.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)

	userPage, err := users.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(3), userPage.Total)

	rel, err := auths.GetByKey(conn.Id, "app-a", "user", "u-1")
	require.NoError(t, err)
	require.Equal(t, entity.AuthConfirmed, rel.Status)
	require.NotNil(t, rel.LastConfirmedAt)

	client.users = []casdoor.User{{SourceID: "u-1", Username: "alice", Organization: "org", Enabled: false, EnabledKnown: true}}
	client.appUsers = map[string]casdoor.RelationRead{"app-a": {Status: entity.AuthConfirmed, UserIDs: []string{"u-1"}}}
	require.Equal(t, entity.SyncStatusSuccess, SyncDirectory(ctx, client, conn, apps, users, auths).Users.Status)

	gone, err := users.GetBySource(conn.Id, "u-2")
	require.NoError(t, err)
	require.Equal(t, int8(-1), gone.Enabled)
	require.Equal(t, int8(-1), gone.Present)

	revoked, err := auths.GetByKey(conn.Id, "app-a", "user", "u-2")
	require.NoError(t, err)
	require.Equal(t, entity.AuthRevoked, revoked.Status)

	kept, err := users.GetBySource(conn.Id, "u-1")
	require.NoError(t, err)
	require.Equal(t, int8(-1), kept.Enabled)
	require.Equal(t, int8(1), kept.Present)
}

func TestSyncDirectoryPage2FailureKeepsSnapshot(t *testing.T) {
	setupOctopDB(t)
	apps, users, auths := newCasdoorAppRepoForTest(), newCasdoorUserRepoForTest(), newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 2
	ctx := context.Background()
	ok := &fakeCasdoor{
		apps:     []casdoor.Application{enabledApp("app-a", "app-a", "org")},
		users:    []casdoor.User{enabledUser("u-1", "alice", "org"), enabledUser("u-2", "bob", "org")},
		pagesize: 1,
	}
	require.Equal(t, entity.SyncStatusSuccess, SyncDirectory(ctx, ok, conn, apps, users, auths).Users.Status)

	fail := &fakeCasdoor{
		apps:         []casdoor.Application{enabledApp("app-a", "app-a", "org"), enabledApp("app-new", "app-new", "org")},
		users:        []casdoor.User{enabledUser("u-1", "alice", "org"), enabledUser("u-2", "bob", "org"), enabledUser("u-3", "carol", "org")},
		pagesize:     1,
		failUserPage: 2,
	}
	res := SyncDirectory(ctx, fail, conn, apps, users, auths)
	require.Equal(t, entity.SyncStatusSuccess, res.Apps.Status)
	require.Equal(t, entity.SyncStatusFailed, res.Users.Status)

	u1, err := users.GetBySource(conn.Id, "u-1")
	require.NoError(t, err)
	require.Equal(t, int8(1), u1.Present)
	require.Equal(t, int8(1), u1.Enabled)

	u2, err := users.GetBySource(conn.Id, "u-2")
	require.NoError(t, err)
	require.Equal(t, int8(1), u2.Present)

	u3, err := users.GetBySource(conn.Id, "u-3")
	require.NoError(t, err)
	require.Nil(t, u3)
}

func TestSyncDirectoryAppFailUserOK(t *testing.T) {
	setupOctopDB(t)
	apps, users, auths := newCasdoorAppRepoForTest(), newCasdoorUserRepoForTest(), newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 3
	ctx := context.Background()
	ok := &fakeCasdoor{
		apps:  []casdoor.Application{enabledApp("app-a", "app-a", "org")},
		users: []casdoor.User{enabledUser("u-1", "alice", "org")},
	}
	require.Equal(t, entity.SyncStatusSuccess, SyncDirectory(ctx, ok, conn, apps, users, auths).Apps.Status)

	mixed := &fakeCasdoor{
		appErr: errors.New("casdoor GET /api/get-organization-applications: http 403 (forbidden)"),
		users:  []casdoor.User{enabledUser("u-1", "alice", "org"), enabledUser("u-2", "bob", "org")},
	}
	res := SyncDirectory(ctx, mixed, conn, apps, users, auths)
	require.Equal(t, entity.SyncStatusFailed, res.Apps.Status)
	require.Equal(t, entity.SyncStatusSuccess, res.Users.Status)
	require.Equal(t, 2, res.Users.Count)

	app, err := apps.GetBySource(conn.Id, "app-a")
	require.NoError(t, err)
	require.Equal(t, int8(1), app.Present)

	u2, err := users.GetBySource(conn.Id, "u-2")
	require.NoError(t, err)
	require.Equal(t, "bob", u2.Username)
}

func TestSyncDirectoryUnverifiedRelationNotRevoked(t *testing.T) {
	setupOctopDB(t)
	apps, users, auths := newCasdoorAppRepoForTest(), newCasdoorUserRepoForTest(), newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org", AppOwner: "built-in"}
	conn.Id = 4
	ctx := context.Background()
	ok := &fakeCasdoor{
		apps:  []casdoor.Application{enabledApp("app-a", "app-a", "org"), enabledApp("app-b", "app-b", "org")},
		users: []casdoor.User{enabledUser("u-1", "alice", "org")},
		appUsers: map[string]casdoor.RelationRead{
			"app-a": {Status: entity.AuthConfirmed, UserIDs: []string{"u-1"}},
			"app-b": {Status: entity.AuthConfirmed, UserIDs: []string{"u-1"}},
		},
	}
	require.Equal(t, entity.SyncStatusSuccess, SyncDirectory(ctx, ok, conn, apps, users, auths).Relations.Status)

	partial := &fakeCasdoor{
		apps:  []casdoor.Application{enabledApp("app-a", "app-a", "org"), enabledApp("app-b", "app-b", "org")},
		users: []casdoor.User{enabledUser("u-1", "alice", "org")},
		appUsers: map[string]casdoor.RelationRead{
			"app-a": {Status: entity.AuthUnverified, Failed: true, Reason: "http 403"},
			"app-b": {Status: entity.AuthConfirmed, UserIDs: []string{}},
		},
	}
	res := SyncDirectory(ctx, partial, conn, apps, users, auths)
	require.Equal(t, 1, res.Relations.Failed)
	require.Equal(t, 1, res.Relations.Confirmed)

	kept, err := auths.GetByKey(conn.Id, "app-a", "user", "u-1")
	require.NoError(t, err)
	require.Equal(t, entity.AuthConfirmed, kept.Status)
	require.NotNil(t, kept.LastConfirmedAt)

	revoked, err := auths.GetByKey(conn.Id, "app-b", "user", "u-1")
	require.NoError(t, err)
	require.Equal(t, entity.AuthRevoked, revoked.Status)

	appA, err := apps.GetBySource(conn.Id, "app-a")
	require.NoError(t, err)
	require.Equal(t, entity.AuthUnverified, appA.RelationStatus)
}

func TestSyncDirectoryUnsupportedRelationNotGuessed(t *testing.T) {
	setupOctopDB(t)
	apps, users, auths := newCasdoorAppRepoForTest(), newCasdoorUserRepoForTest(), newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 5
	client := &fakeCasdoor{
		apps:  []casdoor.Application{enabledApp("app-a", "app-a", "org")},
		users: []casdoor.User{enabledUser("u-1", "alice", "org")},
	}
	res := SyncDirectory(context.Background(), client, conn, apps, users, auths)
	require.Equal(t, 0, res.Relations.Confirmed)
	require.Greater(t, res.Relations.Unverified, 0)
	app, err := apps.GetBySource(conn.Id, "app-a")
	require.NoError(t, err)
	require.Equal(t, entity.AuthUnsupported, app.RelationStatus)
	page, err := auths.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(0), page.Total)
}

func TestSyncDirectorySameNameDifferentOrg(t *testing.T) {
	setupOctopDB(t)
	apps, users, auths := newCasdoorAppRepoForTest(), newCasdoorUserRepoForTest(), newCasdoorAuthRepoForTest()
	ctx := context.Background()

	connA := &entity.CasdoorConnection{Organization: "org-a"}
	connA.Id = 10
	clientA := &fakeCasdoor{
		apps: []casdoor.Application{
			enabledApp("portal", "portal", "org-a"),
			{SourceID: "portal", Name: "portal", Organization: "org-b", Owner: "built-in", Enabled: true, EnabledKnown: true},
		},
		users: []casdoor.User{
			enabledUser("alice", "alice", "org-a"),
			{SourceID: "alice", Username: "alice", Organization: "org-b", Enabled: true, EnabledKnown: true},
		},
	}
	require.Equal(t, entity.SyncStatusSuccess, SyncDirectory(ctx, clientA, connA, apps, users, auths).Users.Status)
	pageA, err := users.PageByCond(model.NewCond().Eq("connection_id", connA.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), pageA.Total)
	require.Equal(t, "org-a", pageA.List[0].Organization)

	appPageA, err := apps.PageByCond(model.NewCond().Eq("connection_id", connA.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), appPageA.Total)

	connB := &entity.CasdoorConnection{Organization: "org-b"}
	connB.Id = 11
	clientB := &fakeCasdoor{
		apps:  []casdoor.Application{enabledApp("portal", "portal", "org-b")},
		users: []casdoor.User{enabledUser("alice", "alice", "org-b")},
	}
	require.Equal(t, entity.SyncStatusSuccess, SyncDirectory(ctx, clientB, connB, apps, users, auths).Users.Status)
	pageB, err := users.PageByCond(model.NewCond().Eq("connection_id", connB.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), pageB.Total)
	require.Equal(t, "org-b", pageB.List[0].Organization)
	require.Equal(t, "alice", pageB.List[0].SourceId)
}

func TestSyncDirectoryDoesNotMergeSameName(t *testing.T) {
	setupOctopDB(t)
	apps, users, auths := newCasdoorAppRepoForTest(), newCasdoorUserRepoForTest(), newCasdoorAuthRepoForTest()
	conn := &entity.CasdoorConnection{Organization: "org"}
	conn.Id = 6
	client := &fakeCasdoor{
		users: []casdoor.User{
			enabledUser("id-1", "alice", "org"),
			enabledUser("id-2", "alice", "org"),
		},
	}
	require.Equal(t, entity.SyncStatusSuccess, SyncDirectory(context.Background(), client, conn, apps, users, auths).Users.Status)
	page, err := users.PageByCond(model.NewCond().Eq("connection_id", conn.Id), model.PageParam{PageNum: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
}

func TestSyncConcurrentSameConnection(t *testing.T) {
	setupOctopDB(t)
	connRow := &entity.CasdoorConnection{
		Name: "c1", ServerURL: "http://casdoor.local", Organization: "org",
		ClientId: "id", ClientSecretEnc: "enc", TimeoutSec: 5, SyncIntervalSec: 60, TLSVerify: 1,
	}
	connRepo := &testConnRepo{}
	require.NoError(t, connRepo.Insert(context.Background(), connRow))

	started := make(chan struct{})
	release := make(chan struct{})
	var listCalls atomic.Int32
	impl := &casdoorAppImpl{
		AppImpl:  base.AppImpl[*entity.CasdoorConnection, repository.CasdoorConnection]{Repo: connRepo},
		appRepo:  newCasdoorAppRepoForTest(),
		userRepo: newCasdoorUserRepoForTest(),
		authRepo: newCasdoorAuthRepoForTest(),
		newClient: func(casdoor.Config) casdoor.Client {
			return &blockClient{started: started, release: release, calls: &listCalls}
		},
	}

	var firstErr, secondErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, firstErr = impl.Sync(context.Background(), connRow.Id)
	}()
	<-started
	go func() {
		defer wg.Done()
		_, secondErr = impl.Sync(context.Background(), connRow.Id)
	}()
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	require.NoError(t, firstErr)
	require.Error(t, secondErr)
}

func TestTestConnectionReportsThreeProbes(t *testing.T) {
	impl := &casdoorAppImpl{
		newClient: func(casdoor.Config) casdoor.Client {
			return &fakeCasdoor{appErr: errors.New("casdoor GET /api/get-organization-applications: http 403 (forbidden)")}
		},
	}
	res, err := impl.TestConnection(context.Background(), &entity.CasdoorConnection{
		ServerURL: "http://casdoor.local", Organization: "org", ClientId: "id", TimeoutSec: 5,
	}, "plain-secret")
	require.NoError(t, err)
	require.Equal(t, entity.ProbeOK, res.Service.Status)
	require.Equal(t, entity.ProbeFail, res.Apps.Status)
	require.Equal(t, entity.ProbeOK, res.Users.Status)
	require.Contains(t, res.Apps.Reason, "403")
	require.NotContains(t, res.Apps.Reason, "plain-secret")
}

func TestSaveConnectionKeepsSecretWhenEmpty(t *testing.T) {
	setupOctopDB(t)
	impl := &casdoorAppImpl{
		AppImpl: base.AppImpl[*entity.CasdoorConnection, repository.CasdoorConnection]{Repo: &testConnRepo{}},
	}
	conn := &entity.CasdoorConnection{
		Name: "c", ServerURL: "http://casdoor.local", Organization: "org", AppOwner: "built-in",
		ClientId: "id", TimeoutSec: 5, SyncIntervalSec: 60,
	}
	require.NoError(t, impl.SaveConnection(context.Background(), conn, "first-secret"))
	require.NotEmpty(t, conn.ClientSecretEnc)
	require.Equal(t, int8(1), conn.CredentialConfigured)

	loaded, err := impl.GetById(conn.Id)
	require.NoError(t, err)
	enc := loaded.ClientSecretEnc
	loaded.AppOwner = "admin"
	require.NoError(t, impl.SaveConnection(context.Background(), loaded, ""))
	again, err := impl.GetById(conn.Id)
	require.NoError(t, err)
	require.Equal(t, enc, again.ClientSecretEnc)
	require.Equal(t, "admin", again.AppOwner)
}

type blockClient struct {
	started chan struct{}
	release chan struct{}
	calls   *atomic.Int32
	once    sync.Once
}

func (b *blockClient) TestService(context.Context) error { return nil }
func (b *blockClient) TestApps(context.Context) error    { return nil }
func (b *blockClient) TestUsers(context.Context) error   { return nil }
func (b *blockClient) ListApplications(context.Context, int, int) ([]casdoor.Application, int, error) {
	b.calls.Add(1)
	b.once.Do(func() { close(b.started) })
	<-b.release
	return []casdoor.Application{enabledApp("app-a", "app-a", "org")}, 1, nil
}
func (b *blockClient) ListUsers(context.Context, int, int) ([]casdoor.User, int, error) {
	return []casdoor.User{enabledUser("u-1", "alice", "org")}, 1, nil
}
func (b *blockClient) GetApplicationUsers(context.Context, string, string) casdoor.RelationRead {
	return casdoor.RelationRead{Status: entity.AuthUnsupported}
}

type testConnRepo struct {
	base.RepoImpl[*entity.CasdoorConnection]
}

func newCasdoorAppRepoForTest() repository.CasdoorApp { return &testAppRepo{} }

type testAppRepo struct {
	base.RepoImpl[*entity.CasdoorApplicationSnapshot]
}

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

type testUserRepo struct {
	base.RepoImpl[*entity.CasdoorUserSnapshot]
}

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

type testAuthRepo struct {
	base.RepoImpl[*entity.CasdoorAuthRelation]
}

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

func (r *testAuthRepo) MarkMissingRevoked(ctx context.Context, connectionId uint64, verifiedAppIDs []string, keepKeys []repository.AuthKey, now time.Time) error {
	if len(verifiedAppIDs) == 0 {
		return nil
	}
	rows, err := r.SelectByCond(model.NewCond().Eq("connection_id", connectionId).Eq("status", entity.AuthConfirmed).In("application_source_id", verifiedAppIDs))
	if err != nil {
		return err
	}
	keep := map[string]struct{}{}
	for _, k := range keepKeys {
		keep[k.AppSourceId+"|"+k.SubjectType+"|"+k.SubjectSourceId] = struct{}{}
	}
	verified := map[string]struct{}{}
	for _, id := range verifiedAppIDs {
		verified[id] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := verified[row.ApplicationSourceId]; !ok {
			continue
		}
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
