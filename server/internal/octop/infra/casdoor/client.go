package casdoor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiOrganizationApplications = "/api/get-organization-applications"
	apiUsers                    = "/api/get-users"
	// apiUsersLegacy 是旧版 Casdoor 分页路径。本 fork（smc-copilot/casdoor）公开的是 GET /api/get-users?owner=&p=&pageSize=。
	// 仅在 get-users 返回 404/route_missing 时回退，避免把「接口不存在」误判成空目录。
	apiUsersLegacy = "/api/get-pagination-users"
	apiApplication = "/api/get-application"
	maxSyncPages   = 10000
)

// Client 只读 Casdoor 目录客户端。不提供写用户/改授权的方法。
type Client interface {
	TestService(ctx context.Context) error
	TestApps(ctx context.Context) error
	TestUsers(ctx context.Context) error
	ListApplications(ctx context.Context, page, pageSize int) ([]Application, int, error)
	ListUsers(ctx context.Context, page, pageSize int) ([]User, int, error)
	GetApplicationUsers(ctx context.Context, appOwner, appName string) RelationRead
}

type Application struct {
	SourceID     string
	Name         string
	DisplayName  string
	Organization string
	Owner        string
	Enabled      bool
	EnabledKnown bool
	AppType      string
	HomepageURL  string
}

type User struct {
	SourceID     string
	Username     string
	DisplayName  string
	Organization string
	Email        string
	Enabled      bool
	EnabledKnown bool
	UpdatedAt    string
}

// RelationRead 是单次应用关系读取结果。无法核实不得当成「无授权」。
type RelationRead struct {
	UserIDs []string
	Status  string // confirmed / unverified / unsupported
	Failed  bool   // 请求失败（与「字段缺失」区分，用于统计）
	Reason  string
}

type Config struct {
	ServerURL    string
	Organization string
	AppOwner     string // 应用对象 owner，不得默认等于组织名
	ClientID     string
	ClientSecret string
	Timeout      time.Duration
	TLSVerify    bool
}

type httpClient struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !cfg.TLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // 由管理员显式关闭校验
	}
	return &httpClient{
		cfg: cfg,
		client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
	}
}

