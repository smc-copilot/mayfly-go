package application

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/langfuse"
	"mayfly-go/internal/pkg/utils"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/i18n"
	"mayfly-go/pkg/model"
)

const (
	traceReadTimeout   = 15 * time.Second
	sessionMapThreadID = "sessionId=thread_id,userId=octop_user_id"
)

// Trace reads Langfuse data only after verifying the source Octop session owner.
type Trace interface {
	GetThreadTraceMeta(ctx context.Context, instanceId uint64, userId, threadId string) (*dto.ThreadTraceMeta, error)
	PageTraces(ctx context.Context, instanceId uint64, userId, threadId string, q *entity.TraceQuery) (*entity.TracePage[entity.BizTrace], error)
	PageObservations(ctx context.Context, instanceId uint64, userId, threadId, traceId string, q *entity.TraceQuery) (*entity.TracePage[entity.BizObservation], error)
	GetObservationContent(ctx context.Context, instanceId uint64, userId, threadId, traceId, observationId string, contentAllowed bool, query ...*entity.TraceQuery) (*entity.BizObservationContent, error)
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
	thread, source, err := a.requireThread(ctx, instanceId, userId, threadId)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	connection := &entity.SourceMeta{Source: "langfuse", State: entity.SourceStateReady, ReadAt: &now}
	meta := &dto.ThreadTraceMeta{Thread: thread, Source: source, Connection: connection, Mapping: sessionMapThreadID, Association: entity.SourceStateNotVerified}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	_, view, err := a.openLangfuse(pctx, instanceId)
	if err != nil {
		connection.State, connection.Message = classifyLangfuseErr(err), langfuseMessage(ctx, err)
		meta.Association, meta.Message = connection.State, connection.Message
		return meta, nil
	}
	meta.ProjectId, meta.ProjectName, meta.Version = view.ProjectId, view.ProjectName, view.Version
	meta.ExternalURL = buildSessionURL(view.BaseURL, view.ProjectId, threadId)
	// Authentication establishes connection capability, not session association.
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
	from, to, fallback, err := traceWindow(thread, q, now)
	if err != nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrTraceQuery)
	}
	page := &entity.TracePage[entity.BizTrace]{
		List: []*entity.BizTrace{}, Total: -1, PageNum: q.PageNum, PageSize: q.PageSize,
		Source:  &entity.SourceMeta{Source: "langfuse", State: entity.SourceStateReady, ReadAt: &now},
		Mapping: sessionMapThreadID, CoverageStart: from, CoverageEnd: to,
	}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	client, view, err := a.openLangfuse(pctx, instanceId)
	if err != nil {
		failTracePage(ctx, page, err)
		return page, nil
	}
	page.ExternalURL = buildSessionURL(view.BaseURL, view.ProjectId, threadId)
	res, err := client.ListTraces(pctx, langfuse.ListTracesQuery{
		SessionId: threadId, UserId: userId, From: from, To: to, Page: q.PageNum, Limit: q.PageSize, Cursor: q.Cursor, TraceId: strings.TrimSpace(q.TraceId),
	})
	if err != nil {
		failTracePage(ctx, page, err)
		return page, nil
	}
	filtered, rejected := recheckTraces(res.Items, threadId, userId, from, to)
	if rejected {
		failTracePage(ctx, page, langfuse.ErrAssociation)
		return page, nil
	}
	if q.HasError != nil {
		filtered = filterTracesByError(filtered, *q.HasError)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		filtered = filterTracesByKeyword(filtered, kw)
	}
	page.List, page.Total, page.HasMore, page.NextCursor = filtered, res.Total, res.HasMore, res.NextCursor
	// Filtering a page cannot establish a global filtered count.
	if q.HasError != nil || strings.TrimSpace(q.Keyword) != "" {
		page.Total = -1
	}
	page.Association = entity.SourceStateNotVerified
	if len(res.Items) > 0 {
		page.Association = entity.SourceStateReady
	}
	if len(res.Items) == 0 && !res.HasMore && q.Cursor == "" && q.PageNum == 1 {
		page.Association = entity.SourceStateNotFoundInRange
		page.Source.Message = i18n.TC(ctx, imsg.MsgTraceNotFoundInRange)
	}
	complete := !res.HasMore
	page.Source.Complete = &complete
	if fallback {
		page.Source.Message = joinTraceMessage(page.Source.Message, i18n.TC(ctx, imsg.MsgTraceDefaultWindow))
	}
	return page, nil
}

