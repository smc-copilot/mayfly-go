package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

// CollectorCheckpoint 每套实例、每张源表一条游标。
type CollectorCheckpoint struct {
	model.Model

	InstanceId     uint64     `json:"instanceId" gorm:"not null;uniqueIndex:uk_octop_checkpoint"`
	SourceTable    string     `json:"sourceTable" gorm:"size:64;not null;uniqueIndex:uk_octop_checkpoint"`
	LastRecordId   string     `json:"lastRecordId" gorm:"size:64"`
	LastSuccessAt  *time.Time `json:"lastSuccessAt"`
	RetentionStart *time.Time `json:"retentionStart"` // 当前能采到的源端最早时间（探测所得）
	LastError      string     `json:"lastError" gorm:"size:255"`
	Coverage       string     `json:"coverage" gorm:"size:16"` // ok / unsupported / no_permission / fail
	BackoffUntil   *time.Time `json:"backoffUntil"`
	FailCount      int        `json:"failCount" gorm:"not null;default:0"`
}

func (c *CollectorCheckpoint) TableName() string {
	return "t_octop_collector_checkpoint"
}

// CollectorRun 一次采集运行记录（不含日志原文）
type CollectorRun struct {
	model.ModelNLD

	InstanceId   uint64     `json:"instanceId" gorm:"not null;index"`
	Kind         string     `json:"kind" gorm:"size:16;not null"` // metric / event / casdoor
	Status       string     `json:"status" gorm:"size:16;not null"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	Error        string     `json:"error" gorm:"size:255"`
	Inserted     int        `json:"inserted"`
	Skipped      int        `json:"skipped"`
	RequestId    string     `json:"requestId" gorm:"size:64"`
}

func (c *CollectorRun) TableName() string {
	return "t_octop_collector_run"
}

// OctopSettings 全局采集/留存配置（单行）
type OctopSettings struct {
	model.Model

	EventRetainDays    int `json:"eventRetainDays" gorm:"not null;default:90"`
	MetricSampleSec    int `json:"metricSampleSec" gorm:"not null;default:120"`
	CollectConcurrency int `json:"collectConcurrency" gorm:"not null;default:3"`
	CollectTimeoutSec  int `json:"collectTimeoutSec" gorm:"not null;default:45"`
}

func (o *OctopSettings) TableName() string {
	return "t_octop_settings"
}
