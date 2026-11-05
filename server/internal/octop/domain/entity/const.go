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

// 目录同步状态
const (
	SyncStatusNever   = "never"
	SyncStatusOK      = "ok"
	SyncStatusExpired = "expired" // 上次成功快照仍在，但本次失败
	SyncStatusRunning = "running"
)

// 授权关系核实状态
const (
	AuthConfirmed   = "confirmed"   // Casdoor 实际返回且可读
	AuthUnverified  = "unverified"  // 无法判定
	AuthRevoked     = "revoked"     // 同步后确认已撤销
)

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
	SourceTableAuditLog          = "audit_log"
	SourceTableUsageLog          = "usage_log"
	SourceTableTrajectoryEvents  = "trajectory_events"
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
	GapNone            = ""            // 该时段确无事件
	GapCollectFail     = "collect_fail"
	GapSourcePurged    = "source_purged"
	GapUnsupported     = "unsupported"
)

const (
	DefaultSampleIntervalSec   = 120
	DefaultEventRetainDays     = 90
	DefaultCollectConcurrency  = 3
	DefaultCollectTimeoutSec   = 45
	DefaultHttpFailThreshold   = 3
	DefaultCpuWarn             = 85
	DefaultCpuCrit             = 95
	DefaultMemWarn             = 85
	DefaultMemCrit             = 95
	DefaultDiskWarn            = 85
	DefaultDiskCrit            = 95
	DefaultSyncPageSize        = 100
	DefaultCollectBatchSize    = 500
	DefaultBackoffSec          = 30
	DefaultMaxBackoffSec       = 300
)
