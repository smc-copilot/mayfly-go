package dto

import (
	"time"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
)

type SaveInstance struct {
	*entity.OctopInstance
}

type ProbeItem struct {
	Name   string     `json:"name"`
	Status string     `json:"status"`
	Reason string     `json:"reason"`
	At     *time.Time `json:"at,omitempty"`
}

type InstanceOverview struct {
	*entity.OctopInstance
	MachineName    string     `json:"machineName"`
	MachineIp      string     `json:"machineIp"`
	MachineOnline  string     `json:"machineOnline"`
	OctopProcess   string     `json:"octopProcess"`
	HttpHealth     string     `json:"httpHealth"`
	PgConnect      string     `json:"pgConnect"`
	HealthStatus   string     `json:"healthStatus"`
	HealthReason   string     `json:"healthReason"`
	LastCollectAt  *time.Time `json:"lastCollectAt"`
	CasdoorAppName string     `json:"casdoorAppName"`
}

type InstanceDetail struct {
	Overview     *InstanceOverview             `json:"overview"`
	Probes       []ProbeItem                   `json:"probes"`
	Thresholds   *entity.OctopInstance         `json:"thresholds"`
	CanRestart   bool                          `json:"canRestart"`
	RestartHint  string                        `json:"restartHint"`
	Checkpoints  []*entity.CollectorCheckpoint `json:"checkpoints"`
	LatestSample *entity.OctopMetricSample     `json:"latestSample"`
}

type ConnTestResult struct {
	SSH   ProbeItem `json:"ssh"`
	HTTP  ProbeItem `json:"http"`
	PG    ProbeItem `json:"pg"`
	Saved string    `json:"saved"` // ok / pending_fix
}

type RestartResult struct {
	Output     string `json:"output"`
	Notice     string `json:"notice"`
	DurationMs int64  `json:"durationMs"`
	UnitActive string `json:"unitActive"`
	HttpHealth string `json:"httpHealth"`
}

type CasdoorTestResult struct {
	Service ProbeItem `json:"service"`
	Apps    ProbeItem `json:"apps"`
	Users   ProbeItem `json:"users"`
}

type CategorySyncResult struct {
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Error      string     `json:"error"`
	Count      int        `json:"count"`
	DurationMs int64      `json:"durationMs"`
	AttemptAt  *time.Time `json:"attemptAt,omitempty"`
	SuccessAt  *time.Time `json:"successAt,omitempty"`
	Confirmed  int        `json:"confirmed,omitempty"`
	Unverified int        `json:"unverified,omitempty"`
	Failed     int        `json:"failed,omitempty"`
}

type CasdoorSyncResult struct {
	Apps      CategorySyncResult `json:"apps"`
	Users     CategorySyncResult `json:"users"`
	Relations CategorySyncResult `json:"relations"`
}

type EventGap struct {
	InstanceId     uint64     `json:"instanceId"`
	SourceTable    string     `json:"sourceTable"`
	Coverage       string     `json:"coverage"`
	Reason         string     `json:"reason"`
	LastRecordId   string     `json:"lastRecordId"`
	LastSuccessAt  *time.Time `json:"lastSuccessAt"`
	RetentionStart *time.Time `json:"retentionStart"`
}

type EventPage struct {
	Total    int64                `json:"total"`
	List     []*entity.OctopEvent `json:"list"`
	PageNum  int                  `json:"pageNum"`
	PageSize int                  `json:"pageSize"`
	Gaps     []*EventGap          `json:"gaps"`
}

type EventSummary = repository.EventSummary

// ---------------------------------------------------------------- business

// BusinessPage 业务分页结果，附来源元数据；来源异常时 List 为空且 Source.State 非 ready，不得当作“0 条”。
type BusinessPage[T any] struct {
	List     []*T               `json:"list"`
	Total    int64              `json:"total"`
	PageNum  int                `json:"pageNum"`
	PageSize int                `json:"pageSize"`
	Source   *entity.SourceMeta `json:"source"`
}

