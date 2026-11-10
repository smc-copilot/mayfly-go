package application

import (
	"context"
	"errors"
	"time"

	dbapp "mayfly-go/internal/db/application"
	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/business"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/businessread"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/logx"
	"mayfly-go/pkg/model"
)

const pgReadTimeout = 10 * time.Second

const (
	capCodeBusinessDB   = "business_db"
	capCodeOctopPreview = "octop_preview"
	capCodeLangfuse     = "langfuse"
)

// Business 实例业务只读读取（用户、专家、会话、模板、技能包）。
//
// 所有方法先校验实例可见性；资源一律按 userId 限定，跨用户 ID 视为不存在。
// 来源不可用（未配置/无权限/不支持/失败）时分页与详情在成功信封中以 Source.State 表达，
// 不会把失败当成“0 条”；非法参数、记录不存在、无内容权限为请求级错误。
type Business interface {
	Capabilities(ctx context.Context, instanceId uint64, scope dto.CapabilityScope) (*dto.Capabilities, error)

	PageUserOptions(ctx context.Context, instanceId uint64, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizUserOption], error)
	PageUsers(ctx context.Context, instanceId uint64, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizUser], error)
	GetUserDetail(ctx context.Context, instanceId uint64, userId string) (*dto.UserDetail, error)

	PageExperts(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizExpert], error)
	// GetExpert 返回专家配置白名单投影；includeContent 为 true 时必须 contentAllowed，否则 ErrBusinessContentDenied。
	GetExpert(ctx context.Context, instanceId uint64, userId, expertId string, includeContent, contentAllowed bool) (*dto.BusinessDetail[entity.BizExpert], error)

	PageThreads(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizThread], error)
	GetThread(ctx context.Context, instanceId uint64, userId, threadId string) (*dto.BusinessDetail[entity.BizThread], error)

	PageExpertTemplates(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizExpertTemplate], error)
	GetExpertTemplate(ctx context.Context, instanceId uint64, userId, templateId string) (*dto.BusinessDetail[entity.BizExpertTemplate], error)

	PageSkillPackages(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizSkillPackage], error)
	GetSkillPackage(ctx context.Context, instanceId uint64, userId, packageId string) (*dto.BusinessDetail[entity.BizSkillPackage], error)
	PageSkills(ctx context.Context, instanceId uint64, userId, packageId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizSkill], error)
	GetSkill(ctx context.Context, instanceId uint64, userId, packageId, slug string) (*dto.BusinessDetail[entity.BizSkill], error)

	// GetFileContent 读取技能/模板文件正文。contentAllowed 由 API 层按 octop:content:read 判定；
	// 文件只能通过父资源（技能/模板）返回的条目 ID 引用，不接受任意路径。
	GetFileContent(ctx context.Context, q *dto.FileContentQuery, contentAllowed bool) (*entity.BizFileContent, error)
}

// readerFactory 构造业务读取器，测试可替换。
type readerFactory func(ctx context.Context, inst *entity.OctopInstance) (business.Reader, error)

// promptReader 可选能力：读取专家 Prompt。当前 PG 读取器不提供，保留扩展点。
type promptReader interface {
	GetExpertPrompt(ctx context.Context, userId, expertId string) (string, error)
}

type businessAppImpl struct {
	instanceApp instanceAccessor          `inject:"T"`
	dbApp       dbapp.Db                  `inject:"T"`
	connRepo    repository.ReadConnection `inject:"T"`
	newReader   readerFactory
	newPreview  previewFactory
}

var _ Business = (*businessAppImpl)(nil)

// ---------------------------------------------------------------- plumbing

func (a *businessAppImpl) openReader(ctx context.Context, inst *entity.OctopInstance) (business.Reader, error) {
	if a.newReader != nil {
		return a.newReader(ctx, inst)
	}
	conn, err := a.dbApp.GetDbConn(ctx, inst.DbId, inst.DbName)
	if err != nil {
		return nil, err
	}
	return businessread.NewReader(conn), nil
}

