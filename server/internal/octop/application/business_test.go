package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/business"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/infra/businessread"
	"mayfly-go/internal/octop/infra/octopapi"
	"mayfly-go/internal/pkg/utils"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/model"

	"github.com/stretchr/testify/require"
)

func encSecret(t *testing.T, plain string) string {
	t.Helper()
	enc, err := utils.PwdAesEncrypt(plain)
	require.NoError(t, err)
	return enc
}

type fakeReader struct {
	users       *model.PageResult[*entity.BizUser]
	options     *model.PageResult[*entity.BizUserOption]
	user        *entity.BizUser
	experts     *model.PageResult[*entity.BizExpert]
	expert      *entity.BizExpert
	threads     *model.PageResult[*entity.BizThread]
	thread      *entity.BizThread
	templates   *model.PageResult[*entity.BizExpertTemplate]
	template    *entity.BizExpertTemplate
	packages    *model.PageResult[*entity.BizSkillPackage]
	pkg         *entity.BizSkillPackage
	skills      *model.PageResult[*entity.BizSkill]
	skill       *entity.BizSkill
	meta        *entity.SourceMeta
	err         error
	expertCnt   entity.StatValue
	threadCnt   entity.StatValue
	templateCnt entity.StatValue
	pkgCnt      entity.StatValue
}

func readyMeta() *entity.SourceMeta {
	now := time.Now()
	complete := true
	return &entity.SourceMeta{Source: businessread.SourceName, State: entity.SourceStateReady, ReadAt: &now, Complete: &complete}
}

func (f *fakeReader) pageMeta() (*entity.SourceMeta, error) {
	if f.err != nil {
		return f.meta, f.err
	}
	if f.meta != nil {
		return f.meta, nil
	}
	return readyMeta(), nil
}

func (f *fakeReader) PageUsers(context.Context, *entity.BusinessQuery) (*model.PageResult[*entity.BizUser], *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.users, m, err
}
func (f *fakeReader) PageUserOptions(context.Context, *entity.BusinessQuery) (*model.PageResult[*entity.BizUserOption], *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.options, m, err
}
func (f *fakeReader) GetUser(context.Context, string) (*entity.BizUser, *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.user, m, err
}
func (f *fakeReader) CountExperts(context.Context, string) (entity.StatValue, error) {
	return f.expertCnt, nil
}
func (f *fakeReader) CountThreads(context.Context, string) (entity.StatValue, error) {
	return f.threadCnt, nil
}
func (f *fakeReader) CountTemplates(context.Context, string) (entity.StatValue, error) {
	return f.templateCnt, nil
}
func (f *fakeReader) CountSkillPackages(context.Context, string) (entity.StatValue, error) {
	return f.pkgCnt, nil
}
func (f *fakeReader) PageExperts(context.Context, string, *entity.BusinessQuery) (*model.PageResult[*entity.BizExpert], *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.experts, m, err
}
func (f *fakeReader) GetExpert(context.Context, string, string) (*entity.BizExpert, *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.expert, m, err
}
func (f *fakeReader) PageThreads(context.Context, string, *entity.BusinessQuery) (*model.PageResult[*entity.BizThread], *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.threads, m, err
}
func (f *fakeReader) GetThread(context.Context, string, string) (*entity.BizThread, *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.thread, m, err
}
func (f *fakeReader) PageExpertTemplates(context.Context, string, *entity.BusinessQuery) (*model.PageResult[*entity.BizExpertTemplate], *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.templates, m, err
}
func (f *fakeReader) GetExpertTemplate(context.Context, string, string) (*entity.BizExpertTemplate, *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.template, m, err
}
func (f *fakeReader) PageSkillPackages(context.Context, string, *entity.BusinessQuery) (*model.PageResult[*entity.BizSkillPackage], *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.packages, m, err
}
func (f *fakeReader) GetSkillPackage(context.Context, string, string) (*entity.BizSkillPackage, *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.pkg, m, err
}
func (f *fakeReader) PageSkills(context.Context, string, string, *entity.BusinessQuery) (*model.PageResult[*entity.BizSkill], *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.skills, m, err
}
func (f *fakeReader) GetSkill(context.Context, string, string, string) (*entity.BizSkill, *entity.SourceMeta, error) {
	m, err := f.pageMeta()
	return f.skill, m, err
}

var _ business.Reader = (*fakeReader)(nil)

func newBusinessApp(inst *entity.OctopInstance, rd *fakeReader, preview *fakePreview, repo repository.ReadConnection) *businessAppImpl {
	if inst == nil {
		inst = &entity.OctopInstance{DbId: 11, DbName: "octop"}
		inst.Id = 1
	}
	app := &businessAppImpl{
		instanceApp: &stubInstanceAccess{inst: inst},
		connRepo:    repo,
		newReader: func(context.Context, *entity.OctopInstance) (business.Reader, error) {
			return rd, nil
		},
		newPreview: func(octopapi.PreviewConfig) business.PreviewClient { return preview },
	}
	return app
}

