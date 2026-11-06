package octopapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mayfly-go/internal/octop/domain/entity"
)

const (
	usersPath      = "/api/users"
	maxUsersBody   = 8 << 20
	defaultTimeout = 15 * time.Second
)

// UsersResult 是 GET /api/users 的读取结果。Status 区分未配置 / 鉴权失败 / 不可达 / 非预期载荷。
type UsersResult struct {
	Status     string
	Message    string
	StatusCode int
	Rows       []map[string]any
}

// ListUsers 调用 Octop 公开路由 GET {apiUrl}/api/users。
//
// 契约（TencentCloud/Octop src/octop/api/app.py + routers/users.py + deps.py）：
//   - 路径：/api/users（users.router 挂在 /api/users，list_users 为 @router.get("")）
//   - 鉴权：JWT，Authorization: Bearer <token>；依赖 require_permission("users")
//   - 分页：无 query 分页参数，一次返回全部用户（含 disabled）
//   - 过滤：无 username/email query
//   - 响应：JSON 数组；行含 id/username/display_name/email/disabled 等，不含 password_hash
func ListUsers(ctx context.Context, apiURL, token string, timeout time.Duration) UsersResult {
	token = strings.TrimSpace(token)
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if token == "" || base == "" {
		return UsersResult{Status: entity.UserListUnconfigured, Message: "octop api token or api url is not configured"}
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+usersPath, nil)
	if err != nil {
		return UsersResult{Status: entity.UserListUnreachable, Message: sanitizeErr(err.Error(), token)}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return UsersResult{Status: entity.UserListUnreachable, Message: sanitizeErr(err.Error(), token)}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUsersBody+1))
	if err != nil {
		return UsersResult{Status: entity.UserListUnexpected, Message: sanitizeErr(err.Error(), token), StatusCode: resp.StatusCode}
	}
	if len(body) > maxUsersBody {
		return UsersResult{Status: entity.UserListUnexpected, Message: "octop user list payload exceeds limit", StatusCode: resp.StatusCode}
	}
	msg := sanitizeErr(fmt.Sprintf("http %d", resp.StatusCode), token)
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return UsersResult{Status: entity.UserListForbidden, Message: msg, StatusCode: resp.StatusCode}
	case resp.StatusCode >= 500:
		return UsersResult{Status: entity.UserListUnreachable, Message: msg, StatusCode: resp.StatusCode}
	case resp.StatusCode >= 300:
		return UsersResult{Status: entity.UserListUnexpected, Message: msg, StatusCode: resp.StatusCode}
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return UsersResult{Status: entity.UserListUnexpected, Message: "octop GET /api/users returned a non-array payload", StatusCode: resp.StatusCode}
	}
	return UsersResult{Status: entity.UserListOK, StatusCode: resp.StatusCode, Rows: rows}
}

func sanitizeErr(msg, token string) string {
	if token != "" {
		msg = strings.ReplaceAll(msg, token, "***")
	}
	return msg
}