// BusinessDetail 业务详情结果；来源不可用时 Item 为空，Source 说明原因。
type BusinessDetail[T any] struct {
	Item   *T                 `json:"item"`
	Source *entity.SourceMeta `json:"source"`
}

// UserDetail 用户详情及四项统计。
type UserDetail struct {
	User               *entity.BizUser    `json:"user"`
	ExpertCount        entity.StatValue   `json:"expertCount"`
	ThreadCount        entity.StatValue   `json:"threadCount"`
	TemplateCount      entity.StatValue   `json:"templateCount"`
	SkillPackageCount  entity.StatValue   `json:"skillPackageCount"`
	Source             *entity.SourceMeta `json:"source"`
}

// CapabilityItem 单个来源能力状态。
type CapabilityItem struct {
	Code       string     `json:"code"`
	State      string     `json:"state"`
	Message    string     `json:"message,omitempty"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
}

// CapabilityScope 调用方需要的能力范围，避免为无权查看的来源做探测。
type CapabilityScope struct {
	BusinessDB   bool
	OctopPreview bool
	Langfuse     bool
}

// Capabilities 实例业务读取能力；未请求的来源为 nil。
type Capabilities struct {
	InstanceId   uint64          `json:"instanceId"`
	BusinessDB   *CapabilityItem `json:"businessDb,omitempty"`
	OctopPreview *CapabilityItem `json:"octopPreview,omitempty"`
	Langfuse     *CapabilityItem `json:"langfuse,omitempty"`
}

// FileContentQuery 文件正文读取定位：只能经父资源（技能/模板）引用服务端返回的文件条目 ID。
type FileContentQuery struct {
	InstanceId uint64
	UserId     string
	Kind       string // skill / template
	PackageId  string // Kind=skill
	Slug       string // Kind=skill
	TemplateId string // Kind=template
	FileId     string
}

const (
	FileKindSkill    = "skill"
	FileKindTemplate = "template"
)

// ReadConnectionView 读取连接回显：不含任何密文，仅返回已配置标志。
//
// PublicKey 是公钥/客户端标识而非 Secret，原样回显以便编辑。
type ReadConnectionView struct {
	Id                   uint64     `json:"id"`
	InstanceId           uint64     `json:"instanceId"`
	Type                 string     `json:"type"`
	BaseURL              string     `json:"baseUrl"`
	ProjectId            string     `json:"projectId"`
	PublicKey            string     `json:"publicKey"`
	CredentialConfigured bool       `json:"credentialConfigured"`
	CredentialRotatedAt  *time.Time `json:"credentialRotatedAt"`
	Revision             int64      `json:"revision"`
	VerifyState          string     `json:"verifyState"`
	VerifiedAt           *time.Time `json:"verifiedAt"`
	VerifyError          string     `json:"verifyError"`
}

// ReadConnectionSave 保存读取连接的入参。
//
// Secret 为空表示保留原密钥；ClearSecret=true 显式清除；Revision 必须与当前版本一致（新建为 0）。
type ReadConnectionSave struct {
	Type        string
	BaseURL     string
	ProjectId   string
	PublicKey   string
	Secret      string
	ClearSecret bool
	Revision    int64
}

// ReadConnectionTest 测试读取连接的入参（不保存）。Secret 为空时使用已保存密钥。
type ReadConnectionTest struct {
	Type      string
	BaseURL   string
	ProjectId string
	PublicKey string
	Secret    string
	Revision  int64 // >0 且候选配置与已保存一致时，验证结果会记到该版本上
}

// ReadConnectionTestResult 读取连接测试结果。
type ReadConnectionTestResult struct {
	Ok      bool       `json:"ok"`
	State   string     `json:"state"`
	Message string     `json:"message"`
	At      *time.Time `json:"at,omitempty"`
}