func unavailableMeta(state, msg string) *entity.SourceMeta {
	now := time.Now()
	return &entity.SourceMeta{Source: businessread.SourceName, State: state, ReadAt: &now, Message: msg}
}

// acquire 校验实例可见性并打开读取器。
// 实例不可见返回 error；PG 未关联或无法连接返回 (nil, meta, nil)，由调用方按来源状态表达。
func (a *businessAppImpl) acquire(ctx context.Context, instanceId uint64) (business.Reader, *entity.SourceMeta, error) {
	inst, err := a.instanceApp.GetVisible(ctx, instanceId)
	if err != nil {
		return nil, nil, err
	}
	if inst.DbId == 0 || inst.DbName == "" {
		return nil, unavailableMeta(entity.SourceStateNotConfigured, "该实例未关联 PostgreSQL"), nil
	}
	rd, err := a.openReader(ctx, inst)
	if err != nil {
		logx.Errorf("octop business open reader instance=%d err=%v", instanceId, err)
		return nil, unavailableMeta(entity.SourceStateFailed, "无法建立 PostgreSQL 连接"), nil
	}
	return rd, nil, nil
}

// mapRequestErr 把读取器的请求级错误（非法参数/不存在）转成业务错误；其余返回 nil 表示属来源故障。
func mapRequestErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, businessread.ErrNotFound):
		return errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	case errors.Is(err, businessread.ErrInvalidUserId):
		return errorx.NewBizI(ctx, imsg.ErrBusinessUserRequired)
	case errors.Is(err, businessread.ErrInvalidQuery):
		return errorx.NewBiz(err.Error())
	}
	return nil
}

func sourceOrFailed(meta *entity.SourceMeta) *entity.SourceMeta {
	if meta != nil {
		return meta
	}
	return unavailableMeta(entity.SourceStateFailed, "业务数据读取失败")
}

