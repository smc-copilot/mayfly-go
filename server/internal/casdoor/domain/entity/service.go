package entity

import (
	"mayfly-go/pkg/model"
	"time"
)

type Service struct {
	model.Model
	CredentialRevision uint64     `json:"-" gorm:"-"`
	Name               string     `json:"name" gorm:"size:64;not null"`
	ServerURL          string     `json:"serverUrl" gorm:"size:255;not null;uniqueIndex:uk_casdoor_service_url"`
	TimeoutSec         int        `json:"timeoutSec"`
	SyncIntervalSec    int        `json:"syncIntervalSec"`
	TLSVerify          int8       `json:"tlsVerify"`
	Status             string     `json:"status" gorm:"size:32"`
	ReasonCode         string     `json:"reasonCode" gorm:"size:64"`
	VerifiedAt         *time.Time `json:"verifiedAt"`
}

func (Service) TableName() string { return "t_casdoor_service" }

type ManagementCredential struct {
	model.Model
	ServiceID uint64 `json:"serviceId" gorm:"not null;uniqueIndex:uk_casdoor_service_credential"`
	ClientID  string `json:"clientId" gorm:"size:128;not null"`
	SecretEnc string `json:"-" gorm:"size:512;not null"`
	Revision  uint64 `json:"revision"`
}

func (ManagementCredential) TableName() string { return "t_casdoor_management_credential" }

type ServiceOrganization struct {
	model.Model
	SyncStates   map[string]DirectoryStatus `json:"syncStates" gorm:"-"`
	ServiceID    uint64                     `json:"serviceId" gorm:"not null;uniqueIndex:uk_casdoor_service_org"`
	SourceID     string                     `json:"sourceId" gorm:"size:255"`
	Name         string                     `json:"name" gorm:"size:100;not null;uniqueIndex:uk_casdoor_service_org"`
	DisplayName  string                     `json:"displayName" gorm:"size:255"`
	ConnectionID uint64                     `json:"-" gorm:"not null;uniqueIndex:uk_casdoor_org_context"`
	Present      int8                       `json:"present"`
	LastSyncAt   *time.Time                 `json:"lastSyncAt"`
}

type DirectoryStatus struct {
	Status     string     `json:"status"`
	AttemptAt  *time.Time `json:"attemptAt"`
	SuccessAt  *time.Time `json:"successAt"`
	Count      int        `json:"count"`
	ReasonCode string     `json:"reasonCode"`
}

func (ServiceOrganization) TableName() string { return "t_casdoor_service_organization" }

type ServiceScope struct {
	model.Model
	ServiceID uint64 `json:"serviceId" gorm:"not null;uniqueIndex:uk_casdoor_service_scope"`
	AccountID uint64 `json:"accountId" gorm:"not null;uniqueIndex:uk_casdoor_service_scope"`
	Scope     string `json:"scope" gorm:"size:16"`
}

func (ServiceScope) TableName() string { return "t_casdoor_service_scope" }