func (a *traceAppImpl) PageObservations(ctx context.Context, instanceId uint64, userId, threadId, traceId string, q *entity.TraceQuery) (*entity.TracePage[entity.BizObservation], error) {
	if q == nil {
		q = &entity.TraceQuery{}
	}
	q.Normalize()
	thread, _, err := a.requireThread(ctx, instanceId, userId, threadId)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	from, to, fallback, err := traceWindow(thread, q, now)
	if err != nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrTraceQuery)
	}
	page := &entity.TracePage[entity.BizObservation]{List: []*entity.BizObservation{}, Total: -1, PageNum: q.PageNum, PageSize: q.PageSize, Source: &entity.SourceMeta{Source: "langfuse", State: entity.SourceStateReady, ReadAt: &now}, Mapping: sessionMapThreadID, CoverageStart: from, CoverageEnd: to}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	client, view, err := a.openLangfuse(pctx, instanceId)
	if err != nil {
		failTracePage(ctx, page, err)
		return page, nil
	}
	page.ExternalURL = buildSessionURL(view.BaseURL, view.ProjectId, threadId)
	if err := assertTraceBelongs(pctx, client, userId, threadId, traceId, from, to); err != nil {
		failTracePage(ctx, page, err)
		return page, nil
	}
	res, err := client.ListObservations(pctx, langfuse.ListObservationsQuery{TraceId: traceId, SessionId: threadId, UserId: userId, From: from, To: to, Cursor: q.Cursor, Page: q.PageNum, Limit: q.PageSize})
	if err != nil {
		failTracePage(ctx, page, err)
		return page, nil
	}
	for _, item := range res.Items {
		if item == nil || item.ObservationId == "" || item.TraceId != traceId || item.StartTime == nil || item.StartTime.Before(*from) || !item.StartTime.Before(*to) {
			failTracePage(ctx, page, langfuse.ErrAssociation)
			return page, nil
		}
	}
	items := markOrphans(res.Items)
	if typ := strings.TrimSpace(q.Type); typ != "" {
		items = filterObsByType(items, typ)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		items = filterObsByKeyword(items, kw)
	}
	page.List, page.Total, page.HasMore, page.NextCursor = items, res.Total, res.HasMore, res.NextCursor
	if strings.TrimSpace(q.Type) != "" || strings.TrimSpace(q.Keyword) != "" {
		page.Total = -1
	}
	page.Association = entity.SourceStateReady
	complete := !res.HasMore
	page.Source.Complete = &complete
	if fallback {
		page.Source.Message = i18n.TC(ctx, imsg.MsgTraceDefaultWindow)
	}
	return page, nil
}

func (a *traceAppImpl) GetObservationContent(ctx context.Context, instanceId uint64, userId, threadId, traceId, observationId string, contentAllowed bool, query ...*entity.TraceQuery) (*entity.BizObservationContent, error) {
	if !contentAllowed {
		return nil, errorx.NewBizI(ctx, imsg.ErrBusinessContentDenied)
	}
	thread, _, err := a.requireThread(ctx, instanceId, userId, threadId)
	if err != nil {
		return nil, err
	}
	q := &entity.TraceQuery{}
	if len(query) > 0 && query[0] != nil {
		q = query[0]
	}
	from, to, _, err := traceWindow(thread, q, time.Now())
	if err != nil || strings.TrimSpace(observationId) == "" {
		return nil, errorx.NewBizI(ctx, imsg.ErrTraceQuery)
	}
	pctx, cancel := context.WithTimeout(ctx, traceReadTimeout)
	defer cancel()
	client, _, err := a.openLangfuse(pctx, instanceId)
	if err != nil {
		return nil, errorx.NewBiz(langfuseMessage(ctx, err))
	}
	if err := assertTraceBelongs(pctx, client, userId, threadId, traceId, from, to); err != nil {
		return nil, errorx.NewBiz(langfuseMessage(ctx, err))
	}
	obs, content, err := client.GetObservationInScope(pctx, observationId, langfuse.ListObservationsQuery{TraceId: traceId, SessionId: threadId, UserId: userId, From: from, To: to})
	if err != nil {
		return nil, errorx.NewBiz(langfuseMessage(ctx, err))
	}
	if obs == nil || content == nil || obs.TraceId != traceId || obs.ObservationId != observationId || content.ObservationId != observationId {
		return nil, errorx.NewBizI(ctx, imsg.ErrLangfuseAssociation)
	}
	return content, nil
}

