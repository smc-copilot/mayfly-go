package application

import (
	"testing"

	"mayfly-go/internal/octop/domain/entity"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOIDCURL(t *testing.T) {
	ok := NormalizeOIDCURL("https://Octop.Example.com:443/api/auth/oidc/callback/")
	require.True(t, ok.OK)
	require.Equal(t, "https", ok.Scheme)
	require.Equal(t, "octop.example.com", ok.Host)
	require.Equal(t, "/api/auth/oidc/callback", ok.Path)
	require.Equal(t, "https://octop.example.com/api/auth/oidc/callback", ok.String())

	withPort := NormalizeOIDCURL("http://10.0.0.8:8080")
	require.True(t, withPort.OK)
	require.Equal(t, "10.0.0.8:8080", withPort.Host)
	require.True(t, withPort.IsOriginOnly())

	require.False(t, NormalizeOIDCURL("octop-prod").OK)
	require.False(t, NormalizeOIDCURL("javascript:alert(1)").OK)
	require.False(t, NormalizeOIDCURL("/api/auth/oidc/callback").OK)
	require.False(t, NormalizeOIDCURL("").OK)
}

func TestCallbackMatchesRedirectExactAndOrigin(t *testing.T) {
	redirect := NormalizeOIDCURL("https://octop.example.com/api/auth/oidc/callback")
	origin := NormalizeOIDCURL("https://octop.example.com")
	full := NormalizeOIDCURL("https://octop.example.com/api/auth/oidc/callback")
	internal := NormalizeOIDCURL("http://10.0.0.8:8080")
	evilSub := NormalizeOIDCURL("https://evil-octop.example.com")
	evilSuffix := NormalizeOIDCURL("https://octop.example.com.evil.net")
	otherPath := NormalizeOIDCURL("https://octop.example.com/other")

	require.True(t, CallbackMatchesRedirect(origin, redirect))
	require.True(t, CallbackMatchesRedirect(full, redirect))
	require.False(t, CallbackMatchesRedirect(internal, redirect))
	require.False(t, CallbackMatchesRedirect(evilSub, redirect))
	require.False(t, CallbackMatchesRedirect(evilSuffix, redirect))
	require.False(t, CallbackMatchesRedirect(otherPath, redirect))
}

func TestMatchInstancePrefersCallbackURL(t *testing.T) {
	redirects := []string{"https://octop.example.com/api/auth/oidc/callback"}
	inst := &entity.OctopInstance{
		ApiURL:      "http://10.0.0.8:8080",
		CallbackURL: "https://octop.example.com/api/auth/oidc/callback",
	}
	ok, hit := matchInstanceToRedirects(inst, redirects)
	require.True(t, ok)
	require.Equal(t, entity.MatchBasisCallbackURL, hit.Basis)
	require.Equal(t, "https://octop.example.com/api/auth/oidc/callback", hit.NormalizedRedirect)

	internalOnly := &entity.OctopInstance{ApiURL: "http://10.0.0.8:8080"}
	ok, _ = matchInstanceToRedirects(internalOnly, redirects)
	require.False(t, ok)

	sameOriginAPI := &entity.OctopInstance{ApiURL: "https://octop.example.com"}
	ok, hit = matchInstanceToRedirects(sameOriginAPI, redirects)
	require.True(t, ok)
	require.Equal(t, entity.MatchBasisApiURL, hit.Basis)
}

func TestClassifyBindMatch(t *testing.T) {
	require.Equal(t, entity.BindMatchUnverified, classifyBindMatch(nil, 0))
	require.Equal(t, entity.BindMatchUnverified, classifyBindMatch([]string{"not-a-url"}, 1))
	require.Equal(t, entity.BindMatchNone, classifyBindMatch([]string{"https://octop.example.com/cb"}, 0))
	require.Equal(t, entity.BindMatchUnique, classifyBindMatch([]string{"https://octop.example.com/cb"}, 1))
	require.Equal(t, entity.BindMatchAmbiguous, classifyBindMatch([]string{"https://a.example/cb", "https://b.example/cb"}, 2))
}

func TestNameOrSubstringDoesNotMatch(t *testing.T) {
	inst := &entity.OctopInstance{Name: "octop-prod", Code: "octop.example.com"}
	ok, _ := matchInstanceToRedirects(inst, []string{"https://octop.example.com/api/auth/oidc/callback"})
	require.False(t, ok)
}
