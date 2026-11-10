package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/langfuse"
	"mayfly-go/internal/pkg/utils"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/logx"
)

const (
	traceReadTimeout = 15 * time.Second
	// 默认映射版本：候选 sessionId = threadId；必须复核返回行的 sessionId/userId。
	sessionMapThreadID = "sessionId=thread_id"
)

// Trace 会话运行追踪（Langfuse）。
type Trace interface {
	// GetThreadTraceMeta 返回会话元数据 + 追踪关联状态（不拉 Trace 列表）。
	GetThreadTraceMeta(ctx context.Context, instanceId uint64, userId, threadId string) (*dto.ThreadTraceMeta, error)
	PageTraces(ctx context.Context, instanceId uint64, userId, threadId string, q *entity.TraceQuery) (*entity.TracePage[entity.BizTrace], error)
	PageObservations(ctx context.Context, instanceId uint64, userId, threadId, traceId string, q *entity.TraceQuery) (*entity.TracePage[entity.BizObservation], error)
	GetObservationContent(ctx context.Context, instanceId uint64, userId, threadId, traceId, observationId string, contentAllowed bool) (*entity.BizObservationContent, error)
}

type langfuseFactory func(cfg langfuse.Config) *langfuse.Client

type traceAppImpl struct {
	instanceApp instanceAccessor          `inject:"T"`
	connRepo    repository.ReadConnection `inject:"T"`
	businessApp Business                  `inject:"T"`
	newClient   langfuseFactory
}

var _ Trace = (*traceAppImpl)(nil)

func (a *traceAppImpl) clientFactory() langfuseFactory {
	if a.newClient != nil {
		return a.newClient
	}
	return langfuse.NewClient
}

func (a *traceAppImpl) GetThreadTraceMeta(ctx context.Context, instanceId uint64, userId, threadId string) (*dto.ThreadTraceMeta, error) {
	thread, src, err := a.requireThread(ctx, instanceId, userId, threadId)
	if err != nil {
		return nil, err
	}
	meta := &dto.ThreadTraceMeta{Thread: thread, Source: src, Association: entity.SourceStateNotConfigured}
	client, cfgView, err := a.openLangfuse(ctx, instanceId)
	if err != nil {
		meta.Association = classifyLangfuseOpenErr(err)
		meta.Message = errMessage(err)
		return meta, nil
	}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	if _, err := client.Probe(pctx); err != nil {
		meta.Association = classifyLangfuseErr(err)
		meta.Message = "Langfuse 连接探测失败"
		logx.Errorf("octop langfuse probe instance=%d err=%v", instanceId, err)
		return meta, nil
	}
	meta.Association = entity.SourceStateReady
	meta.ProjectId = cfgView.ProjectId
	meta.Mapping = sessionMapThreadID
	meta.ExternalURL = buildSessionURL(cfgView.BaseURL, threadId)
	return meta, nil
}

func (a *traceAppImpl) PageTraces(ctx context.Context, instanceId uint64, userId, threadId string, q *entity.TraceQuery) (*entity.TracePage[entity.BizTrace], error) {
	if q == nil {
		q = &entity.TraceQuery{}
	}
	q.Normalize()
	thread, _, err := a.requireThread(ctx, instanceId, userId, threadId)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	page := &entity.TracePage[entity.BizTrace]{
		List: []*entity.BizTrace{}, Total: 0, PageNum: q.PageNum, PageSize: q.PageSize,
		Source:  &entity.SourceMeta{Source: "langfuse", State: entity.SourceStateReady, ReadAt: &now},
		Mapping: sessionMapThreadID,
	}

	client, cfgView, err := a.openLangfuse(ctx, instanceId)
	if err != nil {
		page.Source.State = classifyLangfuseOpenErr(err)
		page.Source.Message = errMessage(err)
		page.Association = page.Source.State
		return page, nil
	}
	from, to := traceWindow(thread, q, now)
	page.CoverageStart = from
	page.CoverageEnd = to
	page.ExternalURL = buildSessionURL(cfgView.BaseURL, threadId)

	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	res, err := client.ListTraces(pctx, langfuse.ListTracesQuery{
		SessionId: threadId, UserId: userId, From: from, To: to, Page: q.PageNum, Limit: q.PageSize,
	})
	if err != nil {
		page.Source.State = classifyLangfuseErr(err)
		page.Source.Message = "读取 Trace 失败"
		page.Association = page.Source.State
		logx.Errorf("octop langfuse list traces instance=%d thread=%s err=%v", instanceId, threadId, err)
		return page, nil
	}

	filtered, rejected := recheckTraces(res.Items, threadId, userId, instanceId)
	if rejected {
		page.Association = entity.SourceStateAssociationUnverified
		page.Source.State = entity.SourceStateAssociationUnverified
		page.Source.Message = "上游返回越界数据，已拒绝该批次"
		page.List = []*entity.BizTrace{}
		return page, nil
	}
	if q.HasError != nil {
		filtered = filterTracesByError(filtered, *q.HasError)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		filtered = filterTracesByKeyword(filtered, kw)
	}
	if tid := strings.TrimSpace(q.TraceId); tid != "" {
		filtered = filterTracesByID(filtered, tid)
	}
	page.List = filtered
	page.Total = res.Total
	page.HasMore = res.HasMore
	page.Association = entity.SourceStateReady
	if len(filtered) == 0 && !res.HasMore {
		page.Association = entity.SourceStateNotFoundInRange
		page.Source.State = entity.SourceStateNotFoundInRange
		page.Source.Message = "查询范围内暂未发现追踪（可能延迟上报）"
	}
	complete := !res.HasMore
	page.Source.Complete = &complete
	return page, nil
}

