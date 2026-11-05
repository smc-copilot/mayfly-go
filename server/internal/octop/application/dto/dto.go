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
	Name    string `json:"name"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	At      *time.Time `json:"at,omitempty"`
}

type InstanceOverview struct {
	*entity.OctopInstance
	MachineName  string      `json:"machineName"`
	MachineIp    string      `json:"machineIp"`
	MachineOnline string     `json:"machineOnline"`
	OctopProcess  string     `json:"octopProcess"`
	HttpHealth    string     `json:"httpHealth"`
	PgConnect     string     `json:"pgConnect"`
	HealthStatus  string     `json:"healthStatus"`
	HealthReason  string     `json:"healthReason"`
	LastCollectAt *time.Time `json:"lastCollectAt"`
	CasdoorAppName string    `json:"casdoorAppName"`
}

type InstanceDetail struct {
	Overview     *InstanceOverview          `json:"overview"`
	Probes       []ProbeItem                `json:"probes"`
	Thresholds   *entity.OctopInstance      `json:"thresholds"`
	CanRestart   bool                       `json:"canRestart"`
	RestartHint  string                     `json:"restartHint"`
	Checkpoints  []*entity.CollectorCheckpoint `json:"checkpoints"`
	LatestSample *entity.OctopMetricSample  `json:"latestSample"`
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
	Apps  ProbeItem `json:"apps"`
	Users ProbeItem `json:"users"`
}

type EventGap struct {
	InstanceId   uint64 `json:"instanceId"`
	SourceTable  string `json:"sourceTable"`
	Coverage     string `json:"coverage"`
	Reason       string `json:"reason"`
	LastRecordId string `json:"lastRecordId"`
	LastSuccessAt *time.Time `json:"lastSuccessAt"`
	RetentionStart *time.Time `json:"retentionStart"`
}

type EventPage struct {
	Total int64                 `json:"total"`
	List  []*entity.OctopEvent  `json:"list"`
	PageNum  int                `json:"pageNum"`
	PageSize int                `json:"pageSize"`
	Gaps  []*EventGap           `json:"gaps"`
}

type EventSummary = repository.EventSummary