func TestBusinessPageUsersNotConfigured(t *testing.T) {
	inst := &entity.OctopInstance{}
	inst.Id = 2
	app := newBusinessApp(inst, &fakeReader{}, &fakePreview{}, nil)
	page, err := app.PageUsers(context.Background(), 2, &entity.BusinessQuery{})
	require.NoError(t, err)
	require.Empty(t, page.List)
	require.Equal(t, entity.SourceStateNotConfigured, page.Source.State)
}

func TestBusinessPageUsersReady(t *testing.T) {
	rd := &fakeReader{
		users: &model.PageResult[*entity.BizUser]{
			Total: 1, PageNum: 1, PageSize: 10,
			List: []*entity.BizUser{{UserId: "1", Username: "alice"}},
		},
	}
	app := newBusinessApp(nil, rd, &fakePreview{}, nil)
	page, err := app.PageUsers(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, "alice", page.List[0].Username)
	require.Equal(t, entity.SourceStateReady, page.Source.State)
}

func TestBusinessPageUsersSourceFailureNotZero(t *testing.T) {
	rd := &fakeReader{
		err:  errors.New("db down"),
		meta: &entity.SourceMeta{State: entity.SourceStateFailed, Message: "query failed"},
	}
	app := newBusinessApp(nil, rd, &fakePreview{}, nil)
	page, err := app.PageUsers(context.Background(), 1, &entity.BusinessQuery{})
	require.NoError(t, err)
	require.Empty(t, page.List)
	require.Equal(t, int64(0), page.Total)
	require.Equal(t, entity.SourceStateFailed, page.Source.State)
}

func TestBusinessGetUserDetailAndCounts(t *testing.T) {
	now := time.Now()
	rd := &fakeReader{
		user:        &entity.BizUser{UserId: "9", Username: "bob"},
		expertCnt:   entity.NewStatOK(2, now, "experts"),
		threadCnt:   entity.NewStatOK(3, now, "threads"),
		templateCnt: entity.NewStatOK(1, now, "templates"),
		pkgCnt:      entity.NewStatOK(4, now, "packages"),
	}
	app := newBusinessApp(nil, rd, &fakePreview{}, nil)
	detail, err := app.GetUserDetail(context.Background(), 1, "9")
	require.NoError(t, err)
	require.Equal(t, "bob", detail.User.Username)
	require.Equal(t, int64(2), *detail.ExpertCount.Value)
	require.Equal(t, int64(3), *detail.ThreadCount.Value)
	require.Equal(t, int64(1), *detail.TemplateCount.Value)
	require.Equal(t, int64(4), *detail.SkillPackageCount.Value)
}

func TestBusinessGetUserNotFound(t *testing.T) {
	rd := &fakeReader{err: businessread.ErrNotFound}
	app := newBusinessApp(nil, rd, &fakePreview{}, nil)
	_, err := app.GetUserDetail(context.Background(), 1, "404")
	require.Error(t, err)
	var be *errorx.BizError
	require.ErrorAs(t, err, &be)
	require.Contains(t, be.Error(), "不存在")
}

func TestBusinessGetExpertContentDenied(t *testing.T) {
	app := newBusinessApp(nil, &fakeReader{expert: &entity.BizExpert{ExpertId: "e1"}}, &fakePreview{}, nil)
	_, err := app.GetExpert(context.Background(), 1, "1", "e1", true, false)
	require.Error(t, err)
	var be *errorx.BizError
	require.ErrorAs(t, err, &be)
	require.Contains(t, be.Error(), "正文")
}

func TestBusinessGetExpertStripsPromptWithoutContent(t *testing.T) {
	rd := &fakeReader{expert: &entity.BizExpert{
		ExpertId: "e1",
		Config:   &entity.BizExpertConfig{PromptPreview: "secret-prompt"},
	}}
	app := newBusinessApp(nil, rd, &fakePreview{}, nil)
	detail, err := app.GetExpert(context.Background(), 1, "1", "e1", false, false)
	require.NoError(t, err)
	require.Empty(t, detail.Item.Config.PromptPreview)
}

