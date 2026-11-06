package form

import (
	"strings"

	"mayfly-go/internal/octop/domain/entity"
)

type InstanceForm struct {
	Id                uint64 `json:"id"`
	Name              string `json:"name"`
	Center            string `json:"center"`
	Remark            string `json:"remark"`
	MachineId         uint64 `json:"machineId"`
	ApiURL            string `json:"apiUrl"`
	StartMode         string `json:"startMode"`
	SystemdUnit       string `json:"systemdUnit"`
	ProcessMatch      string `json:"processMatch"`
	DbId              uint64 `json:"dbId"`
	DbName            string `json:"dbName"`
	CollectEnabled    int8   `json:"collectEnabled"`
	Maintenance       int8   `json:"maintenance"`
	SampleIntervalSec int    `json:"sampleIntervalSec"`
	CpuWarn           int    `json:"cpuWarn"`
	CpuCrit           int    `json:"cpuCrit"`
	MemWarn           int    `json:"memWarn"`
	MemCrit           int    `json:"memCrit"`
	DiskWarn          int    `json:"diskWarn"`
	DiskCrit          int    `json:"diskCrit"`
	HttpFailThreshold int    `json:"httpFailThreshold"`
	CallbackURL       string `json:"callbackUrl"`
	ApiToken          string `json:"apiToken"`
}

func (f *InstanceForm) ToEntity() *entity.OctopInstance {
	return &entity.OctopInstance{
		Name: f.Name, Center: f.Center, Remark: f.Remark,
		MachineId: f.MachineId, ApiURL: f.ApiURL, StartMode: f.StartMode,
		SystemdUnit: f.SystemdUnit, ProcessMatch: f.ProcessMatch,
		DbId: f.DbId, DbName: f.DbName, CollectEnabled: f.CollectEnabled,
		Maintenance: f.Maintenance, SampleIntervalSec: f.SampleIntervalSec,
		CpuWarn: f.CpuWarn, CpuCrit: f.CpuCrit, MemWarn: f.MemWarn, MemCrit: f.MemCrit,
		DiskWarn: f.DiskWarn, DiskCrit: f.DiskCrit, HttpFailThreshold: f.HttpFailThreshold,
		CallbackURL: strings.TrimSpace(f.CallbackURL),
	}
}

type CasdoorForm struct {
	Id              uint64 `json:"id"`
	Name            string `json:"name"`
	ServerURL       string `json:"serverUrl"`
	Organization    string `json:"organization"`
	AppOwner        string `json:"appOwner"`
	ClientId        string `json:"clientId"`
	ClientSecret    string `json:"clientSecret"`
	TimeoutSec      int    `json:"timeoutSec"`
	SyncIntervalSec int    `json:"syncIntervalSec"`
	TLSVerify       int8   `json:"tlsVerify"`
}

func (f *CasdoorForm) ToEntity() *entity.CasdoorConnection {
	return &entity.CasdoorConnection{
		Name: f.Name, ServerURL: f.ServerURL, Organization: f.Organization,
		AppOwner: f.AppOwner, ClientId: f.ClientId, TimeoutSec: f.TimeoutSec,
		SyncIntervalSec: f.SyncIntervalSec, TLSVerify: f.TLSVerify,
	}
}

type BindAppForm struct {
	InstanceId       uint64 `json:"instanceId"`
	AppSnapshotId    uint64 `json:"appSnapshotId"`
	ConfirmOverwrite bool   `json:"confirmOverwrite"`
}

type IdentityLinkForm struct {
	ConnectionId        uint64 `json:"connectionId"`
	CasdoorUserSourceId string `json:"casdoorUserSourceId"`
	OctopInstanceId     uint64 `json:"octopInstanceId"`
	OctopUserId         string `json:"octopUserId"`
	Evidence            string `json:"evidence"`
}

type SettingsForm struct {
	EventRetainDays    int `json:"eventRetainDays"`
	MetricSampleSec    int `json:"metricSampleSec"`
	CollectConcurrency int `json:"collectConcurrency"`
	CollectTimeoutSec  int `json:"collectTimeoutSec"`
}

type RecollectForm struct {
	InstanceId  uint64 `json:"instanceId"`
	SourceTable string `json:"sourceTable"`
}
