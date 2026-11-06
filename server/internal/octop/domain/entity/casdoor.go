package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

// CasdoorConnection 一条 Casdoor 连接配置。
//
// ClientSecretEnc 仅后端保存，接口只回显「已配置/未配置」和最近轮换时间。
type CasdoorConnection struct {
	model.Model

	Name            string `json:"name" gorm:"size:64;not null"`
	ServerURL       string `json:"serverUrl" gorm:"size:255;not null"`
	Organization    string `json:"organization" gorm:"size:64;not null"`
	AppOwner        string `json:"appOwner" gorm:"size:64"` // 应用对象 owner，不得默认等于组织名
	ClientId        string `json:"clientId" gorm:"size:128;not null"`
	ClientSecretEnc string `json:"-" gorm:"column:client_secret_enc;size:512"`
	Certificate     string `json:"-" gorm:"type:text"` // 可选 TLS/JWT 证书，不下发前端
	TimeoutSec      int    `json:"timeoutSec" gorm:"not null;default:15"`
	SyncIntervalSec int    `json:"syncIntervalSec" gorm:"not null;default:3600"`
	TLSVerify       int8   `json:"tlsVerify" gorm:"not null;default:1"`

	CredentialConfigured int8       `json:"credentialConfigured" gorm:"not null;default:-1"`
	CredentialRotatedAt  *time.Time `json:"credentialRotatedAt"`

	LastSyncStatus string     `json:"lastSyncStatus" gorm:"size:16"`
	LastSyncAt     *time.Time `json:"lastSyncAt"`
	LastSyncError  string     `json:"lastSyncError" gorm:"size:255"`
	LastSuccessAt  *time.Time `json:"lastSuccessAt"`

	AppSyncStatus     string     `json:"appSyncStatus" gorm:"size:16"`
	AppSyncAt         *time.Time `json:"appSyncAt"`
	AppSuccessAt      *time.Time `json:"appSuccessAt"`
	AppSyncError      string     `json:"appSyncError" gorm:"size:255"`
	AppSyncCount      int        `json:"appSyncCount"`
	AppSyncDurationMs int64      `json:"appSyncDurationMs"`

	UserSyncStatus     string     `json:"userSyncStatus" gorm:"size:16"`
	UserSyncAt         *time.Time `json:"userSyncAt"`
	UserSuccessAt      *time.Time `json:"userSuccessAt"`
	UserSyncError      string     `json:"userSyncError" gorm:"size:255"`
	UserSyncCount      int        `json:"userSyncCount"`
	UserSyncDurationMs int64      `json:"userSyncDurationMs"`

	RelationSyncStatus      string     `json:"relationSyncStatus" gorm:"size:16"`
	RelationSyncAt          *time.Time `json:"relationSyncAt"`
	RelationSuccessAt       *time.Time `json:"relationSuccessAt"`
	RelationSyncError       string     `json:"relationSyncError" gorm:"size:255"`
	RelationConfirmedCount  int        `json:"relationConfirmedCount"`
	RelationUnverifiedCount int        `json:"relationUnverifiedCount"`
	RelationFailedCount     int        `json:"relationFailedCount"`
	RelationSyncDurationMs  int64      `json:"relationSyncDurationMs"`
}

func (c *CasdoorConnection) TableName() string {
	return "t_octop_casdoor_connection"
}

func (c *CasdoorConnection) HasCredential() bool {
	return c.CredentialConfigured == 1 && c.ClientSecretEnc != ""
}

// CasdoorApplicationSnapshot Casdoor 应用快照（不含密钥）
type CasdoorApplicationSnapshot struct {
	model.Model

	ConnectionId   uint64     `json:"connectionId" gorm:"not null;uniqueIndex:uk_octop_casdoor_app"`
	SourceId       string     `json:"sourceId" gorm:"size:128;not null;uniqueIndex:uk_octop_casdoor_app"`
	Name           string     `json:"name" gorm:"size:128;not null"`
	DisplayName    string     `json:"displayName" gorm:"size:128"`
	Organization   string     `json:"organization" gorm:"size:64"`
	AppOwner       string     `json:"appOwner" gorm:"size:64"`
	Enabled        int8       `json:"enabled" gorm:"not null;default:0"` // 1 启用 / -1 停用 / 0 未知，缺失字段不得推断为启用
	AppType        string     `json:"appType" gorm:"size:32"`
	HomepageURL    string     `json:"homepageUrl" gorm:"size:255"`
	RedirectURIs   string     `json:"redirectUris" gorm:"type:text"`     // OIDC redirect/callback URL 白名单 JSON，不含密钥
	Present        int8       `json:"present" gorm:"not null;default:1"` // 本次成功同步是否仍存在
	RelationStatus string     `json:"relationStatus" gorm:"size:16"`     // confirmed / unverified / unsupported
	LastSyncAt     *time.Time `json:"lastSyncAt"`
}

func (c *CasdoorApplicationSnapshot) TableName() string {
	return "t_octop_casdoor_app"
}

// CasdoorUserSnapshot Casdoor 用户快照（不含密码/哈希/令牌）
type CasdoorUserSnapshot struct {
	model.Model

	ConnectionId    uint64     `json:"connectionId" gorm:"not null;uniqueIndex:uk_octop_casdoor_user"`
	SourceId        string     `json:"sourceId" gorm:"size:128;not null;uniqueIndex:uk_octop_casdoor_user"`
	Username        string     `json:"username" gorm:"size:128;not null;index"`
	DisplayName     string     `json:"displayName" gorm:"size:128"`
	Organization    string     `json:"organization" gorm:"size:64"`
	Email           string     `json:"email" gorm:"size:128"`
	Enabled         int8       `json:"enabled" gorm:"not null;default:0"` // 1 启用 / -1 禁用 / 0 未知
	SourceUpdatedAt *time.Time `json:"sourceUpdatedAt"`
	Present         int8       `json:"present" gorm:"not null;default:1"`
	LastSyncAt      *time.Time `json:"lastSyncAt"`
}

func (c *CasdoorUserSnapshot) TableName() string {
	return "t_octop_casdoor_user"
}

// CasdoorAuthRelation 可确认的应用授权关系。无法判定时不写 confirmed。
type CasdoorAuthRelation struct {
	model.Model

	ConnectionId        uint64     `json:"connectionId" gorm:"not null;uniqueIndex:uk_octop_casdoor_auth"`
	ApplicationSourceId string     `json:"applicationSourceId" gorm:"size:128;not null;uniqueIndex:uk_octop_casdoor_auth"`
	SubjectType         string     `json:"subjectType" gorm:"size:16;not null;uniqueIndex:uk_octop_casdoor_auth"` // user/org/group
	SubjectSourceId     string     `json:"subjectSourceId" gorm:"size:128;not null;uniqueIndex:uk_octop_casdoor_auth"`
	Status              string     `json:"status" gorm:"size:16;not null"` // confirmed / unverified / revoked
	LastSyncAt          *time.Time `json:"lastSyncAt"`
	LastConfirmedAt     *time.Time `json:"lastConfirmedAt"`
}

func (c *CasdoorAuthRelation) TableName() string {
	return "t_octop_casdoor_auth"
}
