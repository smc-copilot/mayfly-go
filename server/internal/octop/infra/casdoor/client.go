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

// Client 只读 Casdoor 目录客户端。不提供写用户/改授权的方法。
type Client interface {
	TestApps(ctx context.Context) error
	TestUsers(ctx context.Context) error
	ListApplications(ctx context.Context, page, pageSize int) ([]Application, int, error)
	ListUsers(ctx context.Context, page, pageSize int) ([]User, int, error)
	GetApplicationUsers(ctx context.Context, appName string) ([]string, bool, error)
}

type Application struct {
	SourceID     string
	Name         string
	DisplayName  string
	Organization string
	Enabled      bool
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
	UpdatedAt    string
}

type Config struct {
	ServerURL    string
	Organization string
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
	q.Set("owner", c.cfg.Organization)
	q.Set("p", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	body, err := c.get(ctx, "/api/get-organization-applications", q)
	if err != nil {
		return nil, 0, err
	}
	items, total, err := parseList(body)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Application, 0, len(items))
	for _, item := range items {
		out = append(out, Application{
			SourceID:     firstNonEmpty(str(item, "id"), str(item, "name")),
			Name:         str(item, "name"),
			DisplayName:  firstNonEmpty(str(item, "displayName"), str(item, "name")),
			Organization: firstNonEmpty(str(item, "organization"), str(item, "owner"), c.cfg.Organization),
			Enabled:      boolVal(item, "enablePassword") || !boolVal(item, "disabled"),
			AppType:      firstNonEmpty(str(item, "type"), str(item, "category")),
			HomepageURL:  firstNonEmpty(str(item, "homepageUrl"), str(item, "url")),
		})
		if enabled, ok := item["isEnabled"].(bool); ok {
			out[len(out)-1].Enabled = enabled
		}
		if enabled, ok := item["enable"].(bool); ok {
			out[len(out)-1].Enabled = enabled
		}
	}
	return out, total, nil
}

func (c *httpClient) ListUsers(ctx context.Context, page, pageSize int) ([]User, int, error) {
	q := url.Values{}
	q.Set("owner", c.cfg.Organization)
	q.Set("p", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	body, err := c.get(ctx, "/api/get-pagination-users", q)
	if err != nil {
		return nil, 0, err
	}
	items, total, err := parseList(body)
	if err != nil {
		return nil, 0, err
	}
	out := make([]User, 0, len(items))
	for _, item := range items {
		out = append(out, User{
			SourceID:     firstNonEmpty(str(item, "id"), str(item, "name")),
			Username:     firstNonEmpty(str(item, "name"), str(item, "username")),
			DisplayName:  firstNonEmpty(str(item, "displayName"), str(item, "name")),
			Organization: firstNonEmpty(str(item, "owner"), c.cfg.Organization),
			Email:        str(item, "email"),
			Enabled:      !boolVal(item, "isForbidden") && !boolVal(item, "isDeleted"),
			UpdatedAt:    firstNonEmpty(str(item, "updatedTime"), str(item, "updatedAt")),
		})
	}
	return out, total, nil
}

// GetApplicationUsers 尝试读取应用上可确认的用户列表。读不到时 ok=false，调用方标「未核实」。
func (c *httpClient) GetApplicationUsers(ctx context.Context, appName string) ([]string, bool, error) {
	q := url.Values{}
	q.Set("id", c.cfg.Organization+"/"+appName)
	body, err := c.get(ctx, "/api/get-application", q)
	if err != nil {
		return nil, false, nil // 无权限或接口不可用：未核实，不当成失败清空
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, false, nil
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return nil, false, nil
	}
	raw, ok := data["users"]
	if !ok {
		return nil, false, nil
	}
	ids := flattenIDs(raw)
	return ids, true, nil
}

func (c *httpClient) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	endpoint := strings.TrimRight(c.cfg.ServerURL, "/") + path
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("casdoor http %d", resp.StatusCode)
	}
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err == nil {
		if status, ok := envelope["status"].(string); ok && status != "" && status != "ok" {
			return nil, fmt.Errorf("casdoor status %s: %v", status, envelope["msg"])
		}
	}
	return body, nil
}

func parseList(body []byte) ([]map[string]any, int, error) {
	var resp struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
		Data   json.RawMessage `json:"data"`
		Data2  json.RawMessage `json:"data2"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, err
	}
	var items []map[string]any
	if len(resp.Data) > 0 && string(resp.Data) != "null" {
		if err := json.Unmarshal(resp.Data, &items); err != nil {
			var single map[string]any
			if err2 := json.Unmarshal(resp.Data, &single); err2 == nil {
				items = []map[string]any{single}
			} else {
				return nil, 0, err
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

func boolVal(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func flattenIDs(raw any) []string {
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
			}
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}
