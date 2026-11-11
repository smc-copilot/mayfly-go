package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mayfly-go/internal/octop/domain/entity"
)

// Config Langfuse Public API 读取配置。
type Config struct {
	BaseURL   string
	PublicKey string
	SecretKey string
	ProjectId string
	Timeout   time.Duration
}

const (
	defaultTimeout       = 15 * time.Second
	MaxContentFieldBytes = 32 * 1024
	maxBodyBytes         = 4 << 20
)

var (
	ErrConfig      = errors.New("langfuse: invalid config")
	ErrHTTP        = errors.New("langfuse: http error")
	ErrUnsupported = errors.New("langfuse: unsupported")
	ErrForbidden   = errors.New("langfuse: forbidden")
	ErrProject     = errors.New("langfuse: project mismatch")
	ErrAssociation = errors.New("langfuse: association unverified")
	ErrQuery       = errors.New("langfuse: invalid query")
)

// HTTPError 非 2xx。
type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("langfuse: http %d", e.StatusCode)
}

func (e *HTTPError) Is(target error) bool {
	return target == ErrHTTP
}

// Client selects the verified v2/v3 classic or v4 Observation read protocol.
type Client struct {
	base            string
	auth            string
	client          *http.Client
	cfgErr          error
	expectedProject string
	verifyMu        sync.Mutex
	verified        *Verification
}

