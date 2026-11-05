package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

// OctopMetricSample 实例级合成采样点。
//
// 主机 CPU/内存/磁盘/负载复用 t_machine_metric 的同时，这里保存 Octop 进程与
// PostgreSQL 侧本周期探测结果，以及当时的综合状态。指针/Avail 标记用于区分
// 「值为 0」和「采不到 / 不支持」，禁止把缺口画成零。
type OctopMetricSample struct {
	model.ModelNLD

	InstanceId  uint64    `json:"instanceId" gorm:"not null;index:idx_octop_metric_inst_time"`
	CollectedAt time.Time `json:"collectedAt" gorm:"not null;index:idx_octop_metric_inst_time"`

	HostOnline     *int8    `json:"hostOnline"`
	HostCpu        *float64 `json:"hostCpu"`
	HostMem        *float64 `json:"hostMem"`
	HostDisk       *float64 `json:"hostDisk"`
	HostDiskFreeGB *float64 `json:"hostDiskFreeGb"`
	HostLoad1      *float64 `json:"hostLoad1"`

	OctopProcess   string   `json:"octopProcess" gorm:"size:16"` // ok/fail/unknown
	OctopPid       *int64   `json:"octopPid"`
	OctopCpu       *float64 `json:"octopCpu"`
	OctopRssMB     *float64 `json:"octopRssMb"`
	OctopRestarts  *int64   `json:"octopRestarts"`
	OctopUptimeSec *int64   `json:"octopUptimeSec"`

	HttpHealth string `json:"httpHealth" gorm:"size:16"`
	HttpCode   int    `json:"httpCode"`

	PgConnect     string   `json:"pgConnect" gorm:"size:16"`
	PgPid         *int64   `json:"pgPid"`
	PgCpu         *float64 `json:"pgCpu"`
	PgRssMB       *float64 `json:"pgRssMb"`
	PgDiskFreeGB  *float64 `json:"pgDiskFreeGb"`
	PgConns       *int64   `json:"pgConns"`
	PgActiveConns *int64   `json:"pgActiveConns"`
	PgSizeMB      *float64 `json:"pgSizeMb"`
	PgLockWaits   *int64   `json:"pgLockWaits"`
	PgLockAvail   int8     `json:"pgLockAvail" gorm:"not null;default:-1"` // 1 可采 / -1 不支持或无权限

	HealthStatus string `json:"healthStatus" gorm:"size:16"`
	HealthReason string `json:"healthReason" gorm:"size:255"`
	CollectError string `json:"collectError" gorm:"size:255"`
}

func (o *OctopMetricSample) TableName() string {
	return "t_octop_metric_sample"
}
