package application_test

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"mayfly-go/internal/casdoor/application"
	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/infra/persistence"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/model"
	"testing"
)

type serviceCodec struct{}

func (serviceCodec) Encrypt(s string) (string, error) { return "encrypted:" + s, nil }
func (serviceCodec) Decrypt(s string) (string, error) { return s, nil }

type serviceGateway struct{ calls int }

func (g *serviceGateway) Validate(ctx context.Context, c directory.ProbeConfig) ([]directory.Organization, error) {
	g.calls++
	return []directory.Organization{{Name: "built-in", SourceID: "admin/built-in"}, {Name: "org", SourceID: "admin/org"}}, nil
}
func (g *serviceGateway) Reader(ctx context.Context, c directory.ProbeConfig, org string) (directory.Reader, error) {
	return nil, nil
}

func TestServiceSaveRequiresAdminAndDoesNotTrustClientOrganization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&entity.Service{}, &entity.ManagementCredential{}, &entity.ServiceOrganization{}, &entity.ServiceScope{}, &entity.Connection{}, &entity.ObjectSyncState{}))
	gateway := &serviceGateway{}
	manager := application.NewServiceManager(persistence.NewServiceRepository(db), serviceCodec{}, gateway, application.ServiceDependencies{})
	input := application.ServiceInput{Name: "Casdoor", ServerURL: "http://casdoor.test", ClientID: "management", ClientSecret: "secret", TLSVerify: 1}
	_, err = manager.Save(context.Background(), input)
	require.Error(t, err)
	require.Zero(t, gateway.calls)
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 1})
	id, err := manager.Save(ctx, input)
	require.NoError(t, err)
	require.NotZero(t, id)
	orgs, err := manager.Organizations(ctx, id)
	require.NoError(t, err)
	require.Len(t, orgs, 2)
	_, err = manager.Organization(ctx, id+1, orgs[0].Id, false)
	require.Error(t, err)
	input.ID = id
	input.ServerURL = "http://other.test"
	_, err = manager.Save(ctx, input)
	require.Error(t, err)
}
