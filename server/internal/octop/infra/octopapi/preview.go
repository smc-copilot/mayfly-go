package octopapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"mayfly-go/internal/octop/domain/business"
	"mayfly-go/internal/octop/domain/entity"
)

var _ business.PreviewClient = (*PreviewClient)(nil)

// 预览 API 路径。
//
// 技能摘要列表已对照 Octop skill_packages.list_package_skills 核验。
// 文件预览路径仍沿用旧适配，响应字段兼容 camelCase / snake_case。
const (
	pathPackageSkills = "/api/skill-packages/%s/skills"
	pathSkillFiles    = "/api/skill-packages/%s/skills/%s/files"
	pathTemplateFiles = "/api/experts/published/%s/files"
	pathFileContent   = "/api/preview/files/%s"
	pathHealth        = "/api/health"
)

const (
	defaultPreviewTimeout = 15 * time.Second
	// MaxPreviewFieldBytes GetFileContent 单字段（Content / Name）最大字节数。
	MaxPreviewFieldBytes = 32 * 1024
	// maxPreviewBodyBytes 单次响应体读取上限，超过视为错误，防止内存被打爆。
	maxPreviewBodyBytes = 4 << 20
	maxListEntries      = 1000
)

// PreviewConfig Octop 预览 API 配置。
type PreviewConfig struct {
	BaseURL    string
	Secret     string        // 作为 Bearer token / API key
	AuthHeader string        // 可选：原样作为 Authorization 头，优先于 Secret
	Timeout    time.Duration // 默认 15s
}

// PreviewHTTPError 非 2xx 响应。不包含响应体，避免泄露内容。
type PreviewHTTPError struct {
	StatusCode int
}

func (e *PreviewHTTPError) Error() string {
	return fmt.Sprintf("octop preview api: http %d", e.StatusCode)
}

// ErrPreviewConfig 预览客户端配置非法。
var ErrPreviewConfig = errors.New("octop preview api: invalid config")

// PreviewClient 实现 business.PreviewClient。
type PreviewClient struct {
	base       string
	authHeader string
	client     *http.Client
	cfgErr     error
}

// NewPreviewClient 创建预览客户端。配置非法时不 panic，而在每次调用时返回 ErrPreviewConfig。
func NewPreviewClient(cfg PreviewConfig) *PreviewClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultPreviewTimeout
	}
	c := &PreviewClient{}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	u, err := url.Parse(base)
	switch {
	case base == "" || err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https"):
		c.cfgErr = fmt.Errorf("%w: base url must be absolute http(s)", ErrPreviewConfig)
	case u.User != nil || u.RawQuery != "" || u.Fragment != "":
		c.cfgErr = fmt.Errorf("%w: base url must not contain userinfo/query/fragment", ErrPreviewConfig)
	}
	c.base = base
	switch {
	case strings.TrimSpace(cfg.AuthHeader) != "":
		c.authHeader = strings.TrimSpace(cfg.AuthHeader)
	case cfg.Secret != "":
		c.authHeader = "Bearer " + cfg.Secret
	}
	c.client = &http.Client{
		Timeout: timeout,
		// 不跟随重定向：防止被引导到 BaseURL 之外的主机并携带凭证。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return c
}

// ListSkills 从 Octop 的文件目录索引读取技能摘要；包内技能并非 PostgreSQL 表。
func (c *PreviewClient) ListSkills(ctx context.Context, packageId string) ([]*entity.BizSkill, error) {
	p, err := buildPath(pathPackageSkills, packageId)
	if err != nil {
		return nil, err
	}
	body, _, err := c.get(ctx, p)
	if err != nil {
		return nil, err
	}
	var skills []*entity.BizSkill
	if err := json.Unmarshal(unwrapData(body), &skills); err != nil {
		return nil, fmt.Errorf("octop preview api: decode skill list: %w", err)
	}
	if skills == nil || len(skills) > maxListEntries {
		return nil, errors.New("octop preview api: invalid or oversized skill list")
	}
	for _, skill := range skills {
		if skill == nil || skill.Slug == "" {
			return nil, errors.New("octop preview api: skill slug is missing")
		}
	}
	return skills, nil
}

// ListSkillFiles 列出技能包内某技能的文件。
func (c *PreviewClient) ListSkillFiles(ctx context.Context, packageId, slug string) ([]entity.BizFileEntry, error) {
	p, err := buildPath(pathSkillFiles, packageId, slug)
	if err != nil {
		return nil, err
	}
	return c.listFiles(ctx, p)
}

// ListTemplateFiles 列出已发布专家模板的文件。
func (c *PreviewClient) ListTemplateFiles(ctx context.Context, templateId string) ([]entity.BizFileEntry, error) {
	p, err := buildPath(pathTemplateFiles, templateId)
	if err != nil {
		return nil, err
	}
	return c.listFiles(ctx, p)
}

