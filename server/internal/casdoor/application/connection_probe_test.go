package application

import (
	"context"
	"strings"
	"testing"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
)

type fakeProbe struct{}

func (fakeProbe) TestService(context.Context) error { return nil }
func (fakeProbe) TestApps(context.Context) error    { return nil }
func (fakeProbe) TestUsers(context.Context) error   { return nil }

type fakeProbeFactory struct {
	calls  int
	config directory.ProbeConfig
}

func (f *fakeProbeFactory) NewProbe(config directory.ProbeConfig) directory.Prober {
	f.calls++
	f.config = config
	return fakeProbe{}
}

func TestProbeUsesSavedCredentialWithoutReturningIt(t *testing.T) {
	original := &entity.Connection{
		Name: "test", ServerURL: "http://casdoor.test", Organization: "org",
		ClientId: "client", ClientSecretEnc: "fixture-secret", CredentialConfigured: 1,
	}
	original.SetId(12)
	factory := &fakeProbeFactory{}
	service := NewConnectionTester(&fakeConnections{existing: original}, fakeSecrets{}, NewScopeAuthorizer(nil), factory)
	candidate := &entity.Connection{Name: "test", ServerURL: original.ServerURL, Organization: "org", ClientId: "client"}
	candidate.SetId(12)
	result, err := service.Test(adminContext(), candidate, "")
	if err != nil || result.Service.Status != "ok" || factory.config.ClientSecret != "fixture-secret" {
		t.Fatalf("应使用保存的凭证测试: result=%+v err=%v", result, err)
	}
	if strings.Contains(result.Service.ReasonCode+result.Apps.ReasonCode+result.Users.ReasonCode, "fixture-secret") {
		t.Fatal("测试结果不得回显凭证")
	}
}

func TestProbeRejectsChangedTargetBeforeClientCreation(t *testing.T) {
	original := &entity.Connection{
		Name: "test", ServerURL: "http://casdoor.test", Organization: "org",
		ClientId: "client", ClientSecretEnc: "fixture-secret", CredentialConfigured: 1,
	}
	original.SetId(12)
	factory := &fakeProbeFactory{}
	service := NewConnectionTester(&fakeConnections{existing: original}, fakeSecrets{}, NewScopeAuthorizer(nil), factory)
	candidate := &entity.Connection{Name: "test", ServerURL: "http://other.test", Organization: "org", ClientId: "client"}
	candidate.SetId(12)
	if _, err := service.Test(adminContext(), candidate, ""); err == nil || factory.calls != 0 {
		t.Fatal("改变服务地址必须在创建客户端前拒绝")
	}
}