func TestBusinessCapabilities(t *testing.T) {
	setupReadConnDB(t)
	repo := newTestReadConnRepo()
	rd := &fakeReader{options: &model.PageResult[*entity.BizUserOption]{List: []*entity.BizUserOption{}}}
	app := newBusinessApp(nil, rd, &fakePreview{}, repo)

	caps, err := app.Capabilities(context.Background(), 1, dto.CapabilityScope{
		BusinessDB: true, OctopPreview: true, Langfuse: true,
	})
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, caps.BusinessDB.State)
	require.Equal(t, entity.SourceStateNotConfigured, caps.OctopPreview.State)
	require.Equal(t, entity.SourceStateNotConfigured, caps.Langfuse.State)
}

func TestBusinessGetSkillAttachesPreviewFiles(t *testing.T) {
	setupReadConnDB(t)
	repo := newTestReadConnRepo()
	ctx := context.Background()
	require.NoError(t, repo.Insert(ctx, &entity.OctopReadConnection{
		InstanceId: 1, Type: entity.ReadConnTypeOctopAPI,
		BaseURL: "https://octop.example", SecretEnc: encSecret(t, "tok"), CredentialConfigured: 1, Revision: 1,
		VerifyState: entity.VerifyStateReady,
	}))

	rd := &fakeReader{
		pkg:   &entity.BizSkillPackage{PackageId: "p1", CreatedBy: "1"},
		skill: &entity.BizSkill{Slug: "s1", Name: "Skill"},
	}
	preview := &fakePreview{skills: []*entity.BizSkill{{Slug: "s1", Name: "Skill"}}, files: []entity.BizFileEntry{{FileId: "f1", Name: "SKILL.md"}}}
	app := newBusinessApp(nil, rd, preview, repo)

	detail, err := app.GetSkill(ctx, 1, "1", "p1", "s1")
	require.NoError(t, err)
	require.Len(t, detail.Item.Files, 1)
	require.Equal(t, "f1", detail.Item.Files[0].FileId)
}

func TestBusinessGetFileContentRequiresParentAndAllowlist(t *testing.T) {
	setupReadConnDB(t)
	repo := newTestReadConnRepo()
	ctx := context.Background()
	require.NoError(t, repo.Insert(ctx, &entity.OctopReadConnection{
		InstanceId: 1, Type: entity.ReadConnTypeOctopAPI,
		BaseURL: "https://octop.example", SecretEnc: encSecret(t, "tok"), CredentialConfigured: 1, Revision: 1,
	}))

	rd := &fakeReader{
		pkg:   &entity.BizSkillPackage{PackageId: "p1"},
		skill: &entity.BizSkill{Slug: "s1"},
	}
	preview := &fakePreview{
		files:   []entity.BizFileEntry{{FileId: "f1", Name: "a.md"}},
		content: &entity.BizFileContent{FileId: "f1", Content: "hello"},
	}
	app := newBusinessApp(nil, rd, preview, repo)

	_, err := app.GetFileContent(ctx, &dto.FileContentQuery{
		InstanceId: 1, UserId: "1", Kind: dto.FileKindSkill, PackageId: "p1", Slug: "s1", FileId: "f1",
	}, false)
	require.Error(t, err)

	_, err = app.GetFileContent(ctx, &dto.FileContentQuery{
		InstanceId: 1, UserId: "1", Kind: dto.FileKindSkill, PackageId: "p1", Slug: "s1", FileId: "other",
	}, true)
	require.Error(t, err)

	content, err := app.GetFileContent(ctx, &dto.FileContentQuery{
		InstanceId: 1, UserId: "1", Kind: dto.FileKindSkill, PackageId: "p1", Slug: "s1", FileId: "f1",
	}, true)
	require.NoError(t, err)
	require.Equal(t, "hello", content.Content)
}

func TestBusinessGetFileContentPreviewNotConfigured(t *testing.T) {
	setupReadConnDB(t)
	rd := &fakeReader{
		pkg:   &entity.BizSkillPackage{PackageId: "p1"},
		skill: &entity.BizSkill{Slug: "s1"},
	}
	app := newBusinessApp(nil, rd, &fakePreview{}, newTestReadConnRepo())
	_, err := app.GetFileContent(context.Background(), &dto.FileContentQuery{
		InstanceId: 1, UserId: "1", Kind: dto.FileKindSkill, PackageId: "p1", Slug: "s1", FileId: "f1",
	}, true)
	require.Error(t, err)
	var be *errorx.BizError
	require.ErrorAs(t, err, &be)
	require.Contains(t, be.Error(), "预览 API")
}

func TestMapRequestErr(t *testing.T) {
	ctx := context.Background()
	require.Error(t, mapRequestErr(ctx, businessread.ErrNotFound))
	require.Error(t, mapRequestErr(ctx, businessread.ErrInvalidUserId))
	require.Error(t, mapRequestErr(ctx, businessread.ErrInvalidQuery))
	require.Nil(t, mapRequestErr(ctx, errors.New("other")))
}