func NewClient(cfg Config) *Client {
	c := &Client{expectedProject: strings.TrimSpace(cfg.ProjectId)}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	u, err := url.Parse(base)
	switch {
	case base == "" || err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https"):
		c.cfgErr = fmt.Errorf("%w: base url", ErrConfig)
	case strings.TrimSpace(cfg.PublicKey) == "" || strings.TrimSpace(cfg.SecretKey) == "":
		c.cfgErr = fmt.Errorf("%w: public/secret key required", ErrConfig)
	default:
		c.base = base
		token := base64.StdEncoding.EncodeToString([]byte(cfg.PublicKey + ":" + cfg.SecretKey))
		c.auth = "Basic " + token
		c.client = &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return c
}

// Probe verifies health, authentication, project and read capability.
func (c *Client) Probe(ctx context.Context) (version string, err error) {
	info, err := c.Verify(ctx)
	if err != nil {
		return "", err
	}
	return info.Version, nil
}

// ListTracesQuery 追踪列表查询。
type ListTracesQuery struct {
	SessionId string
	UserId    string
	From      *time.Time
	To        *time.Time
	Page      int
	Limit     int
	Cursor    string
	TraceId   string
}

type ListTracesResult struct {
	Items      []*entity.BizTrace
	Total      int64
	Page       int
	Limit      int
	HasMore    bool
	NextCursor string
}

func (c *Client) ListTraces(ctx context.Context, q ListTracesQuery) (*ListTracesResult, error) {
	info, err := c.Verify(ctx)
	if err != nil {
		return nil, err
	}
	if info.APIMode == ModeV4 {
		return c.listV4Traces(ctx, q, info.ProjectId)
	}
	return c.listClassicTraces(ctx, q)
}

func (c *Client) listClassicTraces(ctx context.Context, q ListTracesQuery) (*ListTracesResult, error) {
	if c.cfgErr != nil {
		return nil, c.cfgErr
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	if q.SessionId != "" {
		params.Set("sessionId", q.SessionId)
	}
	if q.UserId != "" {
		params.Set("userId", q.UserId)
	}
	if q.From != nil {
		params.Set("fromTimestamp", q.From.UTC().Format(time.RFC3339Nano))
	}
	if q.To != nil {
		params.Set("toTimestamp", q.To.UTC().Format(time.RFC3339Nano))
	}
	path := "/api/public/traces?" + params.Encode()
	if q.TraceId != "" {
		path = "/api/public/traces/" + url.PathEscape(q.TraceId)
	}
	body, status, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := mapStatus(status); err != nil {
		return nil, err
	}
	if q.TraceId != "" {
		var raw rawTrace
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("%w: trace response", ErrUnsupported)
		}
		if raw.ID != q.TraceId || raw.SessionID != q.SessionId || raw.UserID != q.UserId {
			return nil, ErrAssociation
		}
		return &ListTracesResult{Items: []*entity.BizTrace{raw.toBiz()}, Total: 1, Page: 1, Limit: 1}, nil
	}
	var resp struct {
		Data []rawTrace `json:"data"`
		Meta struct {
			Page       int   `json:"page"`
			Limit      int   `json:"limit"`
			TotalItems int64 `json:"totalItems"`
			TotalPages int   `json:"totalPages"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("langfuse: decode traces: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("%w: traces envelope", ErrUnsupported)
	}
	items := make([]*entity.BizTrace, 0, len(resp.Data))
	for i := range resp.Data {
		items = append(items, resp.Data[i].toBiz())
	}
	hasMore := false
	if resp.Meta.TotalPages > 0 {
		hasMore = page < resp.Meta.TotalPages
	} else {
		hasMore = len(items) >= limit
	}
	return &ListTracesResult{
		Items: items, Total: resp.Meta.TotalItems, Page: page, Limit: limit, HasMore: hasMore,
	}, nil
}

type ListObservationsQuery struct {
	TraceId   string
	SessionId string
	UserId    string
	From      *time.Time
	To        *time.Time
	Cursor    string
	Page      int
	Limit     int
}

type ListObservationsResult struct {
	Items      []*entity.BizObservation
	Total      int64
	Page       int
	Limit      int
	HasMore    bool
	NextCursor string
}

func (c *Client) ListObservations(ctx context.Context, q ListObservationsQuery) (*ListObservationsResult, error) {
	info, err := c.Verify(ctx)
	if err != nil {
		return nil, err
	}
	if info.APIMode == ModeV4 {
		return c.listV4Observations(ctx, q, info.ProjectId)
	}
	res, err := c.listClassicObservations(ctx, q)
	if err != nil {
		return nil, err
	}
	for _, item := range res.Items {
		if item.ObservationId == "" || item.TraceId != q.TraceId {
			return nil, ErrAssociation
		}
	}
	return res, nil
}

func (c *Client) listClassicObservations(ctx context.Context, q ListObservationsQuery) (*ListObservationsResult, error) {
	if c.cfgErr != nil {
		return nil, c.cfgErr
	}
	if strings.TrimSpace(q.TraceId) == "" {
		return nil, fmt.Errorf("%w: traceId required", ErrConfig)
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("traceId", q.TraceId)
	body, status, err := c.do(ctx, http.MethodGet, "/api/public/observations?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if err := mapStatus(status); err != nil {
		return nil, err
	}
	var resp struct {
		Data []rawObservation `json:"data"`
		Meta struct {
			Page       int   `json:"page"`
			Limit      int   `json:"limit"`
			TotalItems int64 `json:"totalItems"`
			TotalPages int   `json:"totalPages"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("langfuse: decode observations: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("%w: observations envelope", ErrUnsupported)
	}
	items := make([]*entity.BizObservation, 0, len(resp.Data))
	for i := range resp.Data {
		items = append(items, resp.Data[i].toBiz())
	}
	hasMore := false
	if resp.Meta.TotalPages > 0 {
		hasMore = page < resp.Meta.TotalPages
	} else {
		hasMore = len(items) >= limit
	}
	return &ListObservationsResult{
		Items: items, Total: resp.Meta.TotalItems, Page: page, Limit: limit, HasMore: hasMore,
	}, nil
}

// GetObservation 取单条 observation（含 input/output）。
func (c *Client) GetObservation(ctx context.Context, observationId string) (*entity.BizObservation, *entity.BizObservationContent, error) {
	info, err := c.Verify(ctx)
	if err != nil {
		return nil, nil, err
	}
	if info.APIMode == ModeV4 {
		return nil, nil, ErrQuery
	}
	return c.getClassicObservation(ctx, observationId)
}

func (c *Client) getClassicObservation(ctx context.Context, observationId string) (*entity.BizObservation, *entity.BizObservationContent, error) {
	if c.cfgErr != nil {
		return nil, nil, c.cfgErr
	}
	id := strings.TrimSpace(observationId)
	if id == "" {
		return nil, nil, fmt.Errorf("%w: observationId required", ErrConfig)
	}
	body, status, err := c.do(ctx, http.MethodGet, "/api/public/observations/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, nil, err
	}
	if err := mapStatus(status); err != nil {
		return nil, nil, err
	}
	var raw rawObservation
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, nil, fmt.Errorf("%w: observation response", ErrUnsupported)
	}
	if raw.ID == "" {
		// Some classic versions wrap the item in data; absent IDs never pass.
		var wrap struct {
			Data rawObservation `json:"data"`
		}
		if err := json.Unmarshal(body, &wrap); err != nil {
			return nil, nil, fmt.Errorf("%w: observation envelope", ErrUnsupported)
		}
		raw = wrap.Data
	}
	if raw.ID != id || raw.TraceID == "" {
		return nil, nil, ErrAssociation
	}
	obs := raw.toBiz()
	inStr, inTrunc := stringifyTruncate(redactContent(raw.Input), MaxContentFieldBytes)
	outStr, outTrunc := stringifyTruncate(redactContent(raw.Output), MaxContentFieldBytes)
	content := &entity.BizObservationContent{
		ObservationId: obs.ObservationId,
		Input:         inStr,
		Output:        outStr,
		Truncated:     inTrunc || outTrunc,
	}
	return obs, content, nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(data) > maxBodyBytes {
		return nil, resp.StatusCode, fmt.Errorf("langfuse: response too large")
	}
	return data, resp.StatusCode, nil
}

func mapStatus(status int) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("%w: %d", ErrForbidden, status)
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: endpoint not found", ErrUnsupported)
	default:
		return &HTTPError{StatusCode: status}
	}
}