func readPage[T any](ctx context.Context, a *businessAppImpl, instanceId uint64, q *entity.BusinessQuery,
	fn func(context.Context, business.Reader, *entity.BusinessQuery) (*model.PageResult[*T], *entity.SourceMeta, error),
) (*dto.BusinessPage[T], error) {
	if q == nil {
		q = &entity.BusinessQuery{}
	}
	q.Normalize()
	rd, miss, err := a.acquire(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	out := &dto.BusinessPage[T]{List: []*T{}, PageNum: q.PageNum, PageSize: q.PageSize}
	if rd == nil {
		out.Source = miss
		return out, nil
	}
	cctx, cancel := context.WithTimeout(ctx, pgReadTimeout)
	defer cancel()
	res, meta, err := fn(cctx, rd, q)
	if err != nil {
		if rerr := mapRequestErr(ctx, err); rerr != nil {
			return nil, rerr
		}
		logx.Errorf("octop business page instance=%d err=%v", instanceId, err)
		out.Source = sourceOrFailed(meta)
		return out, nil
	}
	if res == nil {
		out.Source = sourceOrFailed(meta)
		return out, nil
	}
	if res.List != nil {
		out.List = res.List
	}
	out.Total = res.Total
	if res.PageNum > 0 {
		out.PageNum = res.PageNum
	}
	if res.PageSize > 0 {
		out.PageSize = res.PageSize
	}
	out.Source = meta
	return out, nil
}

func readOne[T any](ctx context.Context, a *businessAppImpl, instanceId uint64,
	fn func(context.Context, business.Reader) (*T, *entity.SourceMeta, error),
) (*dto.BusinessDetail[T], error) {
	rd, miss, err := a.acquire(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	if rd == nil {
		return &dto.BusinessDetail[T]{Source: miss}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, pgReadTimeout)
	defer cancel()
	item, meta, err := fn(cctx, rd)
	if err != nil {
		if rerr := mapRequestErr(ctx, err); rerr != nil {
			return nil, rerr
		}
		logx.Errorf("octop business detail instance=%d err=%v", instanceId, err)
		return &dto.BusinessDetail[T]{Source: sourceOrFailed(meta)}, nil
	}
	if item == nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	return &dto.BusinessDetail[T]{Item: item, Source: meta}, nil
}

func statOrFailed(s entity.StatValue, err error) entity.StatValue {
	if s.Value == nil && s.State != "" && s.State != entity.SourceStateReady {
		return s // 读取器已给出分类状态（unsupported/forbidden/failed）
	}
	if err != nil || (s.Value == nil && s.State == "") {
		return entity.NewStatNull(entity.SourceStateFailed, nil)
	}
	return s
}

// ---------------------------------------------------------------- capabilities

func (a *businessAppImpl) Capabilities(ctx context.Context, instanceId uint64, scope dto.CapabilityScope) (*dto.Capabilities, error) {
	inst, err := a.instanceApp.GetVisible(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	caps := &dto.Capabilities{InstanceId: inst.Id}
	if scope.BusinessDB {
		caps.BusinessDB = a.dbCapability(ctx, inst)
	}
	if scope.OctopPreview {
		item, err := a.connCapability(ctx, inst.Id, entity.ReadConnTypeOctopAPI, capCodeOctopPreview)
		if err != nil {
			return nil, err
		}
		caps.OctopPreview = item
	}
	if scope.Langfuse {
		item, err := a.connCapability(ctx, inst.Id, entity.ReadConnTypeLangfuse, capCodeLangfuse)
		if err != nil {
			return nil, err
		}
		caps.Langfuse = item
	}
	return caps, nil
}

// dbCapability 以一次最小用户选项查询探测业务库可读性。
func (a *businessAppImpl) dbCapability(ctx context.Context, inst *entity.OctopInstance) *dto.CapabilityItem {
	item := &dto.CapabilityItem{Code: capCodeBusinessDB}
	if inst.DbId == 0 || inst.DbName == "" {
		item.State = entity.SourceStateNotConfigured
		item.Message = "该实例未关联 PostgreSQL"
		return item
	}
	rd, err := a.openReader(ctx, inst)
	if err != nil {
		logx.Errorf("octop business capability open reader instance=%d err=%v", inst.Id, err)
		item.State = entity.SourceStateFailed
		item.Message = "无法建立 PostgreSQL 连接"
		return item
	}
	cctx, cancel := context.WithTimeout(ctx, pgReadTimeout)
	defer cancel()
	q := &entity.BusinessQuery{}
	q.PageNum, q.PageSize = 1, 1
	_, meta, err := rd.PageUserOptions(cctx, q)
	if err != nil {
		logx.Errorf("octop business capability probe instance=%d err=%v", inst.Id, err)
	}
	meta = sourceOrFailed(meta)
	item.State = meta.State
	item.Message = meta.Message
	if meta.State == entity.SourceStateReady {
		item.VerifiedAt = meta.ReadAt
	}
	return item
}

func (a *businessAppImpl) connCapability(ctx context.Context, instanceId uint64, connType, code string) (*dto.CapabilityItem, error) {
	item := &dto.CapabilityItem{Code: code, State: entity.SourceStateNotConfigured}
	row, err := a.connRepo.GetByInstanceType(ctx, instanceId, connType)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return item, nil
	}
	item.State = row.VerifyState
	if item.State == "" {
		item.State = entity.SourceStateNotVerified
	}
	item.Message = row.VerifyError
	item.VerifiedAt = row.VerifiedAt
	return item, nil
}

// ---------------------------------------------------------------- users

func (a *businessAppImpl) PageUserOptions(ctx context.Context, instanceId uint64, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizUserOption], error) {
	return readPage(ctx, a, instanceId, q, func(c context.Context, rd business.Reader, q *entity.BusinessQuery) (*model.PageResult[*entity.BizUserOption], *entity.SourceMeta, error) {
		return rd.PageUserOptions(c, q)
	})
}

func (a *businessAppImpl) PageUsers(ctx context.Context, instanceId uint64, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizUser], error) {
	return readPage(ctx, a, instanceId, q, func(c context.Context, rd business.Reader, q *entity.BusinessQuery) (*model.PageResult[*entity.BizUser], *entity.SourceMeta, error) {
		return rd.PageUsers(c, q)
	})
}

