package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"

	"github.com/stretchr/testify/require"
)

func TestVerifiedPeerProjectRequiresCurrentSuccessfulProtocol(t *testing.T) {
	base := entity.OctopReadConnection{ProjectId: "p2", Revision: 2, VerifyState: entity.VerifyStateReady}
	for _, tc := range []struct {
		name, extra, state string
		valid              bool
	}{
		{"legacy ready", "", "", false},
		{"malformed", "invalid", "", false},
		{"stale revision", `{"langfuseVerification":{"protocol":1,"revision":1,"projectId":"p2"}}`, "", false},
		{"old protocol", `{"langfuseVerification":{"protocol":0,"revision":2,"projectId":"p2"}}`, "", false},
		{"wrong project", `{"langfuseVerification":{"protocol":1,"revision":2,"projectId":"p1"}}`, "", false},
		{"failed current test", `{"langfuseVerification":{"protocol":1,"revision":2,"projectId":"p2"}}`, entity.VerifyStateForbidden, false},
		{"current proof", `{"langfuseVerification":{"protocol":1,"revision":2,"projectId":"p2"}}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			row.ExtraJSON = tc.extra
			if tc.state != "" {
				row.VerifyState = tc.state
			}
			project, ok := verifiedLangfuseProject(&row)
			require.Equal(t, tc.valid, ok)
			if ok {
				require.Equal(t, "p2", project)
			}
		})
	}
}

func TestDistinctVerifiedProjectsAllowReadAndRevisionChangesInvalidateProof(t *testing.T) {
	var denyPeer atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, secret, ok := r.BasicAuth()
		require.True(t, ok)
		require.NotEmpty(t, secret)
		project := "project-1"
		if key == "other-pk" {
			project = "project-2"
		}
		var response any
		switch r.URL.Path {
		case "/api/public/health":
			response = map[string]any{"status": "OK", "version": "4.70.1"}
		case "/api/public/projects":
			if denyPeer.Load() && key == "other-pk" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			response = map[string]any{"data": []any{map[string]any{"id": project, "name": project}}}
		case "/api/public/v2/observations":
			data := []any{}
			if r.URL.Query().Get("sessionId") != "" {
				data = []any{appV4Row(time.Now().Add(-time.Minute))}
			}
			response = map[string]any{"data": data, "meta": map[string]any{}}
		default:
			http.NotFound(w, r)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()
	app, rc := traceTestApp(t, srv.URL, "project-1", nil)
	ctx := context.Background()
	_, err := rc.Save(ctx, 10, &dto.ReadConnectionSave{Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, ProjectId: "project-2", PublicKey: "other-pk", Secret: "other-sk"})
	require.NoError(t, err)
	request := &dto.ReadConnectionTest{Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, ProjectId: "project-2", PublicKey: "other-pk", Revision: 1}
	res, err := rc.Test(ctx, 10, request)
	require.NoError(t, err)
	require.True(t, res.Ok)
	row, err := rc.connRepo.GetByInstanceType(ctx, 10, entity.ReadConnTypeLangfuse)
	require.NoError(t, err)
	project, verified := verifiedLangfuseProject(row)
	require.True(t, verified)
	require.Equal(t, "project-2", project)
	page, err := app.PageTraces(ctx, 9, "u1", "th1", nil)
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, page.Source.State)

	// Even an unchanged Save advances revision and cannot reuse an old proof.
	_, err = rc.Save(ctx, 10, &dto.ReadConnectionSave{Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, ProjectId: "project-2", PublicKey: "other-pk", Revision: 1})
	require.NoError(t, err)
	for _, candidate := range []*dto.ReadConnectionTest{
		request,
		{Type: entity.ReadConnTypeLangfuse, BaseURL: srv.URL, ProjectId: "project-2", PublicKey: "other-pk", Secret: "candidate-sk", Revision: 2},
	} {
		res, err = rc.Test(ctx, 10, candidate)
		require.NoError(t, err)
		require.True(t, res.Ok)
		row, err = rc.connRepo.GetByInstanceType(ctx, 10, entity.ReadConnTypeLangfuse)
		require.NoError(t, err)
		_, verified = verifiedLangfuseProject(row)
		require.False(t, verified, "candidate/stale tests cannot establish the current saved revision")
	}
	page, err = app.PageTraces(ctx, 9, "u1", "th1", nil)
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateAssociationUnverified, page.Source.State)
	request.Revision = 2
	res, err = rc.Test(ctx, 10, request)
	require.NoError(t, err)
	require.True(t, res.Ok)
	denyPeer.Store(true)
	res, err = rc.Test(ctx, 10, request)
	require.NoError(t, err)
	require.False(t, res.Ok)
	row, err = rc.connRepo.GetByInstanceType(ctx, 10, entity.ReadConnTypeLangfuse)
	require.NoError(t, err)
	_, verified = verifiedLangfuseProject(row)
	require.False(t, verified, "current failed authentication invalidates the old success proof")
}
