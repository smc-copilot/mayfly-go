package api

import (
	"bytes"

	"mayfly-go/internal/octop/api/form"
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/req"
	"mayfly-go/pkg/utils/collx"
)

type Event struct {
	eventApp application.Event `inject:"T"`
}

func (e *Event) ReqConfs() *req.Confs {
	reqs := [...]*req.Conf{
		req.NewGet("/events", e.Query).RequiredPermissionCode("octop:event:base"),
		req.NewGet("/events/summary", e.Summary).RequiredPermissionCode("octop:event:base"),
		req.NewGet("/events/export", e.Export).Log(req.NewLogSaveI(imsg.LogOctopEventExport)).RequiredPermissionCode("octop:event:export"),
		req.NewPost("/events/recollect", e.Recollect).Log(req.NewLogSaveI(imsg.LogOctopRecollect)).RequiredPermissionCode("octop:collect:recollect"),
	}
	return req.NewConfs("/octop", reqs[:]...)
}

func (e *Event) Query(rc *req.Ctx) {
	q := rc.BindQuery[entity.EventQuery]()
	res, err := e.eventApp.Query(rc.MetaCtx, q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (e *Event) Summary(rc *req.Ctx) {
	q := rc.BindQuery[entity.EventQuery]()
	groupBy := rc.Query("groupBy")
	res, err := e.eventApp.Summary(rc.MetaCtx, q, groupBy)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (e *Event) Export(rc *req.Ctx) {
	q := rc.BindQuery[entity.EventQuery]()
	rc.ReqParam = collx.Kvs("instanceId", q.InstanceId, "category", q.Category)
	data, filename, err := e.eventApp.Export(rc.MetaCtx, q)
	biz.ErrIsNil(err)
	rc.Download(bytes.NewReader(data), filename)
}

func (e *Event) Recollect(rc *req.Ctx) {
	f := rc.BindJson[form.RecollectForm]()
	rc.ReqParam = collx.Kvs("instanceId", f.InstanceId, "sourceTable", f.SourceTable)
	biz.ErrIsNil(e.eventApp.Recollect(rc.MetaCtx, f.InstanceId, f.SourceTable))
}
