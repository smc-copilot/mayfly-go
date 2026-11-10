package entity

import (
	"time"

	"mayfly-go/pkg/model"
)

const (
	ReadConnTypeOctopAPI = "octop_api"
	ReadConnTypeLangfuse = "langfuse"

	VerifyStateReady         = "ready"
	VerifyStateNotConfigured = "not_configured"
	VerifyStateNotVerified   = "not_verified"
	VerifyStateUnsupported   = "unsupported"
	VerifyStateForbidden     = "forbidden"
	VerifyStateFailed        = "failed"
	VerifyStatePartial       = "partial"
)

// OctopReadConnection 实例业务只读连接配置（Octop 预览 API / Langfuse）。
//
// SecretEnc 仅后端保存；接口只回显已配置标志、更新时间与验证状态。
// PostgreSQL 连接不存本表，沿用实例已有 DbId/DbName 资源引用。
type OctopReadConnection struct {
	model.Model

	InstanceId uint64 `json:"instanceId" gorm:"not null;uniqueIndex:uk_octop_read_conn"`
	Type       string `json:"type" gorm:"size:32;not null;uniqueIndex:uk_octop_read_conn"` // octop_api / langfuse

	BaseURL   string `json:"baseUrl" gorm:"size:255"`
	ProjectId string `json:"projectId" gorm:"size:128"` // Langfuse 项目
	PublicKey string `json:"publicKey" gorm:"size:255"` // 可掩码回显的公钥/客户端标识

	SecretEnc            string     `json:"-" gorm:"column:secret_enc;size:512"`
	CredentialConfigured int8       `json:"credentialConfigured" gorm:"not null;default:-1"`
	CredentialRotatedAt  *time.Time `json:"credentialRotatedAt"`

	AuthHeaderEnc string `json:"-" gorm:"column:auth_header_enc;size:1024"` // 可选认证头密文
	CookieEnc     string `json:"-" gorm:"column:cookie_enc;size:2048"`      // 可选 Cookie 密文（不推荐长期使用）

	Revision    int64      `json:"revision" gorm:"not null;default:1"`
	VerifyState string     `json:"verifyState" gorm:"size:32;not null;default:not_configured"`
	VerifiedAt  *time.Time `json:"verifiedAt"`
	VerifyError string     `json:"verifyError" gorm:"size:255"`
	ExtraJSON   string     `json:"-" gorm:"column:extra_json;type:text"`
}

func (c *OctopReadConnection) TableName() string {
	return "t_octop_read_connection"
}

func (c *OctopReadConnection) HasCredential() bool {
	return c.CredentialConfigured == 1 && c.SecretEnc != ""
}
