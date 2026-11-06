package application

import (
	"context"
	"encoding/json"
	"testing"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/octopapi"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/i18n"
	"mayfly-go/pkg/model"

	"github.com/stretchr/testify/require"
)

func init() {
	i18n.AppendLangMsg(i18n.Zh_CN, imsg.Zh_CN)
}

type testInstanceRepo struct {
	base.RepoImpl[*entity.OctopInstance]
}

func (r *testInstanceRepo) ListEnabled(ctx context.Context) ([]*entity.OctopInstance, error) {
	_ = ctx
	return r.SelectByCond(model.NewCond().Eq("collect_enabled", 1))
}

func newBindFixture(t *testing.T) (*instanceAppImpl, repository.CasdoorApp) {
	t.Helper()
	setupOctopDB(t)
	apps := newCasdoorAppRepoForTest()
	a := &instanceAppImpl{appRepo: apps}
	a.Repo = &testInstanceRepo{}
	return a, apps
}

func adminCtx() context.Context {
	return contextx.NewLoginAccount(&model.LoginAccount{Id: 1, Username: "admin"})
}

func opsCtx() context.Context {
	return contextx.NewLoginAccount(&model.LoginAccount{Id: 99, Username: "ops"})
}

func saveInst(t *testing.T, a *instanceAppImpl, inst *entity.OctopInstance) *entity.OctopInstance {
	t.Helper()
	require.NoError(t, a.Save(context.Background(), inst))
	return inst
}

func TestListBindMatchesExactExternalVsInternal(t *testing.T) {
	a, apps := newBindFixture(t)
	ctx := adminCtx()
	require.NoError(t, apps.Save(ctx, &entity.CasdoorApplicationSnapshot{
		ConnectionId: 1, SourceId: "app-1", Name: "octop-sso",
		RedirectURIs: encodeRedirectURIs([]string{"https://octop.example.com/api/auth/oidc/callback"}),
	}))
	app, err := apps.GetBySource(1, "app-1")
	require.NoError(t, err)

	internal := saveInst(t, a, &entity.OctopInstance{Code: "in", Name: "internal", ApiURL: "http://10.0.0.8:8080"})
	external := saveInst(t, a, &entity.OctopInstance{
		Code: "ex", Name: "external", ApiURL: "http://10.0.0.8:8080",
		CallbackURL: "https://octop.example.com/api/auth/oidc/callback",
	})

	matches, err := a.ListBindMatches(ctx, 1)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, entity.BindMatchUnique, matches[0].MatchStatus)
	require.Len(t, matches[0].Candidates, 1)
	require.Equal(t, external.Id, matches[0].Candidates[0].InstanceId)
	require.Equal(t, entity.MatchBasisCallbackURL, matches[0].Candidates[0].MatchBasis)
	require.Equal(t, app.Id, matches[0].AppSnapshotId)
	require.NotEqual(t, internal.Id, matches[0].Candidates[0].InstanceId)
}

func TestListBindMatchesAmbiguousAndNone(t *testing.T) {
	a, apps := newBindFixture(t)
	ctx := adminCtx()
	require.NoError(t, apps.Save(ctx, &entity.CasdoorApplicationSnapshot{
		ConnectionId: 1, SourceId: "app-multi", Name: "multi",
		RedirectURIs: encodeRedirectURIs([]string{
			"https://a.example.com/api/auth/oidc/callback",
			"https://b.example.com/api/auth/oidc/callback",
		}),
	}))
	require.NoError(t, apps.Save(ctx, &entity.CasdoorApplicationSnapshot{
		ConnectionId: 1, SourceId: "app-empty", Name: "empty",
	}))
	saveInst(t, a, &entity.OctopInstance{Code: "a", Name: "A", CallbackURL: "https://a.example.com"})
	saveInst(t, a, &entity.OctopInstance{Code: "b", Name: "B", CallbackURL: "https://b.example.com"})

	matches, err := a.ListBindMatches(ctx, 1)
	require.NoError(t, err)
	bySource := map[string]*dtoMatch{}
	for _, m := range matches {
		bySource[m.SourceId] = &dtoMatch{Status: m.MatchStatus, N: len(m.Candidates)}
	}
	require.Equal(t, entity.BindMatchAmbiguous, bySource["app-multi"].Status)
	require.Equal(t, 2, bySource["app-multi"].N)
	require.Equal(t, entity.BindMatchUnverified, bySource["app-empty"].Status)
}

