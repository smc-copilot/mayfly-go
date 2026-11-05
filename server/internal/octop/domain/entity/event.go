package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

// OctopEvent 集中库中的业务事件元数据。
//
// 唯一键：source_instance_id + source_table + source_record_id。
// 只保存检索所需白名单字段，不含聊天全文、prompt、工具参数/结果、密码、令牌。
type OctopEvent struct {
	model.ModelNLD

	SourceInstanceId uint64 `json:"sourceInstanceId" gorm:"not null;uniqueIndex:uk_octop_event_src;index:idx_octop_event_query"`
	SourceTable      string `json:"sourceTable" gorm:"size:64;not null;uniqueIndex:uk_octop_event_src"`
	SourceRecordId   string `json:"sourceRecordId" gorm:"size:64;not null;uniqueIndex:uk_octop_event_src"`

	Category string `json:"category" gorm:"size:16;not null;index:idx_octop_event_query"`
	Action   string `json:"action" gorm:"size:64;index"`
	Status   string `json:"status" gorm:"size:32"`

	OctopUserId   string `json:"octopUserId" gorm:"size:128;index"`
	OctopUsername string `json:"octopUsername" gorm:"size:128"`
	AgentId       string `json:"agentId" gorm:"size:128;index"`
	AgentName     string `json:"agentName" gorm:"size:128"` // 展示名快照，统计以 AgentId 为准
	ThreadId      string `json:"threadId" gorm:"size:128;index"`
	TurnId        string `json:"turnId" gorm:"size:128"`
	EventId       string `json:"eventId" gorm:"size:128;index"` // trajectory 的 event_id
	Target        string `json:"target" gorm:"size:128"`
	ToolName      string `json:"toolName" gorm:"size:64"`
	Model         string `json:"model" gorm:"size:64"`
	Source        string `json:"source" gorm:"size:32"` // 来源系统标记：octop / casdoor

	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	DurationMs   int64 `json:"durationMs"`

	IdentityStatus string `json:"identityStatus" gorm:"size:16"` // linked / unlinked
	CasdoorUserId  string `json:"casdoorUserId" gorm:"size:128"`

	SourceTime   *time.Time `json:"sourceTime" gorm:"index:idx_octop_event_query"`
	SourceTimeAbnormal int8 `json:"sourceTimeAbnormal" gorm:"not null;default:-1"`
	CollectedAt  time.Time  `json:"collectedAt"`

	// Diagnostic 脱敏后的必要诊断信息（短文本）。不可安全提取时写「未采集」语义由空+Uncollected=1 表达
	Diagnostic  string `json:"diagnostic" gorm:"size:255"`
	Uncollected int8   `json:"uncollected" gorm:"not null;default:-1"`
}

func (o *OctopEvent) TableName() string {
	return "t_octop_event"
}
