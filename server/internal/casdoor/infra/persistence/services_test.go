package persistence

import (
	"context"
	"github.com/stretchr/testify/require"
	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestServiceDiscoveryPreservesLegacySnapshotContext(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.Service{}, &entity.ManagementCredential{}, &entity.ServiceOrganization{}, &entity.ServiceScope{}, &entity.Connection{}, &entity.ObjectSyncState{}))
	old := &entity.Connection{Name: "old-app", ServerURL: "http://casdoor.test", Organization: "org", ClientId: "old", ClientSecretEnc: "keep-old"}
	old.FillBaseInfo(nil)
	require.NoError(t, db.Create(old).Error)
	svc := &entity.Service{Name: "Casdoor", ServerURL: "http://casdoor.test", TimeoutSec: 15, SyncIntervalSec: 3600, TLSVerify: 1}
	cred := &entity.ManagementCredential{ClientID: "management", SecretEnc: "encrypted"}
	repo := NewServiceRepository(db)
	require.NoError(t, repo.Save(context.Background(), svc, cred, []directory.Organization{{Name: "org", SourceID: "admin/org"}, {Name: "built-in", SourceID: "admin/built-in"}}, old.Id))
	orgs, err := repo.Organizations(context.Background(), svc.Id)
	require.NoError(t, err)
	require.Len(t, orgs, 2)
	for _, org := range orgs {
		if org.Name == "org" {
			require.Equal(t, old.Id, org.ConnectionID)
		}
	}
	require.NoError(t, db.First(old, old.Id).Error)
	require.Equal(t, "keep-old", old.ClientSecretEnc)
	require.Equal(t, svc.Id, old.ServiceID)
	other := &entity.ServiceOrganization{}
	_, err = repo.Organization(context.Background(), svc.Id+1, orgs[0].Id)
	require.Error(t, err)
	_ = other
}

func TestServiceSaveRejectsStaleCredentialAndBoundsInternalContextName(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.Service{}, &entity.ManagementCredential{}, &entity.ServiceOrganization{}, &entity.Connection{}))
	repo := NewServiceRepository(db)
	ctx := context.Background()
	svc := &entity.Service{Name: strings.Repeat("服", 64), ServerURL: "http://casdoor.test"}
	cred := &entity.ManagementCredential{ClientID: "management", SecretEnc: "old"}
	orgs := []directory.Organization{{Name: "built-in"}}
	require.NoError(t, repo.Save(ctx, svc, cred, orgs, 0))
	var internal entity.Connection
	require.NoError(t, db.First(&internal).Error)
	require.LessOrEqual(t, utf8.RuneCountInString(internal.Name), 64)
	staleService, err := repo.Get(ctx, svc.Id)
	require.NoError(t, err)
	staleCredential, err := repo.Credential(ctx, svc.Id)
	require.NoError(t, err)
	staleService.CredentialRevision = staleCredential.Revision
	cred.SecretEnc = "rotated"
	require.NoError(t, repo.Save(ctx, svc, cred, orgs, 0))
	require.Error(t, repo.Save(ctx, staleService, staleCredential, orgs, 0))
	actual, err := repo.Credential(ctx, svc.Id)
	require.NoError(t, err)
	require.Equal(t, "rotated", actual.SecretEnc)
	require.Error(t, repo.Discover(ctx, staleService, orgs))
	require.Error(t, repo.Failure(ctx, svc.Id, staleService.CredentialRevision, "obsolete_failure"))
}