type dtoMatch struct {
	Status string
	N      int
}

func TestBindCasdoorAppRequiresOverwriteAndPersists(t *testing.T) {
	a, apps := newBindFixture(t)
	ctx := adminCtx()
	require.NoError(t, apps.Save(ctx, &entity.CasdoorApplicationSnapshot{ConnectionId: 3, SourceId: "old", Name: "old"}))
	require.NoError(t, apps.Save(ctx, &entity.CasdoorApplicationSnapshot{ConnectionId: 3, SourceId: "new", Name: "new"}))
	oldApp, err := apps.GetBySource(3, "old")
	require.NoError(t, err)
	newApp, err := apps.GetBySource(3, "new")
	require.NoError(t, err)

	inst := saveInst(t, a, &entity.OctopInstance{Code: "b1", Name: "bind"})
	require.NoError(t, a.BindCasdoorApp(ctx, inst.Id, oldApp.Id, false))
	got, err := a.GetById(inst.Id)
	require.NoError(t, err)
	require.Equal(t, oldApp.Id, got.CasdoorAppSnapshotId)
	require.Equal(t, uint64(3), got.CasdoorConnectionId)

	err = a.BindCasdoorApp(ctx, inst.Id, newApp.Id, false)
	require.Error(t, err)
	var be *errorx.BizError
	require.ErrorAs(t, err, &be)
	got, err = a.GetById(inst.Id)
	require.NoError(t, err)
	require.Equal(t, oldApp.Id, got.CasdoorAppSnapshotId)

	require.NoError(t, a.BindCasdoorApp(ctx, inst.Id, newApp.Id, true))
	got, err = a.GetById(inst.Id)
	require.NoError(t, err)
	require.Equal(t, newApp.Id, got.CasdoorAppSnapshotId)
}

func TestBindCasdoorAppDeniedWithoutInstanceAccess(t *testing.T) {
	a, apps := newBindFixture(t)
	require.NoError(t, apps.Save(context.Background(), &entity.CasdoorApplicationSnapshot{ConnectionId: 1, SourceId: "app", Name: "app"}))
	app, err := apps.GetBySource(1, "app")
	require.NoError(t, err)
	inst := saveInst(t, a, &entity.OctopInstance{Code: "x", Name: "no-machine"})
	err = a.BindCasdoorApp(opsCtx(), inst.Id, app.Id, false)
	require.Error(t, err)
}

func TestSaveInstanceKeepsTokenAndBinding(t *testing.T) {
	a, _ := newBindFixture(t)
	ctx := adminCtx()
	inst := &entity.OctopInstance{Code: "tok", Name: "token", MachineId: 1, CallbackURL: "https://octop.example.com"}
	require.NoError(t, a.SaveInstance(ctx, inst, "plain-jwt-token"))
	got, err := a.GetById(inst.Id)
	require.NoError(t, err)
	require.Equal(t, int8(1), got.ApiTokenConfigured)
	require.Equal(t, "plain-jwt-token", got.ApiTokenEnc)
	require.Equal(t, "https://octop.example.com", got.CallbackURL)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "plain-jwt-token")
	require.NotContains(t, string(raw), "apiTokenEnc")
	require.NotContains(t, string(raw), `"apiToken"`)

	got.CasdoorAppSnapshotId = 88
	require.NoError(t, a.UpdateById(ctx, got))
	next := &entity.OctopInstance{Name: "token-renamed", MachineId: 1, CallbackURL: "https://octop.example.com"}
	next.Id = inst.Id
	require.NoError(t, a.SaveInstance(ctx, next, ""))
	saved, err := a.GetById(inst.Id)
	require.NoError(t, err)
	require.Equal(t, "tok", saved.Code)
	require.Equal(t, uint64(88), saved.CasdoorAppSnapshotId)
	require.Equal(t, "plain-jwt-token", saved.ApiTokenEnc)
}