func (a *businessAppImpl) GetUserDetail(ctx context.Context, instanceId uint64, userId string) (*dto.UserDetail, error) {
	rd, miss, err := a.acquire(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	if rd == nil {
		return unavailableUserDetail(miss), nil
	}
	cctx, cancel := context.WithTimeout(ctx, pgReadTimeout)
	defer cancel()
	user, meta, err := rd.GetUser(cctx, userId)
	if err != nil {
		if rerr := mapRequestErr(ctx, err); rerr != nil {
			return nil, rerr
		}
		logx.Errorf("octop business user detail instance=%d err=%v", instanceId, err)
		return unavailableUserDetail(sourceOrFailed(meta)), nil
	}
	if user == nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	d := &dto.UserDetail{User: user, Source: meta}
	d.ExpertCount = statOrFailed(rd.CountExperts(cctx, userId))
	d.ThreadCount = statOrFailed(rd.CountThreads(cctx, userId))
	d.TemplateCount = statOrFailed(rd.CountTemplates(cctx, userId))
	d.SkillPackageCount = statOrFailed(rd.CountSkillPackages(cctx, userId))
	return d, nil
}

func unavailableUserDetail(meta *entity.SourceMeta) *dto.UserDetail {
	null := func() entity.StatValue { return entity.NewStatNull(meta.State, nil) }
	return &dto.UserDetail{
		ExpertCount: null(), ThreadCount: null(), TemplateCount: null(), SkillPackageCount: null(),
		Source: meta,
	}
}

// ---------------------------------------------------------------- experts / threads

func (a *businessAppImpl) PageExperts(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizExpert], error) {
	return readPage(ctx, a, instanceId, q, func(c context.Context, rd business.Reader, q *entity.BusinessQuery) (*model.PageResult[*entity.BizExpert], *entity.SourceMeta, error) {
		return rd.PageExperts(c, userId, q)
	})
}

func (a *businessAppImpl) GetExpert(ctx context.Context, instanceId uint64, userId, expertId string, includeContent, contentAllowed bool) (*dto.BusinessDetail[entity.BizExpert], error) {
	if includeContent && !contentAllowed {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessContentDenied)
	}
	return readOne(ctx, a, instanceId, func(c context.Context, rd business.Reader) (*entity.BizExpert, *entity.SourceMeta, error) {
		e, meta, err := rd.GetExpert(c, userId, expertId)
		if err != nil || e == nil {
			return e, meta, err
		}
		if e.Config == nil {
			return e, meta, nil
		}
		// Prompt 默认不返回：无论读取器是否带出，未显式授权展开都清空。
		e.Config.PromptPreview = ""
		if includeContent {
			if pr, ok := rd.(promptReader); ok {
				prompt, perr := pr.GetExpertPrompt(c, userId, expertId)
				if perr != nil {
					logx.Errorf("octop business expert prompt instance=%d err=%v", instanceId, perr)
					e.Config.Unsupported = append(e.Config.Unsupported, "prompt")
				} else {
					e.Config.PromptPreview = prompt
				}
			} else {
				e.Config.Unsupported = append(e.Config.Unsupported, "prompt")
			}
		}
		return e, meta, nil
	})
}

func (a *businessAppImpl) PageThreads(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizThread], error) {
	return readPage(ctx, a, instanceId, q, func(c context.Context, rd business.Reader, q *entity.BusinessQuery) (*model.PageResult[*entity.BizThread], *entity.SourceMeta, error) {
		return rd.PageThreads(c, userId, q)
	})
}

func (a *businessAppImpl) GetThread(ctx context.Context, instanceId uint64, userId, threadId string) (*dto.BusinessDetail[entity.BizThread], error) {
	return readOne(ctx, a, instanceId, func(c context.Context, rd business.Reader) (*entity.BizThread, *entity.SourceMeta, error) {
		return rd.GetThread(c, userId, threadId)
	})
}

// ---------------------------------------------------------------- library

