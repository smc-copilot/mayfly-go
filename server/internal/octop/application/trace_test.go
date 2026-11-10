package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/infra/langfuse"

	"github.com/stretchr/testify/require"
)

type stubBusinessForTrace struct {
	thread *entity.BizThread
	err    error
}

func (s *stubBusinessForTrace) Capabilities(context.Context, uint64, dto.CapabilityScope) (*dto.Capabilities, error) {
	return nil, nil
}
func (s *stubBusinessForTrace) PageUserOptions(context.Context, uint64, *entity.BusinessQuery) (*dto.BusinessPage[entity.BizUserOption], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) PageUsers(context.Context, uint64, *entity.BusinessQuery) (*dto.BusinessPage[entity.BizUser], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) GetUserDetail(context.Context, uint64, string) (*dto.UserDetail, error) {
	return nil, nil
}
func (s *stubBusinessForTrace) PageExperts(context.Context, uint64, string, *entity.BusinessQuery) (*dto.BusinessPage[entity.BizExpert], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) GetExpert(context.Context, uint64, string, string, bool, bool) (*dto.BusinessDetail[entity.BizExpert], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) PageThreads(context.Context, uint64, string, *entity.BusinessQuery) (*dto.BusinessPage[entity.BizThread], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) GetThread(context.Context, uint64, string, string) (*dto.BusinessDetail[entity.BizThread], error) {
	if s.err != nil {
		return nil, s.err
	}
	return &dto.BusinessDetail[entity.BizThread]{Item: s.thread, Source: readyMeta()}, nil
}
func (s *stubBusinessForTrace) PageExpertTemplates(context.Context, uint64, string, *entity.BusinessQuery) (*dto.BusinessPage[entity.BizExpertTemplate], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) GetExpertTemplate(context.Context, uint64, string, string) (*dto.BusinessDetail[entity.BizExpertTemplate], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) PageSkillPackages(context.Context, uint64, string, *entity.BusinessQuery) (*dto.BusinessPage[entity.BizSkillPackage], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) GetSkillPackage(context.Context, uint64, string, string) (*dto.BusinessDetail[entity.BizSkillPackage], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) PageSkills(context.Context, uint64, string, string, *entity.BusinessQuery) (*dto.BusinessPage[entity.BizSkill], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) GetSkill(context.Context, uint64, string, string, string) (*dto.BusinessDetail[entity.BizSkill], error) {
	return nil, nil
}
func (s *stubBusinessForTrace) GetFileContent(context.Context, *dto.FileContentQuery, bool) (*entity.BizFileContent, error) {
	return nil, nil
}

func TestPageTracesRechecksAssociation(t *testing.T) {
	setupReadConnDB(t)
	created := time.Now().Add(-time.Hour)
	biz := &stubBusinessForTrace{thread: &entity.BizThread{ThreadId: "th1", CreatedAt: &created}}
	access := &stubInstanceAccess{inst: &entity.OctopInstance{Id: 9}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/public/health":
			_, _ = w.Write([]byte(`{"status":"OK"}`))
		case "/api/public/traces":
			// 故意返回错误 sessionId，应用层应拒绝整批
			_, _ = w.Write([]byte(`{"data":[{"id":"t1","sessionId":"OTHER","userId":"u1"}],"meta":{"page":1,"limit":20,"totalItems":1,"totalPages":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := newTestReadConnRepo()
	appRC := &readConnectionAppImpl{connRepo: repo, instanceApp: access}
	_, err := appRC.Save(context.Background(), 9, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, PublicKey: "pk", Secret: "sk", Revision: 0,
	})
	require.NoError(t, err)

	traceApp := &traceAppImpl{
		instanceApp: access,
		connRepo:    repo,
		businessApp: biz,
		newClient:   langfuse.NewClient,
	}
	page, err := traceApp.PageTraces(context.Background(), 9, "u1", "th1", &entity.TraceQuery{})
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateAssociationUnverified, page.Association)
	require.Empty(t, page.List)
}

func TestPageTracesReadyAndContentDenied(t *testing.T) {
	setupReadConnDB(t)
	created := time.Now().Add(-time.Hour)
	biz := &stubBusinessForTrace{thread: &entity.BizThread{ThreadId: "th1", CreatedAt: &created}}
	access := &stubInstanceAccess{inst: &entity.OctopInstance{Id: 10}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/public/health":
			_, _ = w.Write([]byte(`{"status":"OK"}`))
		case r.URL.Path == "/api/public/traces":
			_, _ = w.Write([]byte(`{"data":[{"id":"t1","sessionId":"th1","userId":"u1","name":"n1"}],"meta":{"page":1,"limit":20,"totalItems":1,"totalPages":1}}`))
		case r.URL.Path == "/api/public/observations" && r.URL.Query().Get("traceId") == "t1":
			_, _ = w.Write([]byte(`{"data":[{"id":"o1","traceId":"t1","type":"GENERATION","name":"llm"}],"meta":{"page":1,"limit":50,"totalItems":1,"totalPages":1}}`))
		case r.URL.Path == "/api/public/observations/o1":
			_, _ = w.Write([]byte(`{"id":"o1","traceId":"t1","input":"in","output":"out"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := newTestReadConnRepo()
	appRC := &readConnectionAppImpl{connRepo: repo, instanceApp: access}
	_, err := appRC.Save(context.Background(), 10, &dto.ReadConnectionSave{
		Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, PublicKey: "pk", Secret: "sk", Revision: 0,
	})
	require.NoError(t, err)

	traceApp := &traceAppImpl{instanceApp: access, connRepo: repo, businessApp: biz, newClient: langfuse.NewClient}
	page, err := traceApp.PageTraces(context.Background(), 10, "u1", "th1", &entity.TraceQuery{})
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, page.Association)
	require.Len(t, page.List, 1)

	obs, err := traceApp.PageObservations(context.Background(), 10, "u1", "th1", "t1", &entity.TraceQuery{})
	require.NoError(t, err)
	require.Len(t, obs.List, 1)

	_, err = traceApp.GetObservationContent(context.Background(), 10, "u1", "th1", "t1", "o1", false)
	require.Error(t, err)

	content, err := traceApp.GetObservationContent(context.Background(), 10, "u1", "th1", "t1", "o1", true)
	require.NoError(t, err)
	require.Equal(t, "in", content.Input)
	require.Equal(t, "out", content.Output)
}

func TestGetThreadTraceMetaNotConfigured(t *testing.T) {
	setupReadConnDB(t)
	biz := &stubBusinessForTrace{thread: &entity.BizThread{ThreadId: "th1"}}
	access := &stubInstanceAccess{inst: &entity.OctopInstance{Id: 11}}
	traceApp := &traceAppImpl{
		instanceApp: access,
		connRepo:    newTestReadConnRepo(),
		businessApp: biz,
	}
	meta, err := traceApp.GetThreadTraceMeta(context.Background(), 11, "u1", "th1")
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateNotConfigured, meta.Association)
}
