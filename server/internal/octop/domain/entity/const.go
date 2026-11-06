package entity

// 启动模式
const (
	StartModeSystemd = "systemd"
	StartModeCLI     = "cli"
)

// 实例运维状态（总览/详情展示用，不是采集器自己的运行态）
const (
	HealthNormal      = "normal"      // 正常
	HealthWarning     = "warning"     // 告警
	HealthCritical    = "critical"    // 严重
	HealthUnknown     = "unknown"     // 未知/采集失败
	HealthMaintenance = "maintenance" // 维护中
)

// 单项探测结果
const (
	ProbeOK          = "ok"
	ProbeFail        = "fail"
	ProbeUnknown     = "unknown"     // 采集器故障或凭据失效
	ProbeUnsupported = "unsupported" // 不支持 / 无权限
	ProbeSkipped     = "skipped"
)

// 目录同步状态（PRD：未同步 / 同步中 / 成功 / 失败 / 上次成功数据已过期）
const (
	SyncStatusNever   = "never"
	SyncStatusSyncing = "syncing"
	SyncStatusSuccess = "success"
	SyncStatusFailed  = "failed"
	SyncStatusStale   = "stale"

	// 兼容已有行：历史写入 ok/expired/running，展示时按新口径映射
	SyncStatusOK      = "ok"
	SyncStatusExpired = "expired"
	SyncStatusRunning = "running"
)

// 授权关系核实状态
const (
	AuthConfirmed   = "confirmed"   // Casdoor 实际返回且可读
	AuthUnverified  = "unverified"  // 无法判定，保留上次已确认快照
	AuthUnsupported = "unsupported" // 无可靠关系接口，暂不支持自动确认
	AuthRevoked     = "revoked"     // 本次完整读取后确认已撤销
)

// CasdoorSyncPollInterval 自动同步调度精度：cron 约每 30 分钟检查到期连接
const CasdoorSyncPollInterval = 30 * 60 // seconds

// 身份关联状态
const (
	IdentityLinked   = "linked"
	IdentityUnlinked = "unlinked"
)

// 事件类别
const (
	EventLogin      = "login"
	EventOperation  = "operation"
	EventCall       = "call"
	EventTrajectory = "trajectory"
)

// 源表名（与 Octop 部署版默认 schema 对齐，可按实例覆盖）
const (
	SourceTableAuditLog         = "audit_log"
	SourceTableUsageLog         = "usage_log"
	SourceTableTrajectoryEvents = "trajectory_events"
)

// 采集运行状态
const (
	CollectStatusOK      = "ok"
	CollectStatusFail    = "fail"
	CollectStatusPartial = "partial"
	CollectStatusRunning = "running"
)

// 事件查询缺口原因（四种状态不可混淆）
const (
	GapNone         = "" // 该时段确无事件
	GapCollectFail  = "collect_fail"
	GapSourcePurged = "source_purged"
	GapUnsupported  = "unsupported"
)

const (
	DefaultSampleIntervalSec  = 120
	DefaultEventRetainDays    = 90
	DefaultCollectConcurrency = 3
	DefaultCollectTimeoutSec  = 45
	DefaultHttpFailThreshold  = 3
	DefaultCpuWarn            = 85
	DefaultCpuCrit            = 95
	DefaultMemWarn            = 85
	DefaultMemCrit            = 95
	DefaultDiskWarn           = 85
	DefaultDiskCrit           = 95
	DefaultSyncPageSize       = 100
	DefaultCollectBatchSize   = 500
	DefaultBackoffSec         = 30
	DefaultMaxBackoffSec      = 300
)

// Casdoor 应用与 Octop 实例的 OIDC 回调对照结果
const (
	BindMatchNone       = "none"       // 回调地址齐全但没有实例对上
	BindMatchUnique     = "unique"     // 恰好一个实例
	BindMatchAmbiguous  = "ambiguous"  // 多回调或多实例候选
	BindMatchUnverified = "unverified" // 无可靠回调字段，未核实
)

const (
	MatchBasisCallbackURL = "callbackUrl"
	MatchBasisApiURL      = "apiUrl"
)

// Octop 用户列表读取状态（不是用户账号状态）
const (
	UserListOK           = "ok"
	UserListUnconfigured = "unconfigured"
	UserListForbidden    = "forbidden"
	UserListUnreachable  = "unreachable"
	UserListUnexpected   = "unexpected"
)

const (
	UserStatusEnabled  = "enabled"
	UserStatusDisabled = "disabled"
	UserStatusUnknown  = "unknown"
)

const (
	UserFilterModeLocalFull = "local_full_list"
)