func (a *businessAppImpl) PageExpertTemplates(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizExpertTemplate], error) {
	return readPage(ctx, a, instanceId, q, func(c context.Context, rd business.Reader, q *entity.BusinessQuery) (*model.PageResult[*entity.BizExpertTemplate], *entity.SourceMeta, error) {
		return rd.PageExpertTemplates(c, userId, q)
	})
}

func (a *businessAppImpl) GetExpertTemplate(ctx context.Context, instanceId uint64, userId, templateId string) (*dto.BusinessDetail[entity.BizExpertTemplate], error) {
	d, err := readOne(ctx, a, instanceId, func(c context.Context, rd business.Reader) (*entity.BizExpertTemplate, *entity.SourceMeta, error) {
		return rd.GetExpertTemplate(c, userId, templateId)
	})
	if err != nil || d.Item == nil {
		return d, err
	}
	files, ferr := a.listFiles(ctx, instanceId, func(c context.Context, pc business.PreviewClient) ([]entity.BizFileEntry, error) {
		return pc.ListTemplateFiles(c, templateId)
	})
	if ferr != nil {
		d.Source = degradeSource(d.Source, ferr)
	} else {
		d.Item.Files = files
	}
	return d, nil
}

func (a *businessAppImpl) PageSkillPackages(ctx context.Context, instanceId uint64, userId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizSkillPackage], error) {
	return readPage(ctx, a, instanceId, q, func(c context.Context, rd business.Reader, q *entity.BusinessQuery) (*model.PageResult[*entity.BizSkillPackage], *entity.SourceMeta, error) {
		return rd.PageSkillPackages(c, userId, q)
	})
}

func (a *businessAppImpl) GetSkillPackage(ctx context.Context, instanceId uint64, userId, packageId string) (*dto.BusinessDetail[entity.BizSkillPackage], error) {
	return readOne(ctx, a, instanceId, func(c context.Context, rd business.Reader) (*entity.BizSkillPackage, *entity.SourceMeta, error) {
		return rd.GetSkillPackage(c, userId, packageId)
	})
}

func (a *businessAppImpl) PageSkills(ctx context.Context, instanceId uint64, userId, packageId string, q *entity.BusinessQuery) (*dto.BusinessPage[entity.BizSkill], error) {
	return readPage(ctx, a, instanceId, q, func(c context.Context, rd business.Reader, q *entity.BusinessQuery) (*model.PageResult[*entity.BizSkill], *entity.SourceMeta, error) {
		// 先确认包属于该用户，避免用他人包 ID 枚举技能。
		if _, meta, err := rd.GetSkillPackage(c, userId, packageId); err != nil {
			return nil, meta, err
		}
		return rd.PageSkills(c, userId, packageId, q)
	})
}

func (a *businessAppImpl) GetSkill(ctx context.Context, instanceId uint64, userId, packageId, slug string) (*dto.BusinessDetail[entity.BizSkill], error) {
	d, err := readOne(ctx, a, instanceId, func(c context.Context, rd business.Reader) (*entity.BizSkill, *entity.SourceMeta, error) {
		if _, meta, err := rd.GetSkillPackage(c, userId, packageId); err != nil {
			return nil, meta, err
		}
		return rd.GetSkill(c, userId, packageId, slug)
	})
	if err != nil || d.Item == nil {
		return d, err
	}
	files, ferr := a.listFiles(ctx, instanceId, func(c context.Context, pc business.PreviewClient) ([]entity.BizFileEntry, error) {
		return pc.ListSkillFiles(c, packageId, slug)
	})
	if ferr != nil {
		d.Source = degradeSource(d.Source, ferr)
	} else {
		d.Item.Files = files
	}
	return d, nil
}

// ---------------------------------------------------------------- preview / files

var errPreviewUnavailable = errors.New("octop preview api not configured")

func (a *businessAppImpl) previewFactory() previewFactory {
	if a.newPreview != nil {
		return a.newPreview
	}
	return defaultPreviewFactory
}