type rawTrace struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	SessionID string          `json:"sessionId"`
	UserID    string          `json:"userId"`
	Timestamp *time.Time      `json:"timestamp"`
	StartTime *time.Time      `json:"startTime"`
	EndTime   *time.Time      `json:"endTime"`
	Latency   *float64        `json:"latency"` // seconds
	Tags      []string        `json:"tags"`
	Level     string          `json:"level"`
	Metadata  json.RawMessage `json:"metadata"`
	Usage     *struct {
		Input  *int64 `json:"input"`
		Output *int64 `json:"output"`
		Total  *int64 `json:"total"`
	} `json:"usage"`
}

func (r rawTrace) toBiz() *entity.BizTrace {
	start := r.StartTime
	if start == nil {
		start = r.Timestamp
	}
	t := &entity.BizTrace{
		TraceId:   r.ID,
		Name:      r.Name,
		SessionId: r.SessionID,
		UserId:    stringifyAny(r.UserID),
		StartTime: start,
		EndTime:   r.EndTime,
		Level:     r.Level,
		Tags:      r.Tags,
		HasError:  strings.EqualFold(r.Level, "ERROR") || strings.EqualFold(r.Level, "WARNING"),
	}
	if r.Latency != nil {
		ms := int64(*r.Latency * 1000)
		t.LatencyMs = &ms
	} else if start != nil && r.EndTime != nil {
		ms := r.EndTime.Sub(*start).Milliseconds()
		t.LatencyMs = &ms
	}
	if r.Usage != nil {
		t.InputTokens = r.Usage.Input
		t.OutputTokens = r.Usage.Output
	}
	return t
}

type rawObservation struct {
	ID                  string          `json:"id"`
	TraceID             string          `json:"traceId"`
	ParentObservationID *string         `json:"parentObservationId"`
	Type                string          `json:"type"`
	Name                string          `json:"name"`
	StartTime           *time.Time      `json:"startTime"`
	EndTime             *time.Time      `json:"endTime"`
	CompletionStartTime *time.Time      `json:"completionStartTime"`
	Model               string          `json:"model"`
	Level               string          `json:"level"`
	StatusMessage       string          `json:"statusMessage"`
	Input               json.RawMessage `json:"input"`
	Output              json.RawMessage `json:"output"`
	Usage               *struct {
		Input  *int64 `json:"input"`
		Output *int64 `json:"output"`
	} `json:"usage"`
	CalculatedInputUsage  *int64 `json:"calculatedInputUsage"`
	CalculatedOutputUsage *int64 `json:"calculatedOutputUsage"`
}

func (r rawObservation) toBiz() *entity.BizObservation {
	o := &entity.BizObservation{
		ObservationId: r.ID,
		TraceId:       r.TraceID,
		Type:          r.Type,
		Name:          r.Name,
		StartTime:     r.StartTime,
		EndTime:       r.EndTime,
		Model:         r.Model,
		Level:         r.Level,
		StatusMessage: r.StatusMessage,
		HasContent:    len(r.Input) > 0 || len(r.Output) > 0,
	}
	if r.ParentObservationID != nil {
		o.ParentId = *r.ParentObservationID
	}
	if r.StartTime != nil && r.EndTime != nil {
		ms := r.EndTime.Sub(*r.StartTime).Milliseconds()
		o.LatencyMs = &ms
	}
	if r.Usage != nil {
		o.InputTokens = r.Usage.Input
		o.OutputTokens = r.Usage.Output
	}
	if o.InputTokens == nil {
		o.InputTokens = r.CalculatedInputUsage
	}
	if o.OutputTokens == nil {
		o.OutputTokens = r.CalculatedOutputUsage
	}
	return o
}

func stringifyAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func stringifyTruncate(raw json.RawMessage, max int) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		s := string(raw)
		return truncateRunes(s, max)
	}
	if s, ok := v.(string); ok {
		return truncateRunes(s, max)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return truncateRunes(string(raw), max)
	}
	return truncateRunes(string(b), max)
}

func truncateRunes(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	for maxBytes > 0 && !utf8.ValidString(s[:maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes], true
}
