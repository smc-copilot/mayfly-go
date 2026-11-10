package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

// 业务读取来源状态（与 PRD §9 一致）。
const (
	SourceStateReady               = "ready"
	SourceStateNotConfigured       = "not_configured"
	SourceStateNotVerified         = "not_verified"
	SourceStateUnsupported         = "unsupported"
	SourceStateForbidden           = "forbidden"
	SourceStateFailed              = "failed"
	SourceStatePartial             = "partial"
	SourceStateAssociationUnverified = "association_unverified"
	SourceStateNotFoundInRange     = "not_found_in_range"
)

// SourceMeta 描述一次业务读取的来源元数据。
type SourceMeta struct {
	Source         string     `json:"source"`
	State          string     `json:"state"`
	ReadAt         *time.Time `json:"readAt"`
	LastSuccessAt  *time.Time `json:"lastSuccessAt,omitempty"`
	Complete       *bool      `json:"complete,omitempty"`
	CoverageStart  *time.Time `json:"coverageStart,omitempty"`
	Message        string     `json:"message,omitempty"`
}

// StatValue 统一统计值，避免前端 value||0 掩盖失败。
type StatValue struct {
	Value      *int64     `json:"value"`
	State      string     `json:"state"`
	AsOf       *time.Time `json:"asOf,omitempty"`
	Definition string     `json:"definition,omitempty"`
}

func NewStatOK(v int64, asOf time.Time, def string) StatValue {
	return StatValue{Value: &v, State: SourceStateReady, AsOf: &asOf, Definition: def}
}

func NewStatNull(state string, asOf *time.Time) StatValue {
	return StatValue{Value: nil, State: state, AsOf: asOf}
}

// BizUser Octop 本地用户投影（不含密码相关字段）。
type BizUser struct {
	UserId      string     `json:"userId"`
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName"`
	Enabled     int8       `json:"enabled"` // 1 启用 / -1 禁用 / 0 未知
	RoleSummary string     `json:"roleSummary,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	ExpertCount StatValue  `json:"expertCount"`
	ThreadCount StatValue  `json:"threadCount"`
	LastActive  *time.Time `json:"lastActive,omitempty"`
}

// BizUserOption 用户选择器最小投影。
type BizUserOption struct {
	UserId      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// BizExpert 实际专家（agents, kind=expert）投影。
type BizExpert struct {
	ExpertId       string         `json:"expertId"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Model          string         `json:"model,omitempty"`
	Enabled        int8           `json:"enabled"`
	LastState      string         `json:"lastState,omitempty"`
	SourceTemplate string         `json:"sourceTemplate,omitempty"`
	SkillPkgCount  *int64         `json:"skillPkgCount,omitempty"`
	CreatedAt      *time.Time     `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time     `json:"updatedAt,omitempty"`
	Config         *BizExpertConfig `json:"config,omitempty"`
}

// BizExpertConfig 专家配置白名单投影。
type BizExpertConfig struct {
	Description    string            `json:"description,omitempty"`
	ModelRef       string            `json:"modelRef,omitempty"`
	ProviderName   string            `json:"providerName,omitempty"`
	ProviderId     string            `json:"providerId,omitempty"`
	Enabled        int8              `json:"enabled"`
	LastState      string            `json:"lastState,omitempty"`
	Temperature    *float64          `json:"temperature,omitempty"`
	TopP           *float64          `json:"topP,omitempty"`
	MaxTokens      *int64            `json:"maxTokens,omitempty"`
	MaxIterations  *int64            `json:"maxIterations,omitempty"`
	InputLength    *int64            `json:"inputLength,omitempty"`
	TrajectoryOn   *bool             `json:"trajectoryOn,omitempty"`
	SkillPackages  []BizRefSummary   `json:"skillPackages,omitempty"`
	KnowledgeBases []BizRefSummary   `json:"knowledgeBases,omitempty"`
	MCPConnections []BizRefSummary   `json:"mcpConnections,omitempty"`
	SubExperts     []BizRefSummary   `json:"subExperts,omitempty"`
	Unsupported    []string          `json:"unsupported,omitempty"`
	PromptPreview  string            `json:"promptPreview,omitempty"` // 仅 content 权限时按需返回
}

// BizRefSummary 挂载引用摘要。
type BizRefSummary struct {
	Id     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"` // ok / missing
}

// BizThread 业务会话投影。
type BizThread struct {
	ThreadId     string     `json:"threadId"`
	Title        string     `json:"title"`
	AgentId      string     `json:"agentId,omitempty"`
	AgentName    string     `json:"agentName,omitempty"`
	Channel      string     `json:"channel,omitempty"`
	CreatedAt    *time.Time `json:"createdAt,omitempty"`
	LastActive   *time.Time `json:"lastActive,omitempty"`
	HasMessages  bool       `json:"hasMessages"`
	TraceStatus  string     `json:"traceStatus,omitempty"` // 未查过不声称无追踪
}

// BizExpertTemplate 用户发布的专家模板。
type BizExpertTemplate struct {
	TemplateId     string     `json:"templateId"`
	Slug           string     `json:"slug,omitempty"`
	Name           string     `json:"name"`
	Avatar         string     `json:"avatar,omitempty"`
	Description    string     `json:"description,omitempty"`
	CreatedBy      string     `json:"createdBy"`
	SourceExpertId string     `json:"sourceExpertId,omitempty"`
	SourceAvailable *bool     `json:"sourceAvailable,omitempty"`
	CreatedAt      *time.Time `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
	Files          []BizFileEntry `json:"files,omitempty"`
}

// BizSkillPackage 技能包投影。
type BizSkillPackage struct {
	PackageId   string     `json:"packageId"`
	Name        string     `json:"name"`
	Icon        string     `json:"icon,omitempty"`
	Description string     `json:"description,omitempty"`
	CreatedBy   string     `json:"createdBy"`
	SkillCount  StatValue  `json:"skillCount"`
	Shared      bool       `json:"shared"`
	Visibility  string     `json:"visibility,omitempty"`
	CopyPolicy  string     `json:"copyPolicy,omitempty"`
	DownloadPolicy string  `json:"downloadPolicy,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}

// BizSkill 包内技能条目。
type BizSkill struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Source      string         `json:"source,omitempty"`
	Files       []BizFileEntry `json:"files,omitempty"`
}

// BizFileEntry 安全文件条目（不可传任意主机路径）。
type BizFileEntry struct {
	FileId   string `json:"fileId"`
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// BizFileContent 按需文件/Prompt 正文（瞬时，不落盘）。
type BizFileContent struct {
	FileId    string `json:"fileId"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	MimeType  string `json:"mimeType,omitempty"`
}

// BusinessQuery 通用业务分页查询。
type BusinessQuery struct {
	model.PageParam
	Keyword   string `json:"keyword" form:"keyword"`
	Enabled   *int8  `json:"enabled" form:"enabled"`
	Model     string `json:"model" form:"model"`
	Channel   string `json:"channel" form:"channel"`
	AgentId   string `json:"agentId" form:"agentId"`
	TimeField string `json:"timeField" form:"timeField"` // created / last_active
	StartTime string `json:"startTime" form:"startTime"`
	EndTime   string `json:"endTime" form:"endTime"`
}

func (q *BusinessQuery) Normalize() {
	if q.PageNum <= 0 {
		q.PageNum = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
}