// listFiles 经预览 API 列出文件目录；未配置返回 errPreviewUnavailable。
func (a *businessAppImpl) listFiles(ctx context.Context, instanceId uint64, fn func(context.Context, business.PreviewClient) ([]entity.BizFileEntry, error)) ([]entity.BizFileEntry, error) {
	pc, _, err := loadPreviewClient(ctx, a.connRepo, a.previewFactory(), instanceId)
	if err != nil {
		return nil, err
	}
	if pc == nil {
		return nil, errPreviewUnavailable
	}
	cctx, cancel := context.WithTimeout(ctx, previewTimeout)
	defer cancel()
	return fn(cctx, pc)
}

// degradeSource 文件目录不可用时把来源降级为 partial 并说明原因（元数据本身仍有效）。
func degradeSource(src *entity.SourceMeta, ferr error) *entity.SourceMeta {
	m := entity.SourceMeta{}
	if src != nil {
		m = *src
	}
	m.State = entity.SourceStatePartial
	if errors.Is(ferr, errPreviewUnavailable) {
		m.Message = "Octop 预览 API 未配置，文件目录不可用"
	} else {
		logx.Errorf("octop preview list files err=%v", ferr)
		_, msg := classifyPreviewErr(ferr)
		m.Message = "文件目录读取失败：" + msg
	}
	return &m
}

func (a *businessAppImpl) GetFileContent(ctx context.Context, q *dto.FileContentQuery, contentAllowed bool) (*entity.BizFileContent, error) {
	if !contentAllowed {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessContentDenied)
	}
	if q == nil || q.FileId == "" || (q.Kind != dto.FileKindSkill && q.Kind != dto.FileKindTemplate) {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	rd, miss, err := a.acquire(ctx, q.InstanceId)
	if err != nil {
		return nil, err
	}
	if rd == nil {
		if miss != nil && miss.State == entity.SourceStateNotConfigured {
			return nil, errorx.NewBizI(ctx, imsg.ErrBusinessDbNotLinked)
		}
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}

	// 先经 PG 校验父资源属于该用户，再从服务端目录里确认 fileId，最后才读正文。
	pctx, pcancel := context.WithTimeout(ctx, pgReadTimeout)
	defer pcancel()
	if err := a.verifyFileParent(pctx, rd, q); err != nil {
		if rerr := mapRequestErr(ctx, err); rerr != nil {
			return nil, rerr
		}
		logx.Errorf("octop business file parent instance=%d err=%v", q.InstanceId, err)
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}

	pc, _, err := loadPreviewClient(ctx, a.connRepo, a.previewFactory(), q.InstanceId)
	if err != nil {
		logx.Errorf("octop business load preview instance=%d err=%v", q.InstanceId, err)
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}
	if pc == nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrPreviewNotConfigured)
	}
	fctx, fcancel := context.WithTimeout(ctx, previewTimeout)
	defer fcancel()

	var entries []entity.BizFileEntry
	if q.Kind == dto.FileKindSkill {
		entries, err = pc.ListSkillFiles(fctx, q.PackageId, q.Slug)
	} else {
		entries, err = pc.ListTemplateFiles(fctx, q.TemplateId)
	}
	if err != nil {
		logx.Errorf("octop business list files instance=%d err=%v", q.InstanceId, err)
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}
	found := false
	for _, e := range entries {
		if e.FileId == q.FileId {
			found = true
			break
		}
	}
	if !found {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	content, err := pc.GetFileContent(fctx, q.FileId)
	if err != nil {
		logx.Errorf("octop business file content instance=%d err=%v", q.InstanceId, err)
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}
	if content == nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	return content, nil
}

func (a *businessAppImpl) verifyFileParent(ctx context.Context, rd business.Reader, q *dto.FileContentQuery) error {
	if q.Kind == dto.FileKindTemplate {
		_, _, err := rd.GetExpertTemplate(ctx, q.UserId, q.TemplateId)
		return err
	}
	if _, _, err := rd.GetSkillPackage(ctx, q.UserId, q.PackageId); err != nil {
		return err
	}
	_, _, err := rd.GetSkill(ctx, q.UserId, q.PackageId, q.Slug)
	return err
}
