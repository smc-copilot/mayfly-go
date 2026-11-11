package application

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/business"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/octopapi"
	"mayfly-go/internal/pkg/config"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/global"
	"mayfly-go/pkg/i18n"
	"mayfly-go/pkg/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	i18n.AppendLangMsg(i18n.Zh_CN, imsg.Zh_CN)
	i18n.AppendLangMsg(i18n.En, imsg.En)
}

type stubInstanceAccess struct {
	inst *entity.OctopInstance
	err  error
}

func (s *stubInstanceAccess) GetVisible(_ context.Context, id uint64) (*entity.OctopInstance, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.inst == nil {
		return nil, errorx.NewBizI(context.Background(), imsg.ErrInstanceNotFound)
	}
	out := *s.inst
	if out.Id == 0 {
		out.Id = id
	}
	return &out, nil
}

type fakePreview struct {
	probeErr error
	files    []entity.BizFileEntry
	content  *entity.BizFileContent
	listErr  error
	getErr   error
}

func (f *fakePreview) ListSkillFiles(context.Context, string, string) ([]entity.BizFileEntry, error) {
	return f.files, f.listErr
}
func (f *fakePreview) ListTemplateFiles(context.Context, string) ([]entity.BizFileEntry, error) {
	return f.files, f.listErr
}
func (f *fakePreview) GetFileContent(context.Context, string) (*entity.BizFileContent, error) {
	return f.content, f.getErr
}
func (f *fakePreview) Probe(context.Context) error { return f.probeErr }

type testReadConnRepo struct {
	base.RepoImpl[*entity.OctopReadConnection]
}

func newTestReadConnRepo() repository.ReadConnection { return &testReadConnRepo{} }

func (r *testReadConnRepo) GetByInstanceType(_ context.Context, instanceId uint64, connType string) (*entity.OctopReadConnection, error) {
	var row entity.OctopReadConnection
	err := r.GetByCond(model.NewCond().Eq("instance_id", instanceId).Eq("type", connType).Dest(&row))
	if err != nil {
		if model.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *testReadConnRepo) ListByInstance(_ context.Context, instanceId uint64) ([]*entity.OctopReadConnection, error) {
	return r.SelectByCond(model.NewCond().Eq("instance_id", instanceId))
}

func setupReadConnDB(t *testing.T) {
	t.Helper()
	if config.Conf == nil {
		config.Conf = &config.Config{}
	}
	prevAes := config.Conf.Aes
	config.Conf.Aes = config.Aes{Key: "0123456789abcdef"}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&entity.OctopReadConnection{}, &entity.OctopInstance{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	prev := global.Db
	global.Db = db
	t.Cleanup(func() {
		global.Db = prev
		config.Conf.Aes = prevAes
	})
}

func newReadConnApp(t *testing.T, preview *fakePreview) *readConnectionAppImpl {
	t.Helper()
	setupReadConnDB(t)
	return &readConnectionAppImpl{
		connRepo:    newTestReadConnRepo(),
		instanceApp: &stubInstanceAccess{inst: &entity.OctopInstance{DbId: 1, DbName: "octop"}},
		newPreview:  func(octopapi.PreviewConfig) business.PreviewClient { return preview },
	}
}

func TestReadConnectionSavePreserveAndClearSecret(t *testing.T) {
	preview := &fakePreview{}
	app := newReadConnApp(t, preview)
	ctx := context.Background()

	created, err := app.Save(ctx, 7, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", Secret: "s3cret", Revision: 0,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.Revision)
	require.True(t, created.CredentialConfigured)
	require.Equal(t, entity.VerifyStateNotVerified, created.VerifyState)

	kept, err := app.Save(ctx, 7, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", Secret: "", Revision: 1,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), kept.Revision)
	require.True(t, kept.CredentialConfigured)

	row, err := app.connRepo.GetByInstanceType(ctx, 7, entity.ReadConnTypeOctopAPI)
	require.NoError(t, err)
	require.NotEmpty(t, row.SecretEnc)

	cleared, err := app.Save(ctx, 7, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", ClearSecret: true, Revision: 2,
	})
	require.NoError(t, err)
	require.False(t, cleared.CredentialConfigured)
	row, err = app.connRepo.GetByInstanceType(ctx, 7, entity.ReadConnTypeOctopAPI)
	require.NoError(t, err)
	require.Empty(t, row.SecretEnc)
}

func TestReadConnectionRevisionConflict(t *testing.T) {
	app := newReadConnApp(t, &fakePreview{})
	ctx := context.Background()
	_, err := app.Save(ctx, 3, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeLangfuse, BaseURL: "https://lf.example", PublicKey: "pk", Secret: "sk", Revision: 0,
	})
	require.NoError(t, err)

	_, err = app.Save(ctx, 3, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeLangfuse, BaseURL: "https://lf.example", PublicKey: "pk2", Revision: 0,
	})
	require.Error(t, err)
	var be *errorx.BizError
	require.ErrorAs(t, err, &be)
	require.Contains(t, be.Error(), "已被他人修改")
}

func TestReadConnectionGetAndListEmpty(t *testing.T) {
	app := newReadConnApp(t, &fakePreview{})
	ctx := context.Background()

	view, err := app.Get(ctx, 9, entity.ReadConnTypeOctopAPI)
	require.NoError(t, err)
	require.Equal(t, entity.VerifyStateNotConfigured, view.VerifyState)
	require.Equal(t, int64(0), view.Revision)
	require.False(t, view.CredentialConfigured)

	list, err := app.List(ctx, 9)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, entity.ReadConnTypeOctopAPI, list[0].Type)
	require.Equal(t, entity.ReadConnTypeLangfuse, list[1].Type)
}