func TestListOctopUsersFiltersFullListAndWhitelists(t *testing.T) {
	a, _ := newBindFixture(t)
	ctx := adminCtx()
	inst := saveInst(t, a, &entity.OctopInstance{
		Code: "u1", Name: "users", ApiURL: "http://10.0.0.8:8080",
		ApiTokenEnc: "jwt-a", ApiTokenConfigured: 1,
	})
	other := saveInst(t, a, &entity.OctopInstance{
		Code: "u2", Name: "other", ApiURL: "http://10.0.0.9:8080",
		ApiTokenEnc: "jwt-b", ApiTokenConfigured: 1,
	})
	var seenURL, seenToken string
	a.fetchUsers = func(_ context.Context, apiURL, token string) octopapi.UsersResult {
		seenURL, seenToken = apiURL, token
		if apiURL == other.ApiURL {
			return octopapi.UsersResult{Status: entity.UserListOK, Rows: []map[string]any{
				{"id": 99, "username": "other-user", "email": "o@x.com", "disabled": false},
			}}
		}
		return octopapi.UsersResult{Status: entity.UserListOK, Rows: []map[string]any{
			{"id": 1, "username": "alice", "display_name": "Alice", "email": "alice@ex.com", "disabled": false, "password_hash": "x", "token": "leak"},
			{"id": 2, "username": "bob", "display_name": "Bob", "email": "bob@ex.com", "disabled": true},
			{"id": 3, "username": "cara", "display_name": "Cara", "email": "cara@other.com", "disabled": false},
		}}
	}

	page, err := a.ListOctopUsers(ctx, &entity.OctopUserQuery{
		PageParam: model.PageParam{PageNum: 1, PageSize: 1}, InstanceId: inst.Id, Username: "a", Email: "@ex.com",
	})
	require.NoError(t, err)
	require.Equal(t, entity.UserListOK, page.Status)
	require.Equal(t, entity.UserFilterModeLocalFull, page.FilterMode)
	require.Equal(t, inst.ApiURL, seenURL)
	require.Equal(t, "jwt-a", seenToken)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, page.List, 1)
	require.Equal(t, "1", page.List[0].ID)
	require.Equal(t, "alice", page.List[0].Username)
	require.Equal(t, entity.UserStatusEnabled, page.List[0].Status)
	raw, err := json.Marshal(page.List[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), "password")
	require.NotContains(t, string(raw), "leak")
	require.NotContains(t, string(raw), "token")

	page, err = a.ListOctopUsers(ctx, &entity.OctopUserQuery{InstanceId: other.Id})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, "other-user", page.List[0].Username)
	require.Equal(t, other.ApiURL, seenURL)
	require.Equal(t, "jwt-b", seenToken)
}

func TestListOctopUsersUnconfiguredForbiddenAndDenied(t *testing.T) {
	a, _ := newBindFixture(t)
	ctx := adminCtx()
	empty := saveInst(t, a, &entity.OctopInstance{Code: "e", Name: "empty", ApiURL: "http://10.0.0.8:8080"})
	page, err := a.ListOctopUsers(ctx, &entity.OctopUserQuery{InstanceId: empty.Id})
	require.NoError(t, err)
	require.Equal(t, entity.UserListUnconfigured, page.Status)

	denied := saveInst(t, a, &entity.OctopInstance{Code: "d", Name: "denied", ApiURL: "http://10.0.0.8:8080", ApiTokenEnc: "t", ApiTokenConfigured: 1})
	_, err = a.ListOctopUsers(opsCtx(), &entity.OctopUserQuery{InstanceId: denied.Id})
	require.Error(t, err)

	a.fetchUsers = func(context.Context, string, string) octopapi.UsersResult {
		return octopapi.UsersResult{Status: entity.UserListForbidden, Message: "http 403"}
	}
	ready := saveInst(t, a, &entity.OctopInstance{Code: "f", Name: "fail", ApiURL: "http://10.0.0.8:8080", ApiTokenEnc: "t", ApiTokenConfigured: 1})
	page, err = a.ListOctopUsers(ctx, &entity.OctopUserQuery{InstanceId: ready.Id, Username: "alice"})
	require.NoError(t, err)
	require.Equal(t, entity.UserListForbidden, page.Status)
	require.Empty(t, page.List)

	_, err = a.ListOctopUsers(ctx, &entity.OctopUserQuery{})
	require.Error(t, err)
}

func TestProjectOctopUserDropsUnknownSensitive(t *testing.T) {
	u := projectOctopUser(map[string]any{
		"id": 3, "username": "u", "display_name": "U", "email": "u@e.com",
		"disabled": true, "password_hash": "x", "access_token": "t", "role": "admin",
	})
	require.Equal(t, "3", u.ID)
	require.Equal(t, entity.UserStatusDisabled, u.Status)
	raw, err := json.Marshal(u)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "password")
	require.NotContains(t, string(raw), "access_token")
	require.NotContains(t, string(raw), "admin")
}
