package entity

import "time"

// BizTrace 一条 Langfuse Trace（可对应业务会话的一轮或多轮中的一段）。
type BizTrace struct {
	TraceId     string     `json:"traceId"`
	Name        string     `json:"name,omitempty"`
	SessionId   string     `json:"sessionId,omitempty"`
	UserId      string     `json:"userId,omitempty"`
	StartTime   *time.Time `json:"startTime,omitempty"`
	EndTime     *time.Time `json:"endTime,omitempty"`
	LatencyMs   *int64     `json:"latencyMs,omitempty"`
	InputTokens *int64     `json:"inputTokens,omitempty"`
	OutputTokens *int64    `json:"outputTokens,omitempty"`
	Level       string     `json:"level,omitempty"` // DEFAULT / WARNING / ERROR 等
	HasError    bool       `json:"hasError"`
	Tags        []string   `json:"tags,omitempty"`
}

// BizObservation Trace 内一个节点。
type BizObservation struct {
	ObservationId string     `json:"observationId"`
	TraceId       string     `json:"traceId"`
	ParentId      string     `json:"parentId,omitempty"`
	Type          string     `json:"type"` // SPAN / GENERATION / TOOL / ...
	Name          string     `json:"name,omitempty"`
	StartTime     *time.Time `json:"startTime,omitempty"`
	EndTime       *time.Time `json:"endTime,omitempty"`
	LatencyMs     *int64     `json:"latencyMs,omitempty"`
	Model         string     `json:"model,omitempty"`
	InputTokens   *int64     `json:"inputTokens,omitempty"`
	OutputTokens  *int64     `json:"outputTokens,omitempty"`
	Level         string     `json:"level,omitempty"`
	StatusMessage string     `json:"statusMessage,omitempty"`
	HasContent    bool       `json:"hasContent"` // 是否可能有输入/输出正文
	Orphan        bool       `json:"orphan"`     // 父节点未取得
}

// BizObservationContent 节点输入/输出正文（瞬时，不落盘）。
type BizObservationContent struct {
	ObservationId string `json:"observationId"`
	Input         string `json:"input,omitempty"`
	Output        string `json:"output,omitempty"`
	Truncated     bool   `json:"truncated"`
}

// TraceQuery Trace/Observation 查询参数。
type TraceQuery struct {
	PageNum   int    `json:"pageNum" form:"pageNum"`
	PageSize  int    `json:"pageSize" form:"pageSize"`
	Cursor    string `json:"cursor" form:"cursor"`
	TraceId   string `json:"traceId" form:"traceId"`
	Type      string `json:"type" form:"type"`
	HasError  *bool  `json:"hasError" form:"hasError"`
	Keyword   string `json:"keyword" form:"keyword"`
	StartTime string `json:"startTime" form:"startTime"`
	EndTime   string `json:"endTime" form:"endTime"`
}

func (q *TraceQuery) Normalize() {
	if q.PageNum <= 0 {
		q.PageNum = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
}

// TracePage 游标/页码混合分页；源端无总数时 Total 为 -1。
type TracePage[T any] struct {
	List       []*T        `json:"list"`
	Total      int64       `json:"total"`
	PageNum    int         `json:"pageNum"`
	PageSize   int         `json:"pageSize"`
	NextCursor string      `json:"nextCursor,omitempty"`
	HasMore    bool        `json:"hasMore"`
	Source     *SourceMeta `json:"source"`
	// Association 会话关联状态：ready / association_unverified / not_found_in_range 等
	Association string `json:"association,omitempty"`
	Mapping     string `json:"mapping,omitempty"` // 已采用的关联映射版本说明
	// CoverageStart/End 本次已加载时间覆盖范围
	CoverageStart *time.Time `json:"coverageStart,omitempty"`
	CoverageEnd   *time.Time `json:"coverageEnd,omitempty"`
	ExternalURL   string     `json:"externalUrl,omitempty"` // 在 Langfuse 中打开（无密钥）
}
