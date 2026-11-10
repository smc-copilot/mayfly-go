package api

import (
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/req"
)

// TraceAPI 会话 Langfuse 追踪读取。
type TraceAPI struct {
	traceApp application.Trace `inject:"T"`
}

func (t *TraceAPI) ReqConfs() *req.Confs {
	const p = "/instances/:id/business/users/:userId/threads/:threadId"
	reqs := [...]*req.Conf{
		// 与 business.Thread 路径区分：本路由专供追踪关联状态。
		req.NewGet(p+"/trace-status", t.ThreadMeta).RequiredPermissionCode(permSessionBase),
		req.NewGet(p+"/traces", t.Traces).RequiredPermissionCode(permTraceBase),
		req.NewGet(p+"/traces/:traceId/observations", t.Observations).RequiredPermissionCode(permTraceBase),
		req.NewGet(p+"/traces/:traceId/observations/:observationId/content", t.Content).RequiredPermissionCode(permContentRead),
	}
	return req.NewConfs("/octop", reqs[:]...)
}

func (t *TraceAPI) ThreadMeta(rc *req.Ctx) {
	res, err := t.traceApp.GetThreadTraceMeta(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("threadId"))
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (t *TraceAPI) Traces(rc *req.Ctx) {
	q := rc.BindQuery[entity.TraceQuery]()
	res, err := t.traceApp.PageTraces(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("threadId"), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (t *TraceAPI) Observations(rc *req.Ctx) {
	q := rc.BindQuery[entity.TraceQuery]()
	res, err := t.traceApp.PageObservations(rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("threadId"), rc.PathParam("traceId"), q)
	biz.ErrIsNil(err)
	rc.ResData = res
}

func (t *TraceAPI) Content(rc *req.Ctx) {
	contentAllowed := hasAnyPerm(rc, permContentRead)
	res, err := t.traceApp.GetObservationContent(
		rc.MetaCtx, instanceIdOf(rc), rc.PathParam("userId"), rc.PathParam("threadId"),
		rc.PathParam("traceId"), rc.PathParam("observationId"), contentAllowed,
	)
	biz.ErrIsNil(err)
	rc.ResData = res
}
