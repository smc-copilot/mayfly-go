package application

import (
	"encoding/json"
	"net"
	"net/url"
	"path"
	"strings"

	"mayfly-go/internal/octop/domain/entity"
)

func encodeRedirectURIs(uris []string) string {
	clean := make([]string, 0, len(uris))
	seen := map[string]struct{}{}
	for _, u := range uris {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		return ""
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return ""
	}
	return string(b)
}

func DecodeRedirectURIs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return out
	}
	return nil
}

// NormalizedURL 是用于 OIDC 回调对照的规范化结果。只比较 scheme/host/port/path/query，忽略 fragment。
type NormalizedURL struct {
	Raw    string
	Scheme string
	Host   string
	Path   string
	Query  string
	OK     bool
}

func (n NormalizedURL) String() string {
	if !n.OK {
		return ""
	}
	out := n.Scheme + "://" + n.Host
	if n.Path != "" {
		out += n.Path
	}
	if n.Query != "" {
		out += "?" + n.Query
	}
	return out
}

func (n NormalizedURL) IsOriginOnly() bool {
	return n.OK && (n.Path == "" || n.Path == "/") && n.Query == ""
}

// NormalizeOIDCURL 解析并规范化绝对 http(s) URL。相对地址、无主机、非 http(s) 一律失败，避免按名字或子串误匹配。
func NormalizeOIDCURL(raw string) NormalizedURL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return NormalizedURL{Raw: raw}
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return NormalizedURL{Raw: raw}
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return NormalizedURL{Raw: raw}
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return NormalizedURL{Raw: raw}
	}
	if port := u.Port(); port != "" && !isDefaultHTTPPort(scheme, port) {
		host = net.JoinHostPort(host, port)
	}
	p := u.EscapedPath()
	if p == "" {
		p = u.Path
	}
	if decoded, err := url.PathUnescape(p); err == nil {
		p = decoded
	}
	if p == "" {
		p = "/"
	}
	p = path.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p != "/" {
		p = strings.TrimRight(p, "/")
	}
	return NormalizedURL{
		Raw:    raw,
		Scheme: scheme,
		Host:   host,
		Path:   p,
		Query:  u.RawQuery,
		OK:     true,
	}
}

func isDefaultHTTPPort(scheme, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}

// CallbackMatchesRedirect 用实例已配置地址对照 Casdoor redirect URI。
// 实例填 origin（path 为空或 /）时只比 scheme+host+port；填完整回调时比 path，query 仅在实例侧显式给出时参与。
func CallbackMatchesRedirect(callback, redirect NormalizedURL) bool {
	if !callback.OK || !redirect.OK {
		return false
	}
	if callback.Scheme != redirect.Scheme || callback.Host != redirect.Host {
		return false
	}
	if callback.IsOriginOnly() {
		return true
	}
	if callback.Path != redirect.Path {
		return false
	}
	if callback.Query != "" && callback.Query != redirect.Query {
		return false
	}
	return true
}

type instanceURL struct {
	raw   string
	basis string
}

func instanceMatchURLs(inst *entity.OctopInstance) []instanceURL {
	if inst == nil {
		return nil
	}
	out := make([]instanceURL, 0, 2)
	if cb := strings.TrimSpace(inst.CallbackURL); cb != "" {
		out = append(out, instanceURL{raw: cb, basis: entity.MatchBasisCallbackURL})
	}
	if api := strings.TrimSpace(inst.ApiURL); api != "" {
		out = append(out, instanceURL{raw: api, basis: entity.MatchBasisApiURL})
	}
	return out
}

func matchInstanceToRedirects(inst *entity.OctopInstance, redirects []string) (bool, MatchHit) {
	var hit MatchHit
	for _, src := range instanceMatchURLs(inst) {
		cb := NormalizeOIDCURL(src.raw)
		if !cb.OK {
			continue
		}
		for _, raw := range redirects {
			rd := NormalizeOIDCURL(raw)
			if !CallbackMatchesRedirect(cb, rd) {
				continue
			}
			return true, MatchHit{
				Basis:              src.basis,
				CallbackRaw:        src.raw,
				RedirectRaw:        raw,
				NormalizedCallback: cb.String(),
				NormalizedRedirect: rd.String(),
			}
		}
	}
	return false, hit
}

type MatchHit struct {
	Basis              string
	CallbackRaw        string
	RedirectRaw        string
	NormalizedCallback string
	NormalizedRedirect string
}

func classifyBindMatch(redirects []string, candidateCount int) string {
	usable := 0
	for _, raw := range redirects {
		if NormalizeOIDCURL(raw).OK {
			usable++
		}
	}
	if usable == 0 {
		return entity.BindMatchUnverified
	}
	switch candidateCount {
	case 0:
		return entity.BindMatchNone
	case 1:
		return entity.BindMatchUnique
	default:
		return entity.BindMatchAmbiguous
	}
}
