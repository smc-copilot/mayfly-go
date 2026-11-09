package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

// DirectoryObject 保存角色、群组和权限的白名单投影。
type DirectoryObject struct {
	model.Model
	ConnectionId uint64     `json:"connectionId" gorm:"not null;uniqueIndex:uk_casdoor_object"`
	Kind         string     `json:"kind" gorm:"size:16;not null;uniqueIndex:uk_casdoor_object"`
	SourceId     string     `json:"sourceId" gorm:"size:255;not null;uniqueIndex:uk_casdoor_object"`
	Projection   string     `json:"projection" gorm:"type:text"`
	Fingerprint  string     `json:"-" gorm:"size:128"`
	Present      int8       `json:"present" gorm:"not null;default:1"`
	LastSyncAt   *time.Time `json:"lastSyncAt"`
}

func (DirectoryObject) TableName() string { return "t_casdoor_object" }

// ConnectionScope 限制 Mayfly 账号可管理的 Casdoor 连接。
type ConnectionScope struct {
	model.Model
	AccountId    uint64 `json:"accountId" gorm:"not null;uniqueIndex:uk_casdoor_scope"`
	ConnectionId uint64 `json:"connectionId" gorm:"not null;uniqueIndex:uk_casdoor_scope"`
	Scope        string `json:"scope" gorm:"size:16;not null"`
}

func (ConnectionScope) TableName() string { return "t_casdoor_scope" }

// ManagedApplication 记录 Mayfly 对单个 Casdoor 应用的授权对象。
type ManagedApplication struct {
	model.Model
	ConnectionId        uint64 `json:"connectionId" gorm:"not null;uniqueIndex:uk_casdoor_managed_app"`
	ApplicationSourceId string `json:"applicationSourceId" gorm:"size:255;not null;uniqueIndex:uk_casdoor_managed_app"`
	RoleId              string `json:"roleId" gorm:"size:255"`
	PermissionId        string `json:"permissionId" gorm:"size:255"`
	ManagedMarker       string `json:"managedMarker" gorm:"size:255"`
	Fingerprint         string `json:"-" gorm:"size:128"`
	Status              string `json:"status" gorm:"size:32"`
}

func (ManagedApplication) TableName() string { return "t_casdoor_managed_app" }

// Change 保存变更预览与提交状态。命令仅供后端处理，不直接返回。
type Change struct {
	model.Model
	ConnectionId      uint64     `json:"connectionId" gorm:"not null;uniqueIndex:uk_casdoor_change"`
	IdempotencyKey    *string    `json:"-" gorm:"size:128;uniqueIndex:uk_casdoor_change"`
	ActorId           uint64     `json:"actorId" gorm:"not null"`
	Action            string     `json:"action" gorm:"size:32;not null"`
	Command           string     `json:"-" gorm:"type:text"`
	SourceFingerprint string     `json:"-" gorm:"size:128"`
	Status            string     `json:"status" gorm:"size:16"`
	Result            string     `json:"-" gorm:"type:text"`
	ExpiresAt         *time.Time `json:"expiresAt"`
	SubmittedAt       *time.Time `json:"submittedAt"`
	VerifiedAt        *time.Time `json:"verifiedAt"`
}

func (Change) TableName() string { return "t_casdoor_change" }
