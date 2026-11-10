package api

import (
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/req"
)

// Library 实例资源库：专家模板、技能包、技能及文件正文。
type Library struct {
	businessApp application.Business `inject:"T"`
}

func (l *Library) ReqConfs() *req.Confs {
	const p = "/instances/:id/business/users/:userId"
	reqs := [...]*req.Conf{
		req.NewGet(p+"/expert-templates", l.Templates).RequiredPermissionCode(permLibraryBase),
		req.NewGet(p+"/expert-templates/:templateId", l.Template).RequiredPermissionCode(permLibraryBase),
		req.NewGet(p+"/skill-packages", l.Packages).RequiredPermissionCode(permLibraryBase),
		req.NewGet(p+"/skill-packages/:packageId", l.Package).RequiredPermissionCode(permLibraryBase),
		req.NewGet(p+"/skill-packages/:packageId/skills", l.Skills).RequiredPermissionCode(permLibraryBase),
		req.NewGet(p+"/skill-packages/:packageId/skills/:slug", l.Skill).RequiredPermissionCode(permLibraryBase),
		// 正文：路由要求内容权限，处理函数再要求资源库权限；应用层按父资源归属与文件目录校验 fileId。
		req.NewGet(p+"/expert-templates/:templateId/files/:fileId/content", l.TemplateFile).RequiredPermissionCode(permContentRead),
		req.NewGet(p+"/skill-packages/:packageId/skills/:slug/files/:fileId/content", l.SkillFile).RequiredPermissionCode(permContentRead),
	}
	return req.NewConfs("/octop", reqs[:]...)
}

func (l *Library) Templates(rc *req.Ctx) {
	q := rc.BindQuery[entity.BusinessQuery]()
	res, err := l.businessApp.PageExpertTemplates(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (l *Library) Template(rc *req.Ctx) {
	res, err := l.businessApp.GetExpertTemplate(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("templateId"))
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (l *Library) Packages(rc *req.Ctx) {
	q := rc.BindQuery[entity.BusinessQuery]()
	res, err := l.businessApp.PageSkillPackages(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (l *Library) Package(rc *req.Ctx) {
	res, err := l.businessApp.GetSkillPackage(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("packageId"))
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (l *Library) Skills(rc *req.Ctx) {
	q := rc.BindQuery[entity.BusinessQuery]()
	res, err := l.businessApp.PageSkills(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("packageId"), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (l *Library) Skill(rc *req.Ctx) {
	res, err := l.businessApp.GetSkill(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("packageId"), rc.PathParam("slug"))
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (l *Library) TemplateFile(rc *req.Ctx) {
	requireAnyPerm(rc, permLibraryBase)
	res, err := l.businessApp.GetFileContent(rc.MetaCtx, &dto.FileContentQuery{
		InstanceId: instanceIdOf(rc), UserId: rc.PathParam("userId"), Kind: dto.FileKindTemplate,
		TemplateId: rc.PathParam("templateId"), FileId: rc.PathParam("fileId"),
	}, hasAnyPerm(rc, permContentRead))
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (l *Library) SkillFile(rc *req.Ctx) {
	requireAnyPerm(rc, permLibraryBase)
	res, err := l.businessApp.GetFileContent(rc.MetaCtx, &dto.FileContentQuery{
		InstanceId: instanceIdOf(rc), UserId: rc.PathParam("userId"), Kind: dto.FileKindSkill,
		PackageId: rc.PathParam("packageId"), Slug: rc.PathParam("slug"), FileId: rc.PathParam("fileId"),
	}, hasAnyPerm(rc, permContentRead))
	biz.ErrIsNil(err)
	rc.ResData = res
}
