package api

import (
	"time"

	"mayfly-go/internal/octop/api/form"
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/req"
	"mayfly-go/pkg/utils/collx"

	"github.com/spf13/cast"
)

type Instance struct {
	instanceApp application.Instance `inject:"T"`
	settingsApp application.Settings `inject:"T"`
}

func (i *Instance) ReqConfs() *req.Confs {
	reqs := [...]*req.Conf{
		req.NewGet("/overview", i.Overview).RequiredPermissionCode("octop:overview:base"),
		req.NewGet("/instances", i.List).RequiredPermissionCode("octop:instance:base"),
		req.NewGet("/instances/:id", i.Get).RequiredPermissionCode("octop:instance:base"),
		req.NewGet("/instances/:id/metrics", i.Metrics).RequiredPermissionCode("octop:instance:base"),
		req.NewPost("/instances/test-conn", i.TestConn).RequiredPermissionCode("octop:instance:save"),
		req.NewPost("/instances", i.Save).Log(req.NewLogSaveI(imsg.LogOctopInstanceSave)).RequiredPermissionCode("octop:instance:save"),
		req.NewDelete("/instances/:id", i.Delete).Log(req.NewLogSaveI(imsg.LogOctopInstanceDelete)).RequiredPermissionCode("octop:instance:del"),
		req.NewPost("/instances/:id/restart", i.Restart).Log(req.NewLogSaveI(imsg.LogOctopInstanceRestart)).RequiredPermissionCode("octop:instance:restart"),
		req.NewPost("/instances/bind-app", i.BindApp).Log(req.NewLogSaveI(imsg.LogOctopAppBind)).RequiredPermissionCode("octop:casdoor:save"),
		req.NewGet("/instances/:id/users", i.Users).RequiredPermissionCode("octop:users:base"),
		req.NewGet("/settings", i.GetSettings).RequiredPermissionCode("octop:overview:base"),
		req.NewPost("/settings", i.SaveSettings).Log(req.NewLogSaveI(imsg.LogOctopSettingsSave)).RequiredPermissionCode("octop:config:save"),
	}
	return req.NewConfs("/octop", reqs[:]...)
}

func (i *Instance) Overview(rc *req.Ctx) {
	list, err := i.instanceApp.Overview(rc.MetaCtx)
	biz.ErrIsNil(err)
	rc.ResData = list
}

func (i *Instance) List(rc *req.Ctx) {
	q := rc.BindQuery[entity.InstanceQuery]()
	res, err := i.instanceApp.PageVisible(rc.MetaCtx, q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (i *Instance) Get(rc *req.Ctx) {
	id := cast.ToUint64(rc.PathParam("id"))
	res, err := i.instanceApp.Detail(rc.MetaCtx, id)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (i *Instance) Metrics(rc *req.Ctx) {
	id := cast.ToUint64(rc.PathParam("id"))
	start := cast.ToInt64(rc.Query("start"))
	end := cast.ToInt64(rc.Query("end"))
	now := time.Now()
	endT := now
	startT := now.Add(-24 * time.Hour)
	if start > 0 {
		startT = time.UnixMilli(start)
	}
	if end > 0 {
		endT = time.UnixMilli(end)
	}
	res, err := i.instanceApp.MetricRange(rc.MetaCtx, id, startT, endT)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (i *Instance) TestConn(rc *req.Ctx) {
	f := rc.BindJson[form.InstanceForm]()
	res, err := i.instanceApp.TestConn(rc.MetaCtx, f.ToEntity())
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (i *Instance) Save(rc *req.Ctx) {
	f := rc.BindJson[form.InstanceForm]()
	inst := f.ToEntity()
	inst.Id = f.Id
	rc.ReqParam = collx.Kvs("name", inst.Name, "machineId", inst.MachineId, "callbackUrl", inst.CallbackURL)
	biz.ErrIsNil(i.instanceApp.SaveInstance(rc.MetaCtx, inst, f.ApiToken))
	rc.ResData = inst.Id
}

func (i *Instance) Delete(rc *req.Ctx) {
	id := cast.ToUint64(rc.PathParam("id"))
	rc.ReqParam = id
	biz.ErrIsNil(i.instanceApp.DeleteInstance(rc.MetaCtx, id))
}

func (i *Instance) Restart(rc *req.Ctx) {
	id := cast.ToUint64(rc.PathParam("id"))
	rc.ReqParam = collx.Kvs("instanceId", id)
	res, err := i.instanceApp.Restart(rc.MetaCtx, id)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (i *Instance) BindApp(rc *req.Ctx) {
	f := rc.BindJson[form.BindAppForm]()
	rc.ReqParam = collx.Kvs("instanceId", f.InstanceId, "appSnapshotId", f.AppSnapshotId, "confirmOverwrite", f.ConfirmOverwrite)
	biz.ErrIsNil(i.instanceApp.BindCasdoorApp(rc.MetaCtx, f.InstanceId, f.AppSnapshotId, f.ConfirmOverwrite))
}

func (i *Instance) Users(rc *req.Ctx) {
	q := rc.BindQuery[entity.OctopUserQuery]()
	q.InstanceId = cast.ToUint64(rc.PathParam("id"))
	res, err := i.instanceApp.ListOctopUsers(rc.MetaCtx, q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (i *Instance) GetSettings(rc *req.Ctx) {
	res, err := i.settingsApp.Get(rc.MetaCtx)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (i *Instance) SaveSettings(rc *req.Ctx) {
	f := rc.BindJson[form.SettingsForm]()
	rc.ReqParam = collx.Kvs("eventRetainDays", f.EventRetainDays)
	biz.ErrIsNil(i.settingsApp.SaveSettings(rc.MetaCtx, &entity.OctopSettings{
		EventRetainDays: f.EventRetainDays, MetricSampleSec: f.MetricSampleSec,
		CollectConcurrency: f.CollectConcurrency, CollectTimeoutSec: f.CollectTimeoutSec,
	}))
}