// GetFileContent 读取文件内容；Content/Name 各自截断至 MaxPreviewFieldBytes。
func (c *PreviewClient) GetFileContent(ctx context.Context, fileId string) (*entity.BizFileContent, error) {
	p, err := buildPath(pathFileContent, fileId)
	if err != nil {
		return nil, err
	}
	body, ctype, err := c.get(ctx, p)
	if err != nil {
		return nil, err
	}
	var raw struct {
		FileId        string `json:"fileId"`
		FileIdSnake   string `json:"file_id"`
		ID            string `json:"id"`
		Name          string `json:"name"`
		Content       string `json:"content"`
		Truncated     bool   `json:"truncated"`
		MimeType      string `json:"mimeType"`
		MimeTypeSnake string `json:"mime_type"`
	}
	if err := json.Unmarshal(unwrapData(body), &raw); err != nil {
		// 非 JSON 且声明为纯文本：把响应体当作文件正文。
		if !strings.HasPrefix(strings.ToLower(ctype), "text/plain") {
			return nil, fmt.Errorf("octop preview api: decode file content: %w", err)
		}
		raw = struct {
			FileId        string `json:"fileId"`
			FileIdSnake   string `json:"file_id"`
			ID            string `json:"id"`
			Name          string `json:"name"`
			Content       string `json:"content"`
			Truncated     bool   `json:"truncated"`
			MimeType      string `json:"mimeType"`
			MimeTypeSnake string `json:"mime_type"`
		}{Content: string(body)}
	}
	out := &entity.BizFileContent{
		FileId:    firstNonEmpty(raw.FileId, raw.FileIdSnake, raw.ID, fileId),
		Truncated: raw.Truncated,
		MimeType:  firstNonEmpty(raw.MimeType, raw.MimeTypeSnake),
	}
	var t1, t2 bool
	out.Content, t1 = truncateUTF8(raw.Content, MaxPreviewFieldBytes)
	out.Name, t2 = truncateUTF8(raw.Name, MaxPreviewFieldBytes)
	if t1 || t2 {
		out.Truncated = true
	}
	return out, nil
}

// Probe 轻量鉴权探测：GET /api/health（携带 Authorization）。
func (c *PreviewClient) Probe(ctx context.Context) error {
	_, _, err := c.get(ctx, pathHealth)
	return err
}

func (c *PreviewClient) listFiles(ctx context.Context, path string) ([]entity.BizFileEntry, error) {
	body, _, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	type rawEntry struct {
		FileId        string `json:"fileId"`
		FileIdSnake   string `json:"file_id"`
		ID            string `json:"id"`
		Name          string `json:"name"`
		Path          string `json:"path"`
		Size          int64  `json:"size"`
		MimeType      string `json:"mimeType"`
		MimeTypeSnake string `json:"mime_type"`
	}
	var list []rawEntry
	trimmed := unwrapData(body)
	if err := json.Unmarshal(trimmed, &list); err != nil {
		var wrapped struct {
			Files []rawEntry `json:"files"`
			Items []rawEntry `json:"items"`
		}
		if err2 := json.Unmarshal(trimmed, &wrapped); err2 != nil {
			return nil, fmt.Errorf("octop preview api: decode file list: %w", err)
		}
		list = append(wrapped.Files, wrapped.Items...)
	}
	if len(list) > maxListEntries {
		list = list[:maxListEntries]
	}
	out := make([]entity.BizFileEntry, 0, len(list))
	for _, e := range list {
		out = append(out, entity.BizFileEntry{
			FileId:   firstNonEmpty(e.FileId, e.FileIdSnake, e.ID),
			Name:     e.Name,
			Path:     e.Path,
			Size:     e.Size,
			MimeType: firstNonEmpty(e.MimeType, e.MimeTypeSnake),
		})
	}
	return out, nil
}

// get 仅对 BaseURL 下的相对路径发起 GET；不跟随重定向；非 2xx 返回 PreviewHTTPError。
func (c *PreviewClient) get(ctx context.Context, path string) ([]byte, string, error) {
	if c.cfgErr != nil {
		return nil, "", c.cfgErr
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, "", fmt.Errorf("%w: path must be relative", ErrPreviewConfig)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("octop preview api: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("octop preview api: request: %w", sanitizeURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, "", &PreviewHTTPError{StatusCode: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPreviewBodyBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("octop preview api: read body: %w", err)
	}
	if len(body) > maxPreviewBodyBytes {
		return nil, "", errors.New("octop preview api: response too large")
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// sanitizeURLError 去掉 url.Error 中的 URL，保留底层原因。
func sanitizeURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		return ue.Err
	}
	return err
}

// buildPath 对每个路径段 PathEscape，并拒绝空段与 "."/".."。
func buildPath(format string, segs ...string) (string, error) {
	esc := make([]any, len(segs))
	for i, s := range segs {
		if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "\x00\r\n") {
			return "", fmt.Errorf("%w: invalid path segment", ErrPreviewConfig)
		}
		esc[i] = url.PathEscape(s)
	}
	return fmt.Sprintf(format, esc...), nil
}

// unwrapData 兼容 {"data": ...} 包裹。
func unwrapData(body []byte) []byte {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil && len(env.Data) > 0 && string(env.Data) != "null" {
		return env.Data
	}
	return body
}

// truncateUTF8 按字节截断且不切断 UTF-8 字符。
func truncateUTF8(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