func TestReadConnectionTestPreviewAndLangfuse(t *testing.T) {
	preview := &fakePreview{}
	app := newReadConnApp(t, preview)
	ctx := context.Background()

	ok, err := app.Test(ctx, 1, &dto.ReadConnectionTest{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", Secret: "tok",
	})
	require.NoError(t, err)
	require.True(t, ok.Ok)
	require.Equal(t, entity.VerifyStateReady, ok.State)

	preview.probeErr = &octopapi.PreviewHTTPError{StatusCode: http.StatusUnauthorized}
	denied, err := app.Test(ctx, 1, &dto.ReadConnectionTest{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", Secret: "bad",
	})
	require.NoError(t, err)
	require.False(t, denied.Ok)
	require.Equal(t, entity.VerifyStateForbidden, denied.State)

	lfSrv := traceFixture(t, "2.92.0", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	defer lfSrv.Close()
	lf, err := app.Test(ctx, 1, &dto.ReadConnectionTest{
		Type: entity.ReadConnTypeLangfuse, BaseURL: lfSrv.URL, PublicKey: "pk", Secret: "sk",
	})
	require.NoError(t, err)
	require.True(t, lf.Ok)
	require.Equal(t, entity.VerifyStateReady, lf.State)
	require.Equal(t, "2.92.0", lf.Version)
	require.Equal(t, "project-1", lf.ProjectId)
	require.Len(t, lf.Checks, 4)
}

func TestReadConnectionTestRecordsVerifyOnSavedRevision(t *testing.T) {
	preview := &fakePreview{}
	app := newReadConnApp(t, preview)
	ctx := context.Background()

	saved, err := app.Save(ctx, 4, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", Secret: "tok", Revision: 0,
	})
	require.NoError(t, err)

	res, err := app.Test(ctx, 4, &dto.ReadConnectionTest{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", Revision: saved.Revision,
	})
	require.NoError(t, err)
	require.True(t, res.Ok)

	row, err := app.connRepo.GetByInstanceType(ctx, 4, entity.ReadConnTypeOctopAPI)
	require.NoError(t, err)
	require.Equal(t, entity.VerifyStateReady, row.VerifyState)
	require.NotNil(t, row.VerifiedAt)
}

func TestClassifyPreviewErr(t *testing.T) {
	state, _ := classifyPreviewErr(nil)
	require.Equal(t, entity.VerifyStateReady, state)
	state, msg := classifyPreviewErr(&octopapi.PreviewHTTPError{StatusCode: 500})
	require.Equal(t, entity.VerifyStateFailed, state)
	require.Contains(t, msg, "500")
	state, _ = classifyPreviewErr(errors.Join(octopapi.ErrPreviewConfig, errors.New("bad url")))
	require.Equal(t, entity.VerifyStateFailed, state)
	state, msg = classifyPreviewErr(context.DeadlineExceeded)
	require.Equal(t, entity.VerifyStateFailed, state)
	require.Equal(t, "请求超时", msg)
}

func TestValidateReadBaseURL(t *testing.T) {
	require.NoError(t, validateReadBaseURL(""))
	require.NoError(t, validateReadBaseURL("https://example.com/path"))
	require.Error(t, validateReadBaseURL("ftp://example.com"))
	require.Error(t, validateReadBaseURL("https://user:pass@example.com"))
	require.Error(t, validateReadBaseURL("https://example.com?x=1"))
}

func TestReadConnectionGetMasksSecrets(t *testing.T) {
	app := newReadConnApp(t, &fakePreview{})
	ctx := context.Background()
	_, err := app.Save(ctx, 5, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeOctopAPI, BaseURL: "https://octop.example", Secret: "top-secret", Revision: 0,
	})
	require.NoError(t, err)
	view, err := app.Get(ctx, 5, entity.ReadConnTypeOctopAPI)
	require.NoError(t, err)
	require.True(t, view.CredentialConfigured)
	// DTO 不含 Secret 字段；仓储密文也不应出现在 JSON 视图结构上
	require.NotContains(t, view.BaseURL, "top-secret")
	require.WithinDuration(t, time.Now(), *view.CredentialRotatedAt, time.Minute)
}
