package api

import (
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/req"

	"github.com/spf13/cast"
)

// 业务读取权限码（均由迁移注册）。
const (
	permUserBase        = "octop:user:base"
	permLibraryBase     = "octop:library:base"
	permExpertRead      = "octop:expert:read"
	permSessionBase     = "octop:session:base"
	permTraceBase       = "octop:trace:base"
	permContentRead     = "octop:content:read"
	permIntegrationSave = "octop:integration:save"
)

// hasAnyPerm 当前登录账号是否拥有任一权限码。框架单路由只支持一个权限码，
// 需要“任一”语义的路由不设路由级权限码，改在处理函数入口调用 requireAnyPerm（仍要求登录）。
func hasAnyPerm(rc *req.Ctx, codes ...string) bool {
	la := rc.GetLoginAccount()
	registry := req.GetPermissionCodeRegistery()
	for _, c := range codes {
		if registry.HasCode(la.Id, c) {
			return true
		}
	}
	return false
}

func requireAnyPerm(rc *req.Ctx, codes ...string) {
	if !hasAnyPerm(rc, codes...) {
		biz.ErrIsNil(errorx.ErrPermission)
	}
}

// Business 实例业务读取：能力、用户、专家、会话。
type Business struct {
	businessApp application.Business `inject:"T"`
}

func (b *Business) ReqConfs() *req.Confs {
	const p = "/instances/:id/business"
	reqs := [...]*req.Conf{
		// 任一业务动作权限即可；返回内容按权限裁剪。
		req.NewGet(p+"/capabilities", b.Capabilities),
		// 用户选择器：资源库或用户页权限任一即可。
		req.NewGet(p+"/user-options", b.UserOptions),
		req.NewGet(p+"/users", b.Users).RequiredPermissionCode(permUserBase),
		req.NewGet(p+"/users/:userId", b.UserDetail).RequiredPermissionCode(permUserBase),
		req.NewGet(p+"/users/:userId/experts", b.Experts).RequiredPermissionCode(permExpertRead),
		req.NewGet(p+"/users/:userId/experts/:expertId", b.Expert).RequiredPermissionCode(permExpertRead),
		req.NewGet(p+"/users/:userId/threads", b.Threads).RequiredPermissionCode(permSessionBase),
		req.NewGet(p+"/users/:userId/threads/:threadId", b.Thread).RequiredPermissionCode(permSessionBase),
	}
	return req.NewConfs("/octop", reqs[:]...)
}

func instanceIdOf(rc *req.Ctx) uint64 {
	return cast.ToUint64(rc.PathParam("id"))
}

func (b *Business) Capabilities(rc *req.Ctx) {
	all := hasAnyPerm(rc, permIntegrationSave)
	scope := dto.CapabilityScope{
		BusinessDB:   all || hasAnyPerm(rc, permUserBase, permLibraryBase, permExpertRead, permSessionBase, permTraceBase),
		OctopPreview: all || hasAnyPerm(rc, permLibraryBase, permContentRead),
		Langfuse:     all || hasAnyPerm(rc, permSessionBase, permTraceBase),
	}
	if !scope.BusinessDB && !scope.OctopPreview && !scope.Langfuse {
		biz.ErrIsNil(errorx.ErrPermission)
	}
	res, err := b.businessApp.Capabilities(rc.MetaCtx, instanceIdOf(rc), scope)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (b *Business) UserOptions(rc *req.Ctx) {
	requireAnyPerm(rc, permLibraryBase, permUserBase)
	q := rc.BindQuery[entity.BusinessQuery]()
	res, err := b.businessApp.PageUserOptions(rc.MetaCtx, instanceIdOf(rc), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (b *Business) Users(rc *req.Ctx) {
	q := rc.BindQuery[entity.BusinessQuery]()
	res, err := b.businessApp.PageUsers(rc.MetaCtx, instanceIdOf(rc), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (b *Business) UserDetail(rc *req.Ctx) {
	res, err := b.businessApp.GetUserDetail(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"))
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (b *Business) Experts(rc *req.Ctx) {
	q := rc.BindQuery[entity.BusinessQuery]()
	res, err := b.businessApp.PageExperts(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (b *Business) Expert(rc *req.Ctx) {
	includeContent := cast.ToBool(rc.Query("includeContent"))
	contentAllowed := includeContent && hasAnyPerm(rc, permContentRead)
	res, err := b.businessApp.GetExpert(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("expertId"), includeContent, contentAllowed)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (b *Business) Threads(rc *req.Ctx) {
	q := rc.BindQuery[entity.BusinessQuery]()
	res, err := b.businessApp.PageThreads(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (b *Business) Thread(rc *req.Ctx) {
	res, err := b.businessApp.GetThread(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("threadId"))
	biz.ErrIsNil(err)
	rc.ResData = res
}