func (a *traceAppImpl) PageObservations(ctx context.Context, instanceId uint64, userId, threadId, traceId string, q *entity.TraceQuery) (*entity.TracePage[entity.BizObservation], error) {
	if q == nil {
		q = &entity.TraceQuery{}
	}
	q.Normalize()
	if _, _, err := a.requireThread(ctx, instanceId, userId, threadId); err != nil {
		return nil, err
	}
	now := time.Now()
	page := &entity.TracePage[entity.BizObservation]{
		List: []*entity.BizObservation{}, Total: 0, PageNum: q.PageNum, PageSize: q.PageSize,
		Source: &entity.SourceMeta{Source: "langfuse", State: entity.SourceStateReady, ReadAt: &now},
	}
	// 先确认该 Trace 属于本会话（防伪造 traceId）
	if err := a.assertTraceBelongs(ctx, instanceId, userId, threadId, traceId); err != nil {
		return nil, err
	}
	client, _, err := a.openLangfuse(ctx, instanceId)
	if err != nil {
		page.Source.State = classifyLangfuseOpenErr(err)
		page.Source.Message = errMessage(err)
		return page, nil
	}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	res, err := client.ListObservations(pctx, langfuse.ListObservationsQuery{TraceId: traceId, Page: q.PageNum, Limit: q.PageSize})
	if err != nil {
		page.Source.State = classifyLangfuseErr(err)
		page.Source.Message = "读取 Observation 失败"
		return page, nil
	}
	items := markOrphans(res.Items)
	if typ := strings.TrimSpace(q.Type); typ != "" {
		items = filterObsByType(items, typ)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		items = filterObsByKeyword(items, kw)
	}
	page.List = items
	page.Total = res.Total
	page.HasMore = res.HasMore
	complete := !res.HasMore
	page.Source.Complete = &complete
	page.Association = entity.SourceStateReady
	return page, nil
}

func (a *traceAppImpl) GetObservationContent(ctx context.Context, instanceId uint64, userId, threadId, traceId, observationId string, contentAllowed bool) (*entity.BizObservationContent, error) {
	if !contentAllowed {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessContentDenied)
	}
	if _, _, err := a.requireThread(ctx, instanceId, userId, threadId); err != nil {
		return nil, err
	}
	if err := a.assertTraceBelongs(ctx, instanceId, userId, threadId, traceId); err != nil {
		return nil, err
	}
	client, _, err := a.openLangfuse(ctx, instanceId)
	if err != nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrPreviewNotConfigured)
	}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	obs, content, err := client.GetObservation(pctx, observationId)
	if err != nil {
		if errors.Is(err, langfuse.ErrForbidden) {
			return nil, errorx.NewBizI(ctx, imsg.ErrBusinessContentDenied)
		}
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}
	if obs.TraceId != "" && obs.TraceId != traceId {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	return content, nil
}

// --- helpers ---

