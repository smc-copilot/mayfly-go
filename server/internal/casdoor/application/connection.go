package application

import (
	"context"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/internal/casdoor/imsg"
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
)

type ConnectionService struct {
	connections repository.Connections
	secrets     repository.Secrets
	access      *ScopeAuthorizer
}

func NewConnectionService(connections repository.Connections, secrets repository.Secrets, access *ScopeAuthorizer) *ConnectionService {
	return &ConnectionService{connections: connections, secrets: secrets, access: access}
}

// Save 保存本地连接配置；未提供 secret 或开关时保留已保存值。
func (s *ConnectionService) Save(ctx context.Context, input *entity.Connection, secret string, managementEnabled *bool) error {
	if input == nil || s == nil || s.connections == nil || s.secrets == nil || s.access == nil {
		return errorx.NewBizI(ctx, imsg.ErrConnectionInvalid)
	}
	candidate := *input
	candidate.Name = strings.TrimSpace(candidate.Name)
	candidate.Organization = strings.TrimSpace(candidate.Organization)
	candidate.ClientId = strings.TrimSpace(candidate.ClientId)
	candidate.ServerURL = strings.TrimRight(strings.TrimSpace(candidate.ServerURL), "/")
	if candidate.TimeoutSec == 0 {
		candidate.TimeoutSec = 15
	}
	if candidate.SyncIntervalSec == 0 {
		candidate.SyncIntervalSec = 3600
	}
	if !validConnection(&candidate) {
		return errorx.NewBizI(ctx, imsg.ErrConnectionInvalid)
	}
	if strings.TrimSpace(secret) == "" {
		secret = ""
	}
	var saved *entity.Connection
	if candidate.Id == 0 {
		account := contextx.GetLoginAccount(ctx)
		if account == nil || account.Id != consts.AdminId {
			return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
		}
		saved = &entity.Connection{}
	} else {
		if err := s.access.AssertConnectionAccess(ctx, candidate.Id, true); err != nil {
			return err
		}
		var err error
		saved, err = s.connections.GetConnection(ctx, candidate.Id)
		if err != nil {
			return err
		}
		if saved == nil {
			return errorx.NewBizI(ctx, imsg.ErrConnectionNotFound)
		}
		if saved.ServiceID != 0 {
			return errorx.NewBizI(ctx, imsg.ErrServiceOrganizationInvalid)
		}
		if strings.TrimRight(saved.ServerURL, "/") != candidate.ServerURL || saved.Organization != candidate.Organization {
			return errorx.NewBizI(ctx, imsg.ErrConnectionTargetImmutable)
		}
		if saved.ClientId != candidate.ClientId && secret == "" {
			return errorx.NewBizI(ctx, imsg.ErrConnectionSecretRequired)
		}
	}
	if secret == "" && saved.ClientSecretEnc == "" {
		return errorx.NewBizI(ctx, imsg.ErrConnectionSecretRequired)
	}
	if managementEnabled != nil && *managementEnabled && (!saved.ManagementEnabled || secret != "") {
		return errorx.NewBizI(ctx, imsg.ErrManagementUnverified)
	}
	if secret != "" {
		encrypted, err := s.secrets.Encrypt(secret)
		if err != nil {
			return err
		}
		saved.ClientSecretEnc = encrypted
		saved.CredentialConfigured = 1
		saved.ManagementEnabled = false
		now := time.Now().UTC()
		saved.CredentialRotatedAt = &now
	}
	saved.Name = candidate.Name
	saved.ServerURL = candidate.ServerURL
	saved.Organization = candidate.Organization
	saved.AppOwner = strings.TrimSpace(candidate.AppOwner)
	saved.ClientId = candidate.ClientId
	saved.TimeoutSec = candidate.TimeoutSec
	saved.SyncIntervalSec = candidate.SyncIntervalSec
	saved.TLSVerify = candidate.TLSVerify
	if managementEnabled != nil {
		saved.ManagementEnabled = *managementEnabled
	}
	if err := s.connections.SaveConnection(ctx, saved); err != nil {
		return err
	}
	input.Id = saved.Id
	return nil
}

func validConnection(conn *entity.Connection) bool {
	if conn.Name == "" || utf8.RuneCountInString(conn.Name) > 64 ||
		conn.Organization == "" || utf8.RuneCountInString(conn.Organization) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(conn.AppOwner)) > 64 ||
		conn.ClientId == "" || utf8.RuneCountInString(conn.ClientId) > 128 ||
		utf8.RuneCountInString(conn.ServerURL) > 255 ||
		(conn.TLSVerify != -1 && conn.TLSVerify != 0 && conn.TLSVerify != 1) ||
		conn.TimeoutSec < 1 || conn.TimeoutSec > 120 || conn.SyncIntervalSec < 60 {
		return false
	}
	parsed, err := url.Parse(conn.ServerURL)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" &&
		parsed.Fragment == "" && parsed.Path == ""
}