func (c *httpClient) TestService(ctx context.Context) error {
	endpoint := strings.TrimRight(c.cfg.ServerURL, "/") + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return classifyTransport("/", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return classifyTransport("/", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

func (c *httpClient) TestApps(ctx context.Context) error {
	_, _, err := c.ListApplications(ctx, 1, 1)
	return err
}

func (c *httpClient) TestUsers(ctx context.Context) error {
	_, _, err := c.ListUsers(ctx, 1, 1)
	return err
}

func (c *httpClient) ListApplications(ctx context.Context, page, pageSize int) ([]Application, int, error) {
	q := url.Values{}
	q.Set("organization", c.cfg.Organization)
	q.Set("owner", c.cfg.AppOwner) // 即使为空也显式发送，禁止把组织名填进 owner
	q.Set("p", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	body, err := c.get(ctx, apiOrganizationApplications, q)
	if err != nil {
		return nil, 0, err
	}
	items, total, err := parseList(apiOrganizationApplications, body)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Application, 0, len(items))
	for _, item := range items {
		app := Application{
			SourceID:     appSourceID(item),
			Name:         str(item, "name"),
			DisplayName:  firstNonEmpty(str(item, "displayName"), str(item, "name")),
			Organization: firstNonEmpty(str(item, "organization"), c.cfg.Organization),
			Owner:        str(item, "owner"),
			AppType:      firstNonEmpty(str(item, "type"), str(item, "category")),
			HomepageURL:  firstNonEmpty(str(item, "homepageUrl"), str(item, "url")),
		}
		if enabled, ok := appEnabled(item); ok {
			app.Enabled = enabled
			app.EnabledKnown = true
		}
		out = append(out, app)
	}
	return out, total, nil
}

func (c *httpClient) ListUsers(ctx context.Context, page, pageSize int) ([]User, int, error) {
	q := url.Values{}
	q.Set("owner", c.cfg.Organization)
	q.Set("p", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	body, err := c.get(ctx, apiUsers, q)
	if err != nil && IsKind(err, KindRouteMissing) {
		body, err = c.get(ctx, apiUsersLegacy, q)
	}
	if err != nil {
		return nil, 0, err
	}
	items, total, err := parseList(apiUsers, body)
	if err != nil {
		return nil, 0, err
	}
	out := make([]User, 0, len(items))
	for _, item := range items {
		user := User{
			SourceID:     userSourceID(item),
			Username:     firstNonEmpty(str(item, "name"), str(item, "username")),
			DisplayName:  firstNonEmpty(str(item, "displayName"), str(item, "name")),
			Organization: firstNonEmpty(str(item, "owner"), c.cfg.Organization),
			Email:        str(item, "email"),
			UpdatedAt:    firstNonEmpty(str(item, "updatedTime"), str(item, "updatedAt")),
		}
		if enabled, ok := userEnabled(item); ok {
			user.Enabled = enabled
			user.EnabledKnown = true
		}
		out = append(out, user)
	}
	return out, total, nil
}

// GetApplicationUsers 读取单个应用上可确认的用户列表。
// Casdoor Application 对象本身没有 users 字段；字段缺失视为暂不支持，不得猜测。
func (c *httpClient) GetApplicationUsers(ctx context.Context, appOwner, appName string) RelationRead {
	owner := strings.TrimSpace(appOwner)
	name := strings.TrimSpace(appName)
	if name == "" {
		return RelationRead{Status: "unverified", Reason: "missing application name"}
	}
	if owner == "" {
		return RelationRead{Status: "unverified", Reason: "missing application owner; not substituting organization"}
	}
	q := url.Values{}
	q.Set("id", owner+"/"+name)
	body, err := c.get(ctx, apiApplication, q)
	if err != nil {
		return RelationRead{Status: "unverified", Failed: true, Reason: err.Error()}
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return RelationRead{Status: "unverified", Failed: true, Reason: "unexpected payload shape"}
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return RelationRead{Status: "unverified", Reason: "application payload missing data"}
	}
	raw, ok := data["users"]
	if !ok {
		return RelationRead{Status: "unsupported", Reason: "application has no users field; auto-confirm is not supported"}
	}
	if raw == nil {
		return RelationRead{Status: "unverified", Reason: "users field is null"}
	}
	ids, ok := flattenIDs(raw)
	if !ok {
		return RelationRead{Status: "unverified", Reason: "users field has unexpected shape"}
	}
	return RelationRead{UserIDs: ids, Status: "confirmed"}
}

func (c *httpClient) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	endpoint := strings.TrimRight(c.cfg.ServerURL, "/") + path
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, classifyTransport(path, err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, classifyTransport(path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, classifyTransport(path, err)
	}
	var envelope map[string]any
	casdoorStatus, casdoorMsg := "", ""
	if json.Unmarshal(body, &envelope) == nil {
		casdoorStatus, _ = envelope["status"].(string)
		casdoorMsg = fmt.Sprint(envelope["msg"])
		if casdoorMsg == "<nil>" {
			casdoorMsg = ""
		}
	} else if resp.StatusCode < 300 {
		return nil, &APIError{Kind: KindUnexpectedPayload, API: path, StatusCode: resp.StatusCode, Message: "unexpected payload shape"}
	}
	if resp.StatusCode >= 300 {
		return nil, classifyHTTP(path, resp.StatusCode, casdoorStatus, casdoorMsg)
	}
	if casdoorStatus != "" && casdoorStatus != "ok" {
		return nil, classifyHTTP(path, resp.StatusCode, casdoorStatus, casdoorMsg)
	}
	return body, nil
}

func parseList(api string, body []byte) ([]map[string]any, int, error) {
	var resp struct {
		Status string          `json:"status"`
		Msg    string          `json:"msg"`
		Data   json.RawMessage `json:"data"`
		Data2  json.RawMessage `json:"data2"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, &APIError{Kind: KindUnexpectedPayload, API: api, Message: "unexpected payload shape"}
	}
	var items []map[string]any
	if len(resp.Data) > 0 && string(resp.Data) != "null" {
		if err := json.Unmarshal(resp.Data, &items); err != nil {
			var single map[string]any
			if err2 := json.Unmarshal(resp.Data, &single); err2 == nil {
				items = []map[string]any{single}
			} else {
				return nil, 0, &APIError{Kind: KindUnexpectedPayload, API: api, Message: "unexpected payload shape"}
			}
		}
	}
	total := len(items)
	if len(resp.Data2) > 0 {
		var n int
		if err := json.Unmarshal(resp.Data2, &n); err == nil && n > 0 {
			total = n
		}
	}
	return items, total, nil
}

func appSourceID(item map[string]any) string {
	if id := str(item, "id"); id != "" {
		return id
	}
	owner := str(item, "owner")
	name := str(item, "name")
	if owner != "" && name != "" {
		return owner + "/" + name
	}
	return name
}

func userSourceID(item map[string]any) string {
	if id := str(item, "id"); id != "" {
		return id
	}
	owner := str(item, "owner")
	name := firstNonEmpty(str(item, "name"), str(item, "username"))
	if owner != "" && name != "" {
		return owner + "/" + name
	}
	return name
}

func appEnabled(item map[string]any) (bool, bool) {
	if v, ok := asBool(item, "disableSignin"); ok {
		return !v, true
	}
	if v, ok := asBool(item, "isEnabled"); ok {
		return v, true
	}
	if v, ok := asBool(item, "enable"); ok {
		return v, true
	}
	return false, false
}

func userEnabled(item map[string]any) (bool, bool) {
	forbidden, hasForbidden := asBool(item, "isForbidden")
	deleted, hasDeleted := asBool(item, "isDeleted")
	if !hasForbidden && !hasDeleted {
		return false, false
	}
	return !(forbidden || deleted), true
}

func asBool(m map[string]any, key string) (bool, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func str(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(s)
	}
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func flattenIDs(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch t := item.(type) {
			case string:
				if t != "" {
					out = append(out, t)
				}
			case map[string]any:
				id := firstNonEmpty(str(t, "id"), str(t, "name"))
				if id != "" {
					out = append(out, id)
				}
			default:
				return nil, false
			}
		}
		return out, true
	case []string:
		return v, true
	default:
		return nil, false
	}
}

// ShouldStopPaging 判断分页是否已完整。data2 在部分实现里是页数而非总数，
// 因此只有在 total 不小于 pageSize 时才用它做提前结束条件。
func ShouldStopPaging(got, total, page, pageSize, itemCount int) bool {
	if itemCount == 0 || page >= maxSyncPages {
		return true
	}
	if total >= pageSize && got >= total {
		return true
	}
	// 短页仅在没有「还有未读记录」的总数声明时视为结束，避免 data2 为页数或服务端实际页大小更小时提前停。
	if itemCount < pageSize && (total <= 0 || total <= got) {
		return true
	}
	return false
}
