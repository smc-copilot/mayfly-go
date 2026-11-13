package octopapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type seen struct {
	path string
	raw  string
	auth string
}

func TestListSkillsRejectsMalformedSummaries(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `[{"name":"no slug"}]`, `[null]`} {
		t.Run(body, func(t *testing.T) {
			srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write([]byte(body)); err != nil {
					t.Error(err)
				}
			})
			_, err := NewPreviewClient(PreviewConfig{BaseURL: srv.URL}).ListSkills(context.Background(), "p1")
			require.Error(t, err)
		})
	}
}

func newServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]seen) {
	t.Helper()
	var reqs []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs = append(reqs, seen{path: r.URL.Path, raw: r.URL.EscapedPath(), auth: r.Header.Get("Authorization")})
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

func TestListSkillFilesPathAndAuth(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"files":[{"file_id":"f1","name":"SKILL.md","path":"SKILL.md","size":12,"mime_type":"text/markdown"},{"fileId":"f2","name":"a.py"}]}`))
	})
	c := NewPreviewClient(PreviewConfig{BaseURL: srv.URL + "/", Secret: "s3cret"})
	files, err := c.ListSkillFiles(context.Background(), "pkg 1", "my-skill")
	require.NoError(t, err)
	require.Len(t, files, 2)
	require.Equal(t, "f1", files[0].FileId)
	require.Equal(t, "text/markdown", files[0].MimeType)
	require.Equal(t, int64(12), files[0].Size)
	require.Equal(t, "f2", files[1].FileId)

	require.Len(t, *reqs, 1)
	require.Equal(t, "/api/skill-packages/pkg%201/skills/my-skill/files", (*reqs)[0].raw)
	require.Equal(t, "Bearer s3cret", (*reqs)[0].auth)
}

func TestListTemplateFilesArrayResponse(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"x1","name":"prompt.md"}]`))
	})
	c := NewPreviewClient(PreviewConfig{BaseURL: srv.URL, AuthHeader: "ApiKey abc", Secret: "ignored"})
	files, err := c.ListTemplateFiles(context.Background(), "tpl-9")
	require.NoError(t, err)
	require.Equal(t, "x1", files[0].FileId)
	require.Equal(t, "/api/experts/published/tpl-9/files", (*reqs)[0].path)
	require.Equal(t, "ApiKey abc", (*reqs)[0].auth, "AuthHeader overrides Bearer secret")
}

func TestGetFileContentTruncates(t *testing.T) {
	big := strings.Repeat("a", MaxPreviewFieldBytes+500)
	srv, reqs := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"f1","name":"big.txt","content":"` + big + `","mime_type":"text/plain"}}`))
	})
	c := NewPreviewClient(PreviewConfig{BaseURL: srv.URL, Secret: "k"})
	got, err := c.GetFileContent(context.Background(), "f1")
	require.NoError(t, err)
	require.Equal(t, "/api/preview/files/f1", (*reqs)[0].path)
	require.Len(t, got.Content, MaxPreviewFieldBytes)
	require.True(t, got.Truncated)
	require.Equal(t, "big.txt", got.Name)
	require.Equal(t, "f1", got.FileId)
}

func TestGetFileContentSmallNotTruncated(t *testing.T) {
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"fileId":"f2","name":"n","content":"hello","mimeType":"text/plain"}`))
	})
	got, err := NewPreviewClient(PreviewConfig{BaseURL: srv.URL}).GetFileContent(context.Background(), "f2")
	require.NoError(t, err)
	require.Equal(t, "hello", got.Content)
	require.False(t, got.Truncated)
}

func TestTruncateUTF8Boundary(t *testing.T) {
	s := strings.Repeat("中", 20) // 60 bytes
	out, cut := truncateUTF8(s, 31)
	require.True(t, cut)
	require.Equal(t, 30, len(out))
	require.Equal(t, strings.Repeat("中", 10), out)
}

func TestProbe(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	require.NoError(t, NewPreviewClient(PreviewConfig{BaseURL: srv.URL, Secret: "ok"}).Probe(context.Background()))
	require.Equal(t, "/api/health", (*reqs)[0].path)

	err := NewPreviewClient(PreviewConfig{BaseURL: srv.URL, Secret: "bad"}).Probe(context.Background())
	var he *PreviewHTTPError
	require.True(t, errors.As(err, &he))
	require.Equal(t, http.StatusUnauthorized, he.StatusCode)
	require.NotContains(t, err.Error(), "bad")
}

func TestDoesNotFollowRedirects(t *testing.T) {
	evil, evilReqs := newServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/steal", http.StatusFound)
	})
	_, err := NewPreviewClient(PreviewConfig{BaseURL: srv.URL, Secret: "k"}).ListTemplateFiles(context.Background(), "t")
	var he *PreviewHTTPError
	require.True(t, errors.As(err, &he))
	require.Equal(t, http.StatusFound, he.StatusCode)
	require.Empty(t, *evilReqs)
}

func TestRejectsUnsafeSegmentsAndBadConfig(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	c := NewPreviewClient(PreviewConfig{BaseURL: srv.URL})
	for _, bad := range []string{"", ".", ".."} {
		_, err := c.ListTemplateFiles(context.Background(), bad)
		require.ErrorIs(t, err, ErrPreviewConfig, bad)
	}
	// "/" 被转义为 %2F，不会形成额外路径段
	_, err := c.GetFileContent(context.Background(), "a/../b")
	require.Error(t, err) // 空 JSON 数组无法解码为对象，但请求路径必须保持单段
	require.Equal(t, "/api/preview/files/a%2F..%2Fb", (*reqs)[len(*reqs)-1].raw)

	for _, base := range []string{"", "not a url", "/relative", "ftp://x", "http://u:p@host"} {
		err := NewPreviewClient(PreviewConfig{BaseURL: base}).Probe(context.Background())
		require.ErrorIs(t, err, ErrPreviewConfig, base)
	}
}

func TestTimeoutDefaultsAndApplies(t *testing.T) {
	require.Equal(t, 15*time.Second, NewPreviewClient(PreviewConfig{BaseURL: "http://x"}).client.Timeout)

	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) { time.Sleep(300 * time.Millisecond) })
	err := NewPreviewClient(PreviewConfig{BaseURL: srv.URL, Timeout: 50 * time.Millisecond}).Probe(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), srv.URL)
}
