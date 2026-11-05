package casdoor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"syscall"
)

// 错误分类，对应 PRD 5.6。Error() 含接口名与 HTTP 状态，不含密钥或 Authorization。
const (
	KindUnreachable       = "unreachable"
	KindTimeout           = "timeout"
	KindForbidden         = "forbidden"
	KindInvalidOrg        = "invalid_org"
	KindBadAppOwner       = "bad_app_owner"
	KindRouteMissing      = "route_missing"
	KindUnexpectedPayload = "unexpected_payload"
	KindHTTP              = "http"
)

// APIError 是脱敏后的 Casdoor 调用错误。
type APIError struct {
	Kind       string
	API        string
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	api := strings.TrimSpace(e.API)
	msg := sanitizeErrText(e.Message)
	switch {
	case e.StatusCode > 0 && msg != "":
		return fmt.Sprintf("casdoor GET %s: http %d (%s): %s", api, e.StatusCode, e.Kind, msg)
	case e.StatusCode > 0:
		return fmt.Sprintf("casdoor GET %s: http %d (%s)", api, e.StatusCode, e.Kind)
	case msg != "":
		return fmt.Sprintf("casdoor GET %s: %s: %s", api, e.Kind, msg)
	default:
		return fmt.Sprintf("casdoor GET %s: %s", api, e.Kind)
	}
}

func (e *APIError) Is(target error) bool {
	other, ok := target.(*APIError)
	if !ok || e == nil || other == nil {
		return false
	}
	if other.Kind != "" && e.Kind != other.Kind {
		return false
	}
	if other.API != "" && e.API != other.API {
		return false
	}
	return true
}

func IsKind(err error, kind string) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Kind == kind
	}
	return false
}

func classifyTransport(api string, err error) error {
	if err == nil {
		return nil
	}
	kind := KindUnreachable
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		kind = KindTimeout
	}
	return &APIError{Kind: kind, API: api, Message: sanitizeErrText(err.Error())}
}

func classifyHTTP(api string, status int, casdoorStatus, casdoorMsg string) error {
	kind := KindHTTP
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		kind = KindForbidden
	case http.StatusNotFound:
		kind = KindRouteMissing
	}
	msg := strings.TrimSpace(casdoorMsg)
	if kind == KindHTTP {
		if inferred := inferKindFromMsg(casdoorStatus, msg); inferred != "" {
			kind = inferred
		}
	}
	if kind == KindHTTP && (status == 0 || status == http.StatusOK) {
		if inferred := inferKindFromMsg(casdoorStatus, msg); inferred != "" {
			kind = inferred
		}
	}
	if status == http.StatusOK && casdoorStatus != "" && casdoorStatus != "ok" {
		if inferred := inferKindFromMsg(casdoorStatus, msg); inferred != "" {
			kind = inferred
		} else if kind == KindHTTP {
			kind = inferKindFromMsg(casdoorStatus, msg)
			if kind == "" {
				kind = KindHTTP
			}
		}
	}
	return &APIError{Kind: kind, API: api, StatusCode: status, Message: sanitizeErrText(firstNonEmpty(msg, casdoorStatus))}
}

func inferKindFromMsg(status, msg string) string {
	text := strings.ToLower(status + " " + msg)
	switch {
	case strings.Contains(text, "missing parameter") && strings.Contains(text, "organization"):
		return KindInvalidOrg
	case strings.Contains(text, "organization") && (strings.Contains(text, "not exist") || strings.Contains(text, "invalid") || strings.Contains(text, "doesn't exist") || strings.Contains(text, "does not exist")):
		return KindInvalidOrg
	case strings.Contains(text, "missing parameter") && strings.Contains(text, "owner"):
		return KindBadAppOwner
	case strings.Contains(text, "unauthorized") || strings.Contains(text, "permission") || strings.Contains(text, "forbidden") || strings.Contains(text, "access denied"):
		return KindForbidden
	case strings.Contains(text, "not found") || strings.Contains(text, "404"):
		return KindRouteMissing
	default:
		return ""
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, syscall.ETIMEDOUT)
}

var (
	secretKV   = regexp.MustCompile(`(?i)(client_secret|clientsecret|secret|password|passwd|authorization|token|access_token)=([^&\s"'\\]+)`)
	basicAuth  = regexp.MustCompile(`(?i)(basic\s+)[a-z0-9+/=_-]+`)
	bearerAuth = regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+`)
	userinfo   = regexp.MustCompile(`://[^/@\s]+:[^/@\s]+@`)
)

func sanitizeErrText(s string) string {
	if s == "" {
		return ""
	}
	s = secretKV.ReplaceAllString(s, "${1}=***")
	s = basicAuth.ReplaceAllString(s, "${1}***")
	s = bearerAuth.ReplaceAllString(s, "${1}***")
	s = userinfo.ReplaceAllString(s, "://***:***@")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if len(s) > 240 {
		return s[:240]
	}
	return s
}
