package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

// OctopInstance 一套 Octop + PostgreSQL 的注册记录。
//
// 主机 SSH 与数据库凭据只引用现有 Mayfly 资源，不在本表重复保存明文。
type OctopInstance struct {
	model.Model
	model.ExtraData

	Code   string `json:"code" gorm:"size:32;not null;uniqueIndex:uk_octop_instance_code"`
	Name   string `json:"name" gorm:"size:64;not null"`
	Center string `json:"center" gorm:"size:64"` // 所属中心或部门
	Remark string `json:"remark" gorm:"size:255"`

	MachineId uint64 `json:"machineId" gorm:"not null;index"`
	ApiURL    string `json:"apiUrl" gorm:"size:255"` // Octop 内网 API，如 http://10.x.x.x:8080
	// CallbackURL 是对外 OIDC 回调 origin 或完整回调 URL，可能与内网 ApiURL 不同。
	CallbackURL string `json:"callbackUrl" gorm:"size:512"`
	// ApiTokenEnc 是调用 Octop 受保护 API 的 JWT，AES 加密存储，接口不回传。
	ApiTokenEnc        string `json:"-" gorm:"column:api_token_enc;size:1024"`
	ApiTokenConfigured int8   `json:"apiTokenConfigured" gorm:"not null;default:-1"`

	StartMode    string `json:"startMode" gorm:"size:16;not null;default:cli"` // systemd / cli
	SystemdUnit  string `json:"systemdUnit" gorm:"size:128"`
	ProcessMatch string `json:"processMatch" gorm:"size:64"` // 命令行实例的进程名，仅允许简单标识

	DbId   uint64 `json:"dbId"`
	DbName string `json:"dbName" gorm:"size:64"`

	CasdoorConnectionId  uint64 `json:"casdoorConnectionId"`
	CasdoorAppSnapshotId uint64 `json:"casdoorAppSnapshotId"`

	CollectEnabled    int8 `json:"collectEnabled" gorm:"not null;default:1"`
	Maintenance       int8 `json:"maintenance" gorm:"not null;default:-1"`
	SampleIntervalSec int  `json:"sampleIntervalSec" gorm:"not null;default:120"`

	CpuWarn           int `json:"cpuWarn" gorm:"not null;default:85"`
	CpuCrit           int `json:"cpuCrit" gorm:"not null;default:95"`
	MemWarn           int `json:"memWarn" gorm:"not null;default:85"`
	MemCrit           int `json:"memCrit" gorm:"not null;default:95"`
	DiskWarn          int `json:"diskWarn" gorm:"not null;default:85"`
	DiskCrit          int `json:"diskCrit" gorm:"not null;default:95"`
	HttpFailThreshold int `json:"httpFailThreshold" gorm:"not null;default:3"`

	// 最近一次健康探测的摘要（非密钥）
	LastHealthStatus     string     `json:"lastHealthStatus" gorm:"size:16"`
	LastHealthReason     string     `json:"lastHealthReason" gorm:"size:255"`
	LastCollectAt        *time.Time `json:"lastCollectAt"`
	LastCollectError     string     `json:"lastCollectError" gorm:"size:255"`
	ConsecutiveHttpFails int        `json:"consecutiveHttpFails" gorm:"not null;default:0"`
}

func (o *OctopInstance) TableName() string {
	return "t_octop_instance"
}

func (o *OctopInstance) IsSystemd() bool {
	return o.StartMode == StartModeSystemd && o.SystemdUnit != ""
}

func (o *OctopInstance) IsMaintenance() bool {
	return o.Maintenance == 1
}

func (o *OctopInstance) CollectOn() bool {
	return o.CollectEnabled == 1
}

func (o *OctopInstance) Interval() time.Duration {
	sec := o.SampleIntervalSec
	if sec <= 0 {
		sec = DefaultSampleIntervalSec
	}
	return time.Duration(sec) * time.Second
}
