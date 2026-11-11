package application

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/model"

	"github.com/stretchr/testify/require"
)

func traceTestApp(t *testing.T, baseURL, project string, created *time.Time) (*traceAppImpl, *readConnectionAppImpl) {
	t.Helper()
	setupReadConnDB(t)
	access := &stubInstanceAccess{inst: &entity.OctopInstance{Id: 9}}
	repo := newTestReadConnRepo()
	rc := &readConnectionAppImpl{connRepo: repo, instanceApp: access}
	_, err := rc.Save(context.Background(), 9, &dto.ReadConnectionSave{Type: entity.ReadConnTypeLangfuse, BaseURL: baseURL, ProjectId: project, PublicKey: "pk", Secret: "sk"})
	require.NoError(t, err)
	return &traceAppImpl{instanceApp: access, connRepo: repo, businessApp: &stubBusinessForTrace{thread: &entity.BizThread{ThreadId: "th1", CreatedAt: created}}}, rc
}

func appV4Row(start time.Time) map[string]any {
	return map[string]any{"id": "root-1", "projectId": "project-1", "traceId": "t1", "sessionId": "th1", "userId": "u1", "startTime": start.UTC().Format(time.RFC3339Nano), "type": "SPAN", "name": "chat turn", "isRootObservation": true}
}

func TestV4TraceApplicationFlow(t *testing.T) {
	created := time.Now().Add(-time.Hour)
	start := created.Add(time.Minute)
	calls := 0
	srv := traceFixture(t, "4.70.1", func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/api/public/v2/observations", r.URL.Path)
		require.Equal(t, "th1", r.URL.Query().Get("sessionId"))
		require.Contains(t, []string{"u1", "other-user"}, r.URL.Query().Get("userId"))
		row := appV4Row(start)
		cursor := ""
		if strings.Contains(r.URL.Query().Get("fields"), "io") {
			require.NotEmpty(t, r.URL.Query().Get("filter"))
			row["input"] = map[string]any{"password": "never-return", "text": "hello"}
			row["output"] = "world"
		} else if r.URL.Query().Get("traceId") == "" {
			cursor = "real-cursor"
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{row}, "meta": map[string]any{"cursor": cursor}}))
	})
	app, _ := traceTestApp(t, srv.URL, "project-1", &created)
	ctx := context.Background()
	meta, err := app.GetThreadTraceMeta(ctx, 9, "u1", "th1")
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, meta.Connection.State)
	require.Equal(t, entity.SourceStateNotVerified, meta.Association, "health/authentication does not prove a session association")
	require.Equal(t, "4.70.1", meta.Version)
	require.Equal(t, "project-1", meta.ProjectId)
	require.Equal(t, srv.URL+"/project/project-1/sessions/th1", meta.ExternalURL)
	page, err := app.PageTraces(ctx, 9, "u1", "th1", &entity.TraceQuery{})
	require.NoError(t, err)
	require.Len(t, page.List, 1)
	require.EqualValues(t, -1, page.Total)
	require.True(t, page.HasMore)
	require.Equal(t, "real-cursor", page.NextCursor)
	require.False(t, *page.Source.Complete)
	query := &entity.TraceQuery{StartTime: page.CoverageStart.Format(time.RFC3339Nano), EndTime: page.CoverageEnd.Format(time.RFC3339Nano)}
	obs, err := app.PageObservations(ctx, 9, "u1", "th1", "t1", query)
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, obs.Source.State)
	require.Len(t, obs.List, 1)
	before := calls
	_, err = app.GetObservationContent(ctx, 9, "u1", "th1", "t1", "root-1", false, query)
	require.Error(t, err)
	require.Equal(t, before, calls, "permission denial must precede any upstream content call")
	content, err := app.GetObservationContent(ctx, 9, "u1", "th1", "t1", "root-1", true, query)
	require.NoError(t, err)
	require.NotContains(t, content.Input, "never-return")
	require.Contains(t, content.Input, "hello")
	require.Equal(t, "world", content.Output)
	_, err = app.GetObservationContent(ctx, 9, "u1", "th1", "t1", "forged-node", true, query)
	require.Error(t, err)
	forged, err := app.PageObservations(ctx, 9, "u1", "th1", "forged-trace", query)
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateAssociationUnverified, forged.Source.State)
	require.Empty(t, forged.List)
	forgedUser, err := app.PageTraces(ctx, 9, "other-user", "th1", query)
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateAssociationUnverified, forgedUser.Source.State)
	require.Empty(t, forgedUser.List)
}

func TestTraceWindowIsBoundedAndCursorRequiresSnapshot(t *testing.T) {
	calls := 0
	srv := traceFixture(t, "4.70.1", func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.NotEmpty(t, r.URL.Query().Get("fromStartTime"))
		require.NotEmpty(t, r.URL.Query().Get("toStartTime"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "meta": map[string]any{"cursor": "next-empty-page"}}))
	})
	app, _ := traceTestApp(t, srv.URL, "project-1", nil)
	page, err := app.PageTraces(context.Background(), 9, "u1", "th1", &entity.TraceQuery{})
	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.Equal(t, entity.SourceStateReady, page.Source.State)
	require.Equal(t, entity.SourceStateNotVerified, page.Association)
	require.Equal(t, 30*24*time.Hour, page.CoverageEnd.Sub(*page.CoverageStart))
	require.NotEmpty(t, page.Source.Message)
	before := calls
	for _, query := range []*entity.TraceQuery{
		{Cursor: "next-empty-page"},
		{StartTime: "invalid date"},
		{StartTime: "2026-11-10T10:00:00Z", EndTime: "2026-11-10T09:00:00Z"},
	} {
		_, err = app.PageTraces(context.Background(), 9, "u1", "th1", query)
		require.Error(t, err)
	}
	require.Equal(t, before, calls, "invalid range is rejected before network I/O")
	_, err = app.PageTraces(context.Background(), 9, "u1", "th1", &entity.TraceQuery{Cursor: page.NextCursor, StartTime: page.CoverageStart.Format(time.RFC3339Nano), EndTime: page.CoverageEnd.Format(time.RFC3339Nano)})
	require.NoError(t, err)
}

