package application

import (
	"context"
	"fmt"
	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/internal/casdoor/imsg"
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

type ManagementGateway interface {
	Validate(context.Context, directory.ProbeConfig) ([]directory.Organization, error)
	Reader(context.Context, directory.ProbeConfig, string) (directory.Reader, error)
}
type ServiceDependencies struct {
	Locks       *ConnectionLocks
	Snapshots   repository.Snapshots
	State       repository.SyncStateRecorder
	Objects     repository.ObjectSnapshots
	ObjectState repository.ObjectSyncStates
	Query       ApplicationQuery
	Operations  OperationQuery
	Connections repository.Connections
}
type ServiceManager struct {
	store   repository.ServiceStore
	secrets repository.Secrets
	gateway ManagementGateway
	deps    ServiceDependencies
}

func NewServiceManager(store repository.ServiceStore, secrets repository.Secrets, gateway ManagementGateway, deps ServiceDependencies) *ServiceManager {
	return &ServiceManager{store: store, secrets: secrets, gateway: gateway, deps: deps}
}

type ServiceInput struct {
	ID                 uint64 `json:"id"`
	Name               string `json:"name"`
	ServerURL          string `json:"serverUrl"`
	ClientID           string `json:"clientId"`
	ClientSecret       string `json:"clientSecret"`
	TimeoutSec         int    `json:"timeoutSec"`
	SyncIntervalSec    int    `json:"syncIntervalSec"`
	TLSVerify          int8   `json:"tlsVerify"`
	LegacyConnectionID uint64 `json:"legacyConnectionId"`
}
type ServiceView struct {
	entity.Service
	ClientID          string `json:"clientId"`
	AuthType          string `json:"authType"`
	OrganizationCount int    `json:"organizationCount"`
}

func serviceError(ctx context.Context) error { return errorx.NewBizI(ctx, imsg.ErrServiceInvalid) }
func (s *ServiceManager) assert(ctx context.Context, id uint64, write bool) error {
	account := contextx.GetLoginAccount(ctx)
	if account == nil || account.Id == 0 || id == 0 {
		return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
	}
	if account.Id == consts.AdminId {
		return nil
	}
	scope, err := s.store.Scope(ctx, account.Id, id)
	if err != nil {
		return err
	}
	if scope == "manage" || (!write && scope == "read") {
		return nil
	}
	return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
}
func requireServiceAdmin(ctx context.Context) error {
	account := contextx.GetLoginAccount(ctx)
	if account == nil || account.Id != consts.AdminId {
		return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
	}
	return nil
}
func (s *ServiceManager) List(ctx context.Context) ([]ServiceView, error) {
	actor := contextx.GetLoginAccount(ctx)
	if actor == nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
	}
	rows, err := s.store.List(ctx, actor.Id)
	if err != nil {
		return nil, err
	}
	out := make([]ServiceView, 0, len(rows))
	for _, row := range rows {
		cred, err := s.store.Credential(ctx, row.Id)
		if err != nil {
			return nil, err
		}
		orgs, err := s.store.Organizations(ctx, row.Id)
		if err != nil {
			return nil, err
		}
		out = append(out, ServiceView{Service: row, ClientID: cred.ClientID, AuthType: "client_credentials", OrganizationCount: len(orgs)})
	}
	return out, nil
}
func normalizeService(ctx context.Context, input ServiceInput) (ServiceInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ServerURL = strings.TrimRight(strings.TrimSpace(input.ServerURL), "/")
	input.ClientID = strings.TrimSpace(input.ClientID)
	if input.TimeoutSec == 0 {
		input.TimeoutSec = 15
	}
	if input.SyncIntervalSec == 0 {
		input.SyncIntervalSec = 3600
	}
	if input.TLSVerify == 0 {
		input.TLSVerify = 1
	}
	u, err := url.Parse(input.ServerURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || utf8.RuneCountInString(input.ServerURL) > 255 || input.Name == "" || utf8.RuneCountInString(input.Name) > 64 || input.ClientID == "" || len(input.ClientID) > 128 || len(input.ClientSecret) > 256 || input.TimeoutSec < 1 || input.TimeoutSec > 120 || input.SyncIntervalSec < 60 || (input.TLSVerify != 1 && input.TLSVerify != -1) {
		return input, serviceError(ctx)
	}
	return input, nil
}
func (s *ServiceManager) candidate(ctx context.Context, input ServiceInput) (*entity.Service, *entity.ManagementCredential, directory.ProbeConfig, error) {
	if err := requireServiceAdmin(ctx); err != nil {
		return nil, nil, directory.ProbeConfig{}, err
	}
	input, err := normalizeService(ctx, input)
	if err != nil {
		return nil, nil, directory.ProbeConfig{}, err
	}
	svc := &entity.Service{}
	cred := &entity.ManagementCredential{}
	if input.ID != 0 {
		svc, err = s.store.Get(ctx, input.ID)
		if err != nil {
			return nil, nil, directory.ProbeConfig{}, err
		}
		if svc.ServerURL != input.ServerURL || input.LegacyConnectionID != 0 {
			return nil, nil, directory.ProbeConfig{}, serviceError(ctx)
		}
		cred, err = s.store.Credential(ctx, input.ID)
		if err != nil {
			return nil, nil, directory.ProbeConfig{}, err
		}
		if cred.ClientID != input.ClientID && input.ClientSecret == "" {
			return nil, nil, directory.ProbeConfig{}, errorx.NewBizI(ctx, imsg.ErrConnectionSecretRequired)
		}
	}
	secret := input.ClientSecret
	if secret == "" {
		if cred.SecretEnc == "" {
			return nil, nil, directory.ProbeConfig{}, errorx.NewBizI(ctx, imsg.ErrConnectionSecretRequired)
		}
		secret, err = s.secrets.Decrypt(cred.SecretEnc)
		if err != nil {
			return nil, nil, directory.ProbeConfig{}, serviceError(ctx)
		}
	}
	svc.Name, svc.ServerURL, svc.TimeoutSec, svc.SyncIntervalSec, svc.TLSVerify = input.Name, input.ServerURL, input.TimeoutSec, input.SyncIntervalSec, input.TLSVerify
	cred.ClientID = input.ClientID
	cfg := directory.ProbeConfig{ServerURL: svc.ServerURL, ClientID: input.ClientID, ClientSecret: secret, TLSVerify: svc.TLSVerify != -1, Timeout: time.Duration(svc.TimeoutSec) * time.Second}
	return svc, cred, cfg, nil
}
func (s *ServiceManager) Test(ctx context.Context, input ServiceInput) ([]directory.Organization, error) {
	_, _, cfg, err := s.candidate(ctx, input)
	if err != nil {
		return nil, err
	}
	orgs, err := s.gateway.Validate(ctx, cfg)
	if err != nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrServiceProbeFailed)
	}
	return orgs, nil
}
func (s *ServiceManager) Save(ctx context.Context, input ServiceInput) (uint64, error) {
	svc, cred, cfg, err := s.candidate(ctx, input)
	if err != nil {
		return 0, err
	}
	orgs, err := s.gateway.Validate(ctx, cfg)
	if err != nil {
		return 0, errorx.NewBizI(ctx, imsg.ErrServiceProbeFailed)
	}
	if input.ClientSecret != "" || cred.SecretEnc == "" {
		cred.SecretEnc, err = s.secrets.Encrypt(cfg.ClientSecret)
		if err != nil {
			return 0, err
		}
	}
	if err := s.store.Save(ctx, svc, cred, orgs, input.LegacyConnectionID); err != nil {
		return 0, errorx.NewBizI(ctx, imsg.ErrServiceSaveFailed)
	}
	return svc.Id, nil
}
func (s *ServiceManager) config(ctx context.Context, id uint64) (*entity.Service, directory.ProbeConfig, error) {
	svc, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, directory.ProbeConfig{}, err
	}
	cred, err := s.store.Credential(ctx, id)
	if err != nil {
		return nil, directory.ProbeConfig{}, err
	}
	svc.CredentialRevision = cred.Revision
	secret, err := s.secrets.Decrypt(cred.SecretEnc)
	if err != nil {
		return svc, directory.ProbeConfig{}, serviceError(ctx)
	}
	return svc, directory.ProbeConfig{ServerURL: svc.ServerURL, ClientID: cred.ClientID, ClientSecret: secret, Timeout: time.Duration(svc.TimeoutSec) * time.Second, TLSVerify: svc.TLSVerify != -1}, nil
}
func (s *ServiceManager) Discover(ctx context.Context, id uint64) ([]entity.ServiceOrganization, error) {
	if err := s.assert(ctx, id, true); err != nil {
		return nil, err
	}
	svc, cfg, err := s.config(ctx, id)
	if err != nil {
		return nil, err
	}
	orgs, err := s.gateway.Validate(ctx, cfg)
	if err != nil {
		if saveErr := s.store.Failure(ctx, id, svc.CredentialRevision, "management_probe_failed"); saveErr != nil {
			return nil, saveErr
		}
		return nil, errorx.NewBizI(ctx, imsg.ErrServiceProbeFailed)
	}
	if err := s.store.Discover(ctx, svc, orgs); err != nil {
		return nil, err
	}
	return s.store.Organizations(ctx, id)
}
func (s *ServiceManager) Organizations(ctx context.Context, id uint64) ([]entity.ServiceOrganization, error) {
	if err := s.assert(ctx, id, false); err != nil {
		return nil, err
	}
	return s.store.Organizations(ctx, id)
}
func (s *ServiceManager) Organization(ctx context.Context, service, id uint64, write bool) (*entity.ServiceOrganization, error) {
	if err := s.assert(ctx, service, write); err != nil {
		return nil, err
	}
	org, err := s.store.Organization(ctx, service, id)
	if err != nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrServiceOrganizationInvalid)
	}
	return org, nil
}
func (s *ServiceManager) Sync(ctx context.Context, service, id uint64) (*DirectorySyncResult, error) {
	org, err := s.Organization(ctx, service, id, true)
	if err != nil {
		return nil, err
	}
	return s.syncOrganization(ctx, org)
}
func (s *ServiceManager) syncOrganization(ctx context.Context, org *entity.ServiceOrganization) (*DirectorySyncResult, error) {
	if s.deps.Locks == nil || s.deps.Connections == nil || s.deps.Snapshots == nil || s.deps.State == nil || s.deps.Objects == nil || s.deps.ObjectState == nil {
		return nil, fmt.Errorf("service_sync_unavailable")
	}
	release, err := s.deps.Locks.Acquire(ctx, org.ConnectionID)
	if err != nil {
		return nil, err
	}
	defer release()
	conn, err := s.deps.Connections.GetConnection(ctx, org.ConnectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil || conn.ServiceID != org.ServiceID || conn.Organization != org.Name {
		return nil, errorx.NewBizI(ctx, imsg.ErrServiceOrganizationInvalid)
	}
	svc, cfg, err := s.config(ctx, org.ServiceID)
	if err != nil {
		if svc == nil {
			return nil, err
		}
		return nil, s.recordAuthenticationFailure(ctx, conn, svc.CredentialRevision)
	}
	reader, err := s.gateway.Reader(ctx, cfg, org.Name)
	if err != nil {
		return nil, s.recordAuthenticationFailure(ctx, conn, svc.CredentialRevision)
	}
	// The legacy context holds only snapshot identity; never use its old owner or credential.
	conn.AppOwner = ""
	result := SyncDirectory(ctx, conn, reader, s.deps.Snapshots, 100)
	if err := s.deps.State.RecordSyncState(ctx, conn.Id, repository.SyncState{Applications: syncCategoryState(result.Apps), Users: syncCategoryState(result.Users)}); err != nil {
		return nil, err
	}
	objectReader, ok := reader.(ObjectReader)
	if !ok {
		return nil, fmt.Errorf("object_reader_unavailable")
	}
	result.Objects = map[string]DirectoryCategoryResult{}
	for _, kind := range []string{"role", "group", "permission"} {
		r := SyncObjectCategory(ctx, conn, objectReader, s.deps.Objects, kind, 100)
		if err := s.deps.ObjectState.RecordObjectSyncState(ctx, conn.Id, kind, syncCategoryState(r)); err != nil {
			return nil, err
		}
		result.Objects[kind] = r
	}
	return &result, nil
}

func (s *ServiceManager) recordAuthenticationFailure(ctx context.Context, conn *entity.Connection, revision uint64) error {
	if err := s.store.Failure(ctx, conn.ServiceID, revision, "management_authentication_failed"); err != nil {
		return err
	}
	failure := fmt.Errorf("management_authentication_failed")
	state := repository.SyncState{Applications: syncCategoryState(categoryFailure(conn.AppSuccessAt, failure)), Users: syncCategoryState(categoryFailure(conn.UserSuccessAt, failure))}
	if err := s.deps.State.RecordSyncState(ctx, conn.Id, state); err != nil {
		return err
	}
	for _, kind := range []string{"role", "group", "permission"} {
		if err := s.deps.ObjectState.RecordObjectSyncState(ctx, conn.Id, kind, syncCategoryState(categoryFailure(nil, failure))); err != nil {
			return err
		}
	}
	return errorx.NewBizI(ctx, imsg.ErrServiceProbeFailed)
}
