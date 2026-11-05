package api

import (
	"mayfly-go/internal/octop/api/form"
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/model"
	"mayfly-go/pkg/req"
	"mayfly-go/pkg/utils/collx"

	"github.com/spf13/cast"
)

type Casdoor struct {
	casdoorApp application.Casdoor `inject:"T"`
}

func (c *Casdoor) ReqConfs() *req.Confs {
	reqs := [...]*req.Conf{
		req.NewGet("/casdoor/connections", c.List).RequiredPermissionCode("octop:casdoor:base"),
		req.NewPost("/casdoor/connections/test", c.Test).RequiredPermissionCode("octop:casdoor:save"),
		req.NewPost("/casdoor/connections", c.Save).Log(req.NewLogSaveI(imsg.LogOctopCasdoorSave)).RequiredPermissionCode("octop:casdoor:save"),
		req.NewPost("/casdoor/connections/:id/sync", c.Sync).Log(req.NewLogSaveI(imsg.LogOctopCasdoorSync)).RequiredPermissionCode("octop:casdoor:save"),
		req.NewGet("/casdoor/apps", c.Apps).RequiredPermissionCode("octop:casdoor:base"),
		req.NewGet("/casdoor/users", c.Users).RequiredPermissionCode("octop:casdoor:base"),
		req.NewGet("/casdoor/auths", c.Auths).RequiredPermissionCode("octop:casdoor:base"),
		req.NewGet("/casdoor/identity-links", c.Links).RequiredPermissionCode("octop:casdoor:base"),
		req.NewGet("/casdoor/identity-candidates", c.Candidates).RequiredPermissionCode("octop:casdoor:base"),
		req.NewPost("/casdoor/identity-links", c.SaveLink).Log(req.NewLogSaveI(imsg.LogOctopIdentityLink)).RequiredPermissionCode("octop:casdoor:save"),
		req.NewDelete("/casdoor/identity-links/:id", c.DeleteLink).Log(req.NewLogSaveI(imsg.LogOctopIdentityLink)).RequiredPermissionCode("octop:casdoor:save"),
	}
	return req.NewConfs("/octop", reqs[:]...)
}

func (c *Casdoor) List(rc *req.Ctx) {
	list, err := c.casdoorApp.ListByCond(model.NewCond())
	biz.ErrIsNil(err)
	rc.ResData = list
}

func (c *Casdoor) Test(rc *req.Ctx) {
	f := rc.BindJson[form.CasdoorForm]()
	res, err := c.casdoorApp.TestConnection(rc.MetaCtx, f.ToEntity(), f.ClientSecret)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (c *Casdoor) Save(rc *req.Ctx) {
	f := rc.BindJson[form.CasdoorForm]()
	conn := f.ToEntity()
	conn.Id = f.Id
	rc.ReqParam = collx.Kvs("name", conn.Name, "organization", conn.Organization)
	biz.ErrIsNil(c.casdoorApp.SaveConnection(rc.MetaCtx, conn, f.ClientSecret))
	rc.ResData = conn.Id
}

func (c *Casdoor) Sync(rc *req.Ctx) {
	id := cast.ToUint64(rc.PathParam("id"))
	rc.ReqParam = id
	res, err := c.casdoorApp.Sync(rc.MetaCtx, id)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (c *Casdoor) Apps(rc *req.Ctx) {
	q := rc.BindQuery[entity.CasdoorAppQuery]()
	res, err := c.casdoorApp.PageApps(rc.MetaCtx, q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (c *Casdoor) Users(rc *req.Ctx) {
	q := rc.BindQuery[entity.CasdoorUserQuery]()
	res, err := c.casdoorApp.PageUsers(rc.MetaCtx, q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (c *Casdoor) Auths(rc *req.Ctx) {
	connectionId := cast.ToUint64(rc.Query("connectionId"))
	appSourceId := rc.Query("applicationSourceId")
	res, err := c.casdoorApp.ListAuths(rc.MetaCtx, connectionId, appSourceId)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (c *Casdoor) Links(rc *req.Ctx) {
	instanceId := cast.ToUint64(rc.Query("instanceId"))
	res, err := c.casdoorApp.ListIdentityLinks(rc.MetaCtx, instanceId)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (c *Casdoor) Candidates(rc *req.Ctx) {
	q := rc.BindQuery[entity.IdentityCandidateQuery]()
	res, err := c.casdoorApp.MatchCandidates(rc.MetaCtx, q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (c *Casdoor) SaveLink(rc *req.Ctx) {
	f := rc.BindJson[form.IdentityLinkForm]()
	rc.ReqParam = collx.Kvs("casdoorUserSourceId", f.CasdoorUserSourceId, "octopInstanceId", f.OctopInstanceId, "octopUserId", f.OctopUserId)
	biz.ErrIsNil(c.casdoorApp.SaveIdentityLink(rc.MetaCtx, &entity.OctopIdentityLink{
		ConnectionId: f.ConnectionId, CasdoorUserSourceId: f.CasdoorUserSourceId,
		OctopInstanceId: f.OctopInstanceId, OctopUserId: f.OctopUserId, Evidence: f.Evidence,
	}))
}

func (c *Casdoor) DeleteLink(rc *req.Ctx) {
	id := cast.ToUint64(rc.PathParam("id"))
	rc.ReqParam = id
	biz.ErrIsNil(c.casdoorApp.DeleteIdentityLink(rc.MetaCtx, id))
}
