package api

import (
	"mayfly-go/internal/octop/api/form"
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/req"
	"mayfly-go/pkg/utils/collx"
)

// ReadConnection 实例业务读取连接配置（Octop 预览 API / Langfuse）。
type ReadConnection struct {
	readConnApp application.ReadConnectionApp `inject:"T"`
}

func (r *ReadConnection) ReqConfs() *req.Confs {
	reqs := [...]*req.Conf{
		req.NewGet("/instances/:id/read-connection", r.Get).RequiredPermissionCode(permIntegrationSave),
		req.NewPost("/instances/:id/read-connection", r.Save).Log(req.NewLogSaveI(imsg.LogOctopReadConnectionSave)).RequiredPermissionCode(permIntegrationSave),
		req.NewPost("/instances/:id/read-connection/test", r.Test).RequiredPermissionCode(permIntegrationSave),
	}
	return req.NewConfs("/octop", reqs[:]...)
}

// Get 带 type 查询参数返回单个配置，否则返回全部类型。
func (r *ReadConnection) Get(rc *req.Ctx) {
	id := instanceIdOf(rc)
	if t := rc.Query("type"); t != "" {
		res, err := r.readConnApp.Get(rc.MetaCtx, id, t)
		biz.ErrIsNil(err)
		rc.ResData = res
		return
	}
	res, err := r.readConnApp.List(rc.MetaCtx, id)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (r *ReadConnection) Save(rc *req.Ctx) {
	f := rc.BindJson[form.ReadConnectionForm]()
	id := instanceIdOf(rc)
	// 日志参数不含 secret。
	rc.ReqParam = collx.Kvs("instanceId", id, "type", f.Type, "baseUrl", f.BaseURL, "clearSecret", f.ClearSecret)
	res, err := r.readConnApp.Save(rc.MetaCtx, id, &dto.ReadConnectionSave{
		Type: f.Type, BaseURL: f.BaseURL, ProjectId: f.ProjectId, PublicKey: f.PublicKey,
		Secret: f.Secret, ClearSecret: f.ClearSecret, Revision: f.Revision,
	})
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (r *ReadConnection) Test(rc *req.Ctx) {
	f := rc.BindJson[form.ReadConnectionForm]()
	res, err := r.readConnApp.Test(rc.MetaCtx, instanceIdOf(rc), &dto.ReadConnectionTest{
		Type: f.Type, BaseURL: f.BaseURL, ProjectId: f.ProjectId, PublicKey: f.PublicKey,
		Secret: f.Secret, Revision: f.Revision,
	})
	biz.ErrIsNil(err)
	rc.ResData = res
}
