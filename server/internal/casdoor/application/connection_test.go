package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/model"
)

type fakeConnections struct {
	existing *entity.Connection
	saved    *entity.Connection
}

func TestSaveConnectionRejectsInvalidOwnerAndTLSSetting(t *testing.T) {
	repo := &fakeConnections{}
	service := NewConnectionService(repo, fakeSecrets{}, NewScopeAuthorizer(nil))
	conn := &entity.Connection{Name: "test", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "client", AppOwner: strings.Repeat("a", 65), TLSVerify: 1}
	if err := service.Save(adminContext(), conn, "secret", nil); err == nil {
		t.Fatal("Owner 超过字段长度必须拒绝")
	}
	conn.AppOwner = "admin"
	conn.TLSVerify = 2
	if err := service.Save(adminContext(), conn, "secret", nil); err == nil {
		t.Fatal("不支持的 TLS 设置必须拒绝")
	}
}

func (f *fakeConnections) GetConnection(_ context.Context, id uint64) (*entity.Connection, error) {
	if f.existing == nil || f.existing.Id != id {
		return nil, nil
	}
	copied := *f.existing
	return &copied, nil
}

func (f *fakeConnections) SaveConnection(_ context.Context, conn *entity.Connection) error {
	copied := *conn
	f.saved = &copied
	return nil
}

type fakeSecrets struct{}

func (fakeSecrets) Encrypt(value string) (string, error) { return "encrypted:" + value, nil }
func (fakeSecrets) Decrypt(value string) (string, error) { return value, nil }

func adminContext() context.Context {
	return contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 1})
}

func TestSaveConnectionKeepsSavedSecretOnSameTarget(t *testing.T) {
	original := &entity.Connection{
		Name: "old", ServerURL: "http://casdoor.test", Organization: "org",
		ClientId: "client", ClientSecretEnc: "encrypted:old", CredentialConfigured: 1,
		LastSyncStatus: "success",
	}
	original.SetId(12)
	repo := &fakeConnections{existing: original}
	service := NewConnectionService(repo, fakeSecrets{}, NewScopeAuthorizer(nil))
	proposal := &entity.Connection{Name: "new", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "client"}
	proposal.SetId(12)
	if err := service.Save(adminContext(), proposal, "", nil); err != nil {
		t.Fatal(err)
	}
	if repo.saved == nil || repo.saved.ClientSecretEnc != "encrypted:old" || repo.saved.LastSyncStatus != "success" {
		t.Fatalf("编辑时应保留密钥和同步状态: %+v", repo.saved)
	}
}

func TestSaveConnectionRejectsRetargetAndRequiresNewClientSecret(t *testing.T) {
	original := &entity.Connection{
		Name: "old", ServerURL: "http://casdoor.test", Organization: "org",
		ClientId: "client", ClientSecretEnc: "encrypted:old", CredentialConfigured: 1,
	}
	original.SetId(12)
	repo := &fakeConnections{existing: original}
	service := NewConnectionService(repo, fakeSecrets{}, NewScopeAuthorizer(nil))
	proposal := &entity.Connection{Name: "new", ServerURL: "http://other.test", Organization: "org", ClientId: "client"}
	proposal.SetId(12)
	if err := service.Save(adminContext(), proposal, "new-secret", nil); err == nil || repo.saved != nil {
		t.Fatal("已有连接不能改指向")
	}
	proposal.ServerURL = original.ServerURL
	proposal.ClientId = "other-client"
	if err := service.Save(adminContext(), proposal, "", nil); err == nil || repo.saved != nil {
		t.Fatal("变更 Client ID 必须重新输入密钥")
	}
}

func TestSaveConnectionRejectsInvalidURLAndUnauthenticatedCreation(t *testing.T) {
	repo := &fakeConnections{}
	service := NewConnectionService(repo, fakeSecrets{}, NewScopeAuthorizer(nil))
	proposal := &entity.Connection{Name: "test", ServerURL: "http://name:password@casdoor.test", Organization: "org", ClientId: "client"}
	if err := service.Save(adminContext(), proposal, "secret", nil); err == nil {
		t.Fatal("服务地址不能包含用户信息")
	}
	proposal.ServerURL = "http://casdoor.test"
	if err := service.Save(context.Background(), proposal, "secret", nil); err == nil || repo.saved != nil {
		t.Fatal("没有账号不能创建连接")
	}
}

func TestSaveConnectionPropagatesRepositoryFailure(t *testing.T) {
	expected := errors.New("storage failure")
	repo := &failingConnections{err: expected}
	service := NewConnectionService(repo, fakeSecrets{}, NewScopeAuthorizer(nil))
	conn := &entity.Connection{Name: "test", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "client"}
	if err := service.Save(adminContext(), conn, "secret", nil); !errors.Is(err, expected) {
		t.Fatalf("存储错误不能丢失: %v", err)
	}
}

func TestSaveConnectionDistinguishesOmittedAndDisabledManagement(t *testing.T) {
	original := &entity.Connection{
		Name: "old", ServerURL: "http://casdoor.test", Organization: "org",
		ClientId: "client", ClientSecretEnc: "encrypted:old", CredentialConfigured: 1,
		ManagementEnabled: true,
	}
	original.SetId(12)
	repo := &fakeConnections{existing: original}
	service := NewConnectionService(repo, fakeSecrets{}, NewScopeAuthorizer(nil))
	proposal := &entity.Connection{Name: "old", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "client"}
	proposal.SetId(12)
	if err := service.Save(adminContext(), proposal, "", nil); err != nil || !repo.saved.ManagementEnabled {
		t.Fatalf("未提交开关时应保留原值: %v", err)
	}
	disable := false
	if err := service.Save(adminContext(), proposal, "", &disable); err != nil || repo.saved.ManagementEnabled {
		t.Fatalf("显式关闭必须保存 false: %v", err)
	}
}

func TestSecretRotationDisablesUnverifiedManagement(t *testing.T) {
	original := &entity.Connection{
		Name: "old", ServerURL: "http://casdoor.test", Organization: "org",
		ClientId: "client", ClientSecretEnc: "encrypted:old", CredentialConfigured: 1,
		ManagementEnabled: true,
	}
	original.SetId(12)
	repo := &fakeConnections{existing: original}
	service := NewConnectionService(repo, fakeSecrets{}, NewScopeAuthorizer(nil))
	proposal := &entity.Connection{Name: "old", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "client"}
	proposal.SetId(12)
	if err := service.Save(adminContext(), proposal, "new-secret", nil); err != nil {
		t.Fatal(err)
	}
	if repo.saved.ManagementEnabled || repo.saved.ClientSecretEnc != "encrypted:new-secret" {
		t.Fatal("轮换凭证后必须重新核验写入能力")
	}
}

type failingConnections struct{ err error }

func (f *failingConnections) GetConnection(context.Context, uint64) (*entity.Connection, error) {
	return nil, f.err
}
func (f *failingConnections) SaveConnection(context.Context, *entity.Connection) error {
	return f.err
}