func (a *traceAppImpl) requireThread(ctx context.Context, instanceId uint64, userId, threadId string) (*entity.BizThread, *entity.SourceMeta, error) {
	detail, err := a.businessApp.GetThread(ctx, instanceId, userId, threadId)
	if err != nil {
		return nil, nil, err
	}
	if detail == nil || detail.Item == nil {
		return nil, nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	return detail.Item, detail.Source, nil
}

func (a *traceAppImpl) assertTraceBelongs(ctx context.Context, instanceId uint64, userId, threadId, traceId string) error {
	if strings.TrimSpace(traceId) == "" {
		return errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	client, _, err := a.openLangfuse(ctx, instanceId)
	if err != nil {
		return errorx.NewBizI(ctx, imsg.ErrPreviewNotConfigured)
	}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	res, err := client.ListTraces(pctx, langfuse.ListTracesQuery{SessionId: threadId, UserId: userId, Limit: 100})
	if err != nil {
		return errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}
	for _, t := range res.Items {
		if t.TraceId == traceId {
			if t.SessionId != "" && t.SessionId != threadId {
				return errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
			}
			if t.UserId != "" && t.UserId != userId {
				return errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
			}
			return nil
		}
	}
	return errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
}

type langfuseConnView struct {
	BaseURL   string
	ProjectId string
}

func (a *traceAppImpl) openLangfuse(ctx context.Context, instanceId uint64) (*langfuse.Client, *langfuseConnView, error) {
	if _, err := a.instanceApp.GetVisible(ctx, instanceId); err != nil {
		return nil, nil, err
	}
	row, err := a.connRepo.GetByInstanceType(ctx, instanceId, entity.ReadConnTypeLangfuse)
	if err != nil {
		return nil, nil, err
	}
	if row == nil || !row.HasCredential() || strings.TrimSpace(row.BaseURL) == "" || strings.TrimSpace(row.PublicKey) == "" {
		return nil, nil, errLangfuseNotConfigured
	}
	secret, err := utils.PwdAesDecrypt(row.SecretEnc)
	if err != nil {
		return nil, nil, err
	}
	client := a.clientFactory()(langfuse.Config{
		BaseURL: row.BaseURL, PublicKey: row.PublicKey, SecretKey: secret, Timeout: traceReadTimeout,
	})
	return client, &langfuseConnView{BaseURL: row.BaseURL, ProjectId: row.ProjectId}, nil
}

var errLangfuseNotConfigured = errors.New("langfuse not configured")

func classifyLangfuseOpenErr(err error) string {
	if errors.Is(err, errLangfuseNotConfigured) {
		return entity.SourceStateNotConfigured
	}
	return entity.SourceStateFailed
}

func classifyLangfuseErr(err error) string {
	switch {
	case errors.Is(err, langfuse.ErrForbidden):
		return entity.SourceStateForbidden
	case errors.Is(err, langfuse.ErrUnsupported), errors.Is(err, langfuse.ErrConfig):
		return entity.SourceStateUnsupported
	default:
		return entity.SourceStateFailed
	}
}

func errMessage(err error) string {
	if errors.Is(err, errLangfuseNotConfigured) {
		return "未配置 Langfuse 读取连接"
	}
	return "Langfuse 不可用"
}

func traceWindow(thread *entity.BizThread, q *entity.TraceQuery, now time.Time) (*time.Time, *time.Time) {
	var from, to *time.Time
	to = &now
	if thread != nil && thread.CreatedAt != nil {
		from = thread.CreatedAt
	}
	if s := strings.TrimSpace(q.StartTime); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = &t
		}
	}
	if s := strings.TrimSpace(q.EndTime); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = &t
		}
	}
	return from, to
}

// recheckTraces 复核稳定标识；出现越界则 rejected=true，调用方拒绝整批。
func recheckTraces(items []*entity.BizTrace, threadId, userId string, instanceId uint64) (kept []*entity.BizTrace, rejected bool) {
	_ = instanceId
	kept = make([]*entity.BizTrace, 0, len(items))
	for _, t := range items {
		if t == nil {
			continue
		}
		if t.SessionId != "" && t.SessionId != threadId {
			return nil, true
		}
		if t.UserId != "" && t.UserId != userId {
			return nil, true
		}
		kept = append(kept, t)
	}
	return kept, false
}

func markOrphans(items []*entity.BizObservation) []*entity.BizObservation {
	ids := make(map[string]struct{}, len(items))
	for _, o := range items {
		if o != nil {
			ids[o.ObservationId] = struct{}{}
		}
	}
	for _, o := range items {
		if o == nil {
			continue
		}
		if o.ParentId != "" {
			if _, ok := ids[o.ParentId]; !ok {
				o.Orphan = true
			}
		}
	}
	return items
}

func filterTracesByError(items []*entity.BizTrace, want bool) []*entity.BizTrace {
	out := make([]*entity.BizTrace, 0, len(items))
	for _, t := range items {
		if t.HasError == want {
			out = append(out, t)
		}
	}
	return out
}

func filterTracesByKeyword(items []*entity.BizTrace, kw string) []*entity.BizTrace {
	kw = strings.ToLower(kw)
	out := make([]*entity.BizTrace, 0, len(items))
	for _, t := range items {
		if strings.Contains(strings.ToLower(t.TraceId), kw) || strings.Contains(strings.ToLower(t.Name), kw) {
			out = append(out, t)
		}
	}
	return out
}

func filterTracesByID(items []*entity.BizTrace, id string) []*entity.BizTrace {
	out := make([]*entity.BizTrace, 0, 1)
	for _, t := range items {
		if t.TraceId == id {
			out = append(out, t)
		}
	}
	return out
}

func filterObsByType(items []*entity.BizObservation, typ string) []*entity.BizObservation {
	out := make([]*entity.BizObservation, 0, len(items))
	for _, o := range items {
		if strings.EqualFold(o.Type, typ) {
			out = append(out, o)
		}
	}
	return out
}

func filterObsByKeyword(items []*entity.BizObservation, kw string) []*entity.BizObservation {
	kw = strings.ToLower(kw)
	out := make([]*entity.BizObservation, 0, len(items))
	for _, o := range items {
		if strings.Contains(strings.ToLower(o.Name), kw) || strings.Contains(strings.ToLower(o.ObservationId), kw) || strings.Contains(strings.ToLower(o.Model), kw) {
			out = append(out, o)
		}
	}
	return out
}

func buildSessionURL(baseURL, sessionId string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" || sessionId == "" {
		return ""
	}
	return fmt.Sprintf("%s/project/sessions/%s", base, sessionId)
}