func (a *traceAppImpl) requireThread(ctx context.Context, instanceId uint64, userId, threadId string) (*entity.BizThread, *entity.SourceMeta, error) {
	if strings.TrimSpace(userId) == "" || strings.TrimSpace(threadId) == "" {
		return nil, nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	if _, err := a.instanceApp.GetVisible(ctx, instanceId); err != nil {
		return nil, nil, err
	}
	detail, err := a.businessApp.GetThread(ctx, instanceId, userId, threadId)
	if err != nil {
		return nil, nil, err
	}
	if detail == nil || detail.Item == nil || detail.Item.ThreadId != threadId {
		return nil, nil, errorx.NewBizI(ctx, imsg.ErrBusinessNotFound)
	}
	if detail.Source == nil || detail.Source.State != entity.SourceStateReady {
		return nil, nil, errorx.NewBizI(ctx, imsg.ErrBusinessSourceFailed)
	}
	return detail.Item, detail.Source, nil
}

func assertTraceBelongs(ctx context.Context, client *langfuse.Client, userId, threadId, traceId string, from, to *time.Time) error {
	if strings.TrimSpace(traceId) == "" {
		return langfuse.ErrQuery
	}
	// Query this exact trace rather than checking only the first 100 session traces.
	q := langfuse.ListTracesQuery{TraceId: traceId, SessionId: threadId, UserId: userId, From: from, To: to, Limit: 1}
	seen := map[string]bool{}
	for scanned := 0; scanned < 32; scanned++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		res, err := client.ListTraces(ctx, q)
		if err != nil {
			return err
		}
		checked, rejected := recheckTraces(res.Items, threadId, userId, from, to)
		if rejected {
			return langfuse.ErrAssociation
		}
		if len(checked) > 0 {
			if checked[0].TraceId != traceId {
				return langfuse.ErrAssociation
			}
			return nil
		}
		if !res.HasMore || res.NextCursor == "" {
			return langfuse.ErrAssociation
		}
		if seen[res.NextCursor] {
			return langfuse.ErrUnsupported
		}
		seen[res.NextCursor] = true
		q.Cursor = res.NextCursor
	}
	return langfuse.ErrAssociation
}

type langfuseConnView struct{ BaseURL, ProjectId, ProjectName, Version string }

func (a *traceAppImpl) openLangfuse(ctx context.Context, instanceId uint64) (*langfuse.Client, *langfuseConnView, error) {
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
	client := a.clientFactory()(langfuse.Config{BaseURL: row.BaseURL, ProjectId: row.ProjectId, PublicKey: row.PublicKey, SecretKey: secret, Timeout: traceReadTimeout})
	info, err := client.Verify(ctx)
	if err != nil {
		return nil, nil, err
	}
	// A key authenticates a project, not an Octop instance. Without an instance
	// discriminator shared projects cannot be safely disambiguated.
	peers, err := a.connRepo.SelectByCond(model.NewCond().Eq("type", entity.ReadConnTypeLangfuse), "instance_id", "base_url", "project_id", "public_key", "credential_configured", "verify_state", "revision", "extra_json")
	if err != nil {
		return nil, nil, err
	}
	for _, peer := range peers {
		if peer.InstanceId != instanceId && peer.CredentialConfigured == 1 && sameBaseURL(peer.BaseURL, row.BaseURL) {
			project, verified := verifiedLangfuseProject(peer)
			if !verified || project == info.ProjectId || peer.PublicKey == row.PublicKey {
				return nil, nil, langfuse.ErrAssociation
			}
		}
	}
	return client, &langfuseConnView{BaseURL: row.BaseURL, ProjectId: info.ProjectId, ProjectName: info.ProjectName, Version: info.Version}, nil
}

var errLangfuseNotConfigured = errors.New("langfuse not configured")

