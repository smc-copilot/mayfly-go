package api

import (
	"github.com/spf13/cast"
	"mayfly-go/internal/casdoor/application"
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/model"
	"mayfly-go/pkg/req"
)

type ServiceAPI struct{ Manager *application.ServiceManager }

func (a *ServiceAPI) ReqConfs() *req.Confs {
	base := "/services/:serviceId/organizations/:organizationId"
	return req.NewConfs("/casdoor",
		req.NewGet("/service-capabilities", func(rc *req.Ctx) {
			rc.ResData = map[string]bool{"canConfigure": rc.GetLoginAccount().Id == consts.AdminId}
		}).RequiredPermissionCode("casdoor:read"),
		req.NewGet("/services", a.List).RequiredPermissionCode("casdoor:read"),
		req.NewPost("/services", a.Save).RequiredPermissionCode("casdoor:connection:manage"),
		req.NewPost("/services/test", a.Test).RequiredPermissionCode("casdoor:connection:manage"),
		req.NewPost("/services/:serviceId/discover", a.Discover).RequiredPermissionCode("casdoor:connection:manage"),
		req.NewGet("/services/:serviceId/organizations", a.Organizations).RequiredPermissionCode("casdoor:read"),
		req.NewPost(base+"/sync", a.Sync).RequiredPermissionCode("casdoor:sync"),
		req.NewGet(base+"/applications", a.Applications).RequiredPermissionCode("casdoor:read"),
		req.NewGet(base+"/users", a.Users).RequiredPermissionCode("casdoor:read"),
		req.NewGet(base+"/objects", a.Objects).RequiredPermissionCode("casdoor:read"),
		req.NewGet(base+"/operations", a.Operations).RequiredPermissionCode("casdoor:audit:read"),
	)
}
func serviceIDs(rc *req.Ctx) (uint64, uint64) {
	return cast.ToUint64(rc.PathParam("serviceId")), cast.ToUint64(rc.PathParam("organizationId"))
}
func (a *ServiceAPI) List(rc *req.Ctx) {
	rows, err := a.Manager.List(rc.MetaCtx)
	biz.ErrIsNil(err)
	rc.ResData = rows
}
func (a *ServiceAPI) Save(rc *req.Ctx) {
	f := rc.BindJson[application.ServiceInput]()
	id, err := a.Manager.Save(rc.MetaCtx, *f)
	biz.ErrIsNil(err)
	rc.ResData = id
}
func (a *ServiceAPI) Test(rc *req.Ctx) {
	f := rc.BindJson[application.ServiceInput]()
	rows, err := a.Manager.Test(rc.MetaCtx, *f)
	biz.ErrIsNil(err)
	rc.ResData = rows
}
func (a *ServiceAPI) Discover(rc *req.Ctx) {
	id, _ := serviceIDs(rc)
	rows, err := a.Manager.Discover(rc.MetaCtx, id)
	biz.ErrIsNil(err)
	rc.ResData = rows
}
func (a *ServiceAPI) Organizations(rc *req.Ctx) {
	id, _ := serviceIDs(rc)
	rows, err := a.Manager.Organizations(rc.MetaCtx, id)
	biz.ErrIsNil(err)
	rc.ResData = rows
}
func (a *ServiceAPI) Sync(rc *req.Ctx) {
	service, org := serviceIDs(rc)
	result, err := a.Manager.Sync(rc.MetaCtx, service, org)
	biz.ErrIsNil(err)
	response := map[string]syncCategory{"applications": category(result.Apps), "users": category(result.Users)}
	for kind, r := range result.Objects {
		response[kind] = category(r)
	}
	rc.ResData = response
}
func (a *ServiceAPI) Applications(rc *req.Ctx) {
	service, org := serviceIDs(rc)
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Manager.Applications(rc.MetaCtx, service, org, q.Keyword, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[any]{List: toAny(rows), Total: total, PageNum: page, PageSize: size}
}
func (a *ServiceAPI) Users(rc *req.Ctx) {
	service, org := serviceIDs(rc)
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Manager.Users(rc.MetaCtx, service, org, q.Keyword, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[any]{List: toAny(rows), Total: total, PageNum: page, PageSize: size}
}
func (a *ServiceAPI) Objects(rc *req.Ctx) {
	service, org := serviceIDs(rc)
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Manager.Objects(rc.MetaCtx, service, org, q.Kind, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[any]{List: toAny(rows), Total: total, PageNum: page, PageSize: size}
}
func (a *ServiceAPI) Operations(rc *req.Ctx) {
	service, org := serviceIDs(rc)
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Manager.Operations(rc.MetaCtx, service, org, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[any]{List: toAny(rows), Total: total, PageNum: page, PageSize: size}
}
func toAny[T any](rows []T) []any {
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}
