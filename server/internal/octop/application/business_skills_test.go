package application

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"mayfly-go/internal/octop/domain/business"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/infra/businessread"
	"mayfly-go/internal/octop/infra/octopapi"
)

const packageSkillResponse = `[
 {"slug":"contract-review","name":"Contract Review","description":"Review contracts"},
 {"slug":"employment-contract","name":"Employment Contract"},
 {"slug":"hetongzhushou","name":"hetongzhushou"},
 {"slug":"legal-advisor","name":"Legal Advisor"},
 {"slug":"legal-doc-writer","name":"Legal Doc Writer"},
 {"slug":"nathan-legal-os-pro","name":"nathan-legal-os-pro"}
]`

func packageSkillsApp(t *testing.T, rd *fakeReader, handler http.HandlerFunc) *businessAppImpl {
	t.Helper()
	setupReadConnDB(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	repo := newTestReadConnRepo()
	require.NoError(t, repo.Insert(context.Background(), &entity.OctopReadConnection{
		InstanceId: 1, Type: entity.ReadConnTypeOctopAPI,
		BaseURL: srv.URL, SecretEnc: encSecret(t, "skill-read-token"), CredentialConfigured: 1,
	}))
	app := newBusinessApp(nil, rd, nil, repo)
	app.newPreview = func(cfg octopapi.PreviewConfig) business.PreviewClient {
		return octopapi.NewPreviewClient(cfg)
	}
	return app
}

func TestBusinessPackageSkillsReadFromOctopAPI(t *testing.T) {
	app := packageSkillsApp(t, &fakeReader{pkg: &entity.BizSkillPackage{PackageId: "TN2R8B", CreatedBy: "1"}},
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/skill-packages/TN2R8B/skills" || r.Header.Get("Authorization") != "Bearer skill-read-token" {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			if _, err := io.WriteString(w, packageSkillResponse); err != nil {
				t.Error(err)
			}
		})
	q := &entity.BusinessQuery{}
	q.PageNum, q.PageSize = 1, 2
	page, err := app.PageSkills(context.Background(), 1, "1", "TN2R8B", q)
	require.NoError(t, err)
	require.EqualValues(t, 6, page.Total)
	require.Len(t, page.List, 2)
	require.Equal(t, "contract-review", page.List[0].Slug)
	require.Equal(t, "employment-contract", page.List[1].Slug)
	require.Equal(t, "octop_api", page.Source.Source)
	require.Equal(t, entity.SourceStateReady, page.Source.State)

	q.Keyword, q.PageNum = "LEGAL", 2
	page, err = app.PageSkills(context.Background(), 1, "1", "TN2R8B", q)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	require.Len(t, page.List, 1)
	require.Equal(t, "nathan-legal-os-pro", page.List[0].Slug)

	detail, err := app.GetSkill(context.Background(), 1, "1", "TN2R8B", "contract-review")
	require.NoError(t, err)
	require.NotNil(t, detail.Item)
	require.Equal(t, "Contract Review", detail.Item.Name)
}

func TestBusinessPackageSkillsCheckOwnerBeforeAPI(t *testing.T) {
	app := packageSkillsApp(t, &fakeReader{err: businessread.ErrNotFound}, func(w http.ResponseWriter, r *http.Request) {
		t.Error("must not contact Octop for another user's package")
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	_, err := app.PageSkills(context.Background(), 1, "2", "TN2R8B", nil)
	require.Error(t, err)
	_, err = app.GetSkill(context.Background(), 1, "2", "TN2R8B", "contract-review")
	require.Error(t, err)
}

func TestBusinessPackageSkillsExposeAPIUnavailable(t *testing.T) {
	app := packageSkillsApp(t, &fakeReader{pkg: &entity.BizSkillPackage{PackageId: "TN2R8B"}}, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	page, err := app.PageSkills(context.Background(), 1, "1", "TN2R8B", nil)
	require.NoError(t, err)
	require.Equal(t, "octop_api", page.Source.Source)
	require.Equal(t, entity.SourceStateForbidden, page.Source.State)
	require.Empty(t, page.List)
}

func TestBusinessPackageSkillsRequireConfiguredAPI(t *testing.T) {
	setupReadConnDB(t)
	app := newBusinessApp(nil, &fakeReader{pkg: &entity.BizSkillPackage{PackageId: "TN2R8B"}}, &fakePreview{}, newTestReadConnRepo())
	page, err := app.PageSkills(context.Background(), 1, "1", "TN2R8B", nil)
	require.NoError(t, err)
	require.Equal(t, "octop_api", page.Source.Source)
	require.Equal(t, entity.SourceStateNotConfigured, page.Source.State)
	require.Empty(t, page.List)
}