func TestSharedLangfuseProjectRejectsUnprovenInstanceAssociation(t *testing.T) {
	srv := traceFixture(t, "4.70.1", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("shared project must not read any session data: %s", r.URL.Path)
	})
	app, rc := traceTestApp(t, srv.URL, "project-1", nil)
	_, err := rc.Save(context.Background(), 10, &dto.ReadConnectionSave{Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, ProjectId: "project-1", PublicKey: "other-pk", Secret: "other-sk"})
	require.NoError(t, err)
	page, err := app.PageTraces(context.Background(), 9, "u1", "th1", &entity.TraceQuery{})
	require.NoError(t, err)
	require.Empty(t, page.List)
	require.Equal(t, entity.SourceStateAssociationUnverified, page.Association)
}

func TestTraceProjectDisplayNameCannotBeUsedAsID(t *testing.T) {
	srv := traceFixture(t, "4.70.1", func(w http.ResponseWriter, r *http.Request) { t.Errorf("project mismatch must fail before reads") })
	app, rc := traceTestApp(t, srv.URL, "Octop 8091", nil)
	meta, err := app.GetThreadTraceMeta(context.Background(), 9, "u1", "th1")
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateAssociationUnverified, meta.Connection.State)
	require.Empty(t, meta.ExternalURL)
	result, err := rc.Test(context.Background(), 9, &dto.ReadConnectionTest{Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, ProjectId: "Octop 8091", PublicKey: "pk", Revision: 1})
	require.NoError(t, err)
	require.False(t, result.Ok)
	require.Equal(t, "project-1", result.ProjectId)
	require.Equal(t, "project", result.Checks[len(result.Checks)-1].Name)
	row, err := rc.connRepo.GetByInstanceType(context.Background(), 9, entity.ReadConnTypeLangfuse)
	require.NoError(t, err)
	require.Equal(t, "Octop 8091", row.ProjectId, "testing does not rewrite saved project settings")
	require.NotEqual(t, entity.VerifyStateReady, row.VerifyState)
}

func TestRequireThreadRejectsDifferentSourceRecordAndInvisibleInstance(t *testing.T) {
	app, _ := traceTestApp(t, "http://127.0.0.1:1", "project-1", nil)
	_, err := app.PageTraces(context.Background(), 9, "u1", "other-thread", nil)
	require.Error(t, err)
	app.instanceApp = &stubInstanceAccess{err: errorx.NewBiz("denied")}
	_, err = app.GetThreadTraceMeta(context.Background(), 9, "u1", "th1")
	require.Error(t, err)
}

func TestSessionURLUsesEscapedRealProjectAndThread(t *testing.T) {
	require.Equal(t, "https://lf.example/base/project/p%2F1/sessions/thread%3Fx=1", buildSessionURL("https://lf.example/base/", "p/1", "thread?x=1"))
	require.Empty(t, buildSessionURL("https://lf.example", "", "thread"))
}

func TestTraceOwnershipFollowsEmptyCursorPage(t *testing.T) {
	created := time.Now().Add(-time.Hour)
	lookupCursors := []string{}
	srv := traceFixture(t, "4.70.1", func(w http.ResponseWriter, r *http.Request) {
		data := []any{appV4Row(created.Add(time.Minute))}
		cursor := ""
		if r.URL.Query().Get("isRootObservation") == "true" && r.URL.Query().Get("traceId") == "t1" {
			lookupCursors = append(lookupCursors, r.URL.Query().Get("cursor"))
			if r.URL.Query().Get("cursor") == "" {
				data = []any{}
				cursor = "ownership-next"
			}
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": map[string]any{"cursor": cursor}}))
	})
	app, _ := traceTestApp(t, srv.URL, "project-1", &created)
	page, err := app.PageObservations(context.Background(), 9, "u1", "th1", "t1", &entity.TraceQuery{})
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, page.Source.State)
	require.Len(t, page.List, 1)
	require.Equal(t, []string{"", "ownership-next"}, lookupCursors)
}

func TestUnprovenPeerProjectCannotBypassInstanceIsolation(t *testing.T) {
	for _, peerProject := range []string{"another-project", "Octop 8091"} {
		t.Run(peerProject, func(t *testing.T) {
			reads := 0
			srv := traceFixture(t, "4.70.1", func(w http.ResponseWriter, r *http.Request) {
				reads++
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []any{appV4Row(time.Now().Add(-time.Minute))}, "meta": map[string]any{}}))
			})
			app, rc := traceTestApp(t, srv.URL, "project-1", nil)
			_, err := rc.Save(context.Background(), 10, &dto.ReadConnectionSave{Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, ProjectId: peerProject, PublicKey: "other-pk", Secret: "other-sk"})
			require.NoError(t, err)
			// A legacy health-only ready flag does not establish a peer's real project.
			_, err = rc.connRepo.UpdateColsByCond(context.Background(), base.NewPatch().Set("verify_state", entity.VerifyStateReady), model.NewCond().Eq("instance_id", 10))
			require.NoError(t, err)
			page, err := app.PageTraces(context.Background(), 9, "u1", "th1", &entity.TraceQuery{})
			require.NoError(t, err)
			require.Equal(t, entity.SourceStateAssociationUnverified, page.Source.State)
			require.Zero(t, reads)
		})
	}
}