func classifyLangfuseErr(err error) string {
	switch {
	case err == nil:
		return entity.SourceStateReady
	case errors.Is(err, errLangfuseNotConfigured):
		return entity.SourceStateNotConfigured
	case errors.Is(err, langfuse.ErrForbidden):
		return entity.SourceStateForbidden
	case errors.Is(err, langfuse.ErrAssociation), errors.Is(err, langfuse.ErrProject):
		return entity.SourceStateAssociationUnverified
	case errors.Is(err, langfuse.ErrUnsupported), errors.Is(err, langfuse.ErrConfig):
		return entity.SourceStateUnsupported
	default:
		return entity.SourceStateFailed
	}
}

func langfuseMessage(ctx context.Context, err error) string {
	var he *langfuse.HTTPError
	var timeout net.Error
	switch {
	case err == nil:
		return i18n.TC(ctx, imsg.MsgLangfuseReady)
	case errors.Is(err, errLangfuseNotConfigured):
		return i18n.TC(ctx, imsg.ErrLangfuseNotConfigured)
	case errors.Is(err, langfuse.ErrForbidden):
		return i18n.TC(ctx, imsg.ErrLangfuseForbidden)
	case errors.Is(err, langfuse.ErrProject):
		return i18n.TC(ctx, imsg.ErrLangfuseProject)
	case errors.Is(err, langfuse.ErrAssociation):
		return i18n.TC(ctx, imsg.ErrLangfuseAssociation)
	case errors.Is(err, langfuse.ErrQuery):
		return i18n.TC(ctx, imsg.ErrTraceQuery)
	case errors.Is(err, langfuse.ErrConfig), errors.Is(err, langfuse.ErrUnsupported):
		return i18n.TC(ctx, imsg.ErrLangfuseUnsupported)
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout():
		return i18n.TC(ctx, imsg.ErrLangfuseTimeout)
	case errors.As(err, &he) && he.StatusCode == http.StatusTooManyRequests:
		return i18n.TC(ctx, imsg.ErrLangfuseRateLimited)
	default:
		return i18n.TC(ctx, imsg.ErrLangfuseFailed)
	}
}

func failTracePage[T any](ctx context.Context, page *entity.TracePage[T], err error) {
	page.Source.State, page.Source.Message = classifyLangfuseErr(err), langfuseMessage(ctx, err)
	complete := false
	page.Source.Complete = &complete
	page.Association = page.Source.State
	page.ExternalURL = ""
}

func traceWindow(thread *entity.BizThread, q *entity.TraceQuery, now time.Time) (*time.Time, *time.Time, bool, error) {
	from, to := now.Add(-30*24*time.Hour), now
	fallback := thread == nil || thread.CreatedAt == nil
	if !fallback {
		from = *thread.CreatedAt
	}
	if q.Cursor != "" && (q.StartTime == "" || q.EndTime == "") {
		return nil, nil, false, langfuse.ErrQuery
	}
	if strings.TrimSpace(q.StartTime) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, q.StartTime)
		if err != nil {
			return nil, nil, false, langfuse.ErrQuery
		}
		from = parsed
		fallback = false
	}
	if strings.TrimSpace(q.EndTime) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, q.EndTime)
		if err != nil {
			return nil, nil, false, langfuse.ErrQuery
		}
		to = parsed
	}
	if !from.Before(to) || len(q.Cursor) > 8192 {
		return nil, nil, false, langfuse.ErrQuery
	}
	return &from, &to, fallback, nil
}

func recheckTraces(items []*entity.BizTrace, threadId, userId string, from, to *time.Time) (kept []*entity.BizTrace, rejected bool) {
	kept = make([]*entity.BizTrace, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item == nil || item.TraceId == "" || item.SessionId != threadId || item.UserId != userId || item.StartTime == nil || item.StartTime.Before(*from) || !item.StartTime.Before(*to) {
			return nil, true
		}
		if !seen[item.TraceId] {
			kept = append(kept, item)
			seen[item.TraceId] = true
		}
	}
	return kept, false
}

func joinTraceMessage(first, second string) string {
	if first == "" {
		return second
	}
	return first + "; " + second
}

func markOrphans(items []*entity.BizObservation) []*entity.BizObservation {
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.ObservationId] = true
	}
	for _, item := range items {
		item.Orphan = item.ParentId != "" && !ids[item.ParentId]
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

func buildSessionURL(baseURL, projectId, sessionId string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" || projectId == "" || sessionId == "" {
		return ""
	}
	return base + "/project/" + url.PathEscape(projectId) + "/sessions/" + url.PathEscape(sessionId)
}
