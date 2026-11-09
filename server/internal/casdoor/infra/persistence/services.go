package persistence

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/contextx"
	"time"
)

type serviceRepository struct{ db *gorm.DB }

func NewServiceRepository(db *gorm.DB) repository.ServiceStore { return &serviceRepository{db: db} }
func (r *serviceRepository) List(ctx context.Context, account uint64) ([]entity.Service, error) {
	if account == 0 {
		return nil, fmt.Errorf("account_required")
	}
	q := r.db.WithContext(ctx).Where("is_deleted=0")
	if account != consts.AdminId {
		q = q.Where("id IN (?)", r.db.Model(&entity.ServiceScope{}).Select("service_id").Where("account_id=? AND is_deleted=0 AND scope IN ?", account, []string{"read", "manage"}))
	}
	rows := []entity.Service{}
	err := q.Order("name,id").Find(&rows).Error
	return rows, err
}
func (r *serviceRepository) Get(ctx context.Context, id uint64) (*entity.Service, error) {
	row := new(entity.Service)
	err := r.db.WithContext(ctx).Where("id=? AND is_deleted=0", id).First(row).Error
	return row, err
}
func (r *serviceRepository) Credential(ctx context.Context, id uint64) (*entity.ManagementCredential, error) {
	row := new(entity.ManagementCredential)
	err := r.db.WithContext(ctx).Where("service_id=? AND is_deleted=0", id).First(row).Error
	return row, err
}
func (r *serviceRepository) Scope(ctx context.Context, account, service uint64) (string, error) {
	var row entity.ServiceScope
	err := r.db.WithContext(ctx).Where("service_id=? AND account_id=? AND is_deleted=0", service, account).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return row.Scope, err
}
func (r *serviceRepository) Organizations(ctx context.Context, id uint64) ([]entity.ServiceOrganization, error) {
	rows := []entity.ServiceOrganization{}
	err := r.db.WithContext(ctx).Where("service_id=? AND is_deleted=0 AND present=1", id).Order("name,id").Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ConnectionID)
	}
	var connections []entity.Connection
	if err := r.db.WithContext(ctx).Where("id IN ? AND service_id=? AND is_deleted=0", ids, id).Find(&connections).Error; err != nil {
		return nil, err
	}
	states := map[uint64]map[string]entity.DirectoryStatus{}
	for _, conn := range connections {
		states[conn.Id] = map[string]entity.DirectoryStatus{
			"applications": {Status: conn.AppSyncStatus, AttemptAt: conn.AppSyncAt, SuccessAt: conn.AppSuccessAt, Count: conn.AppSyncCount, ReasonCode: conn.AppSyncError},
			"users":        {Status: conn.UserSyncStatus, AttemptAt: conn.UserSyncAt, SuccessAt: conn.UserSuccessAt, Count: conn.UserSyncCount, ReasonCode: conn.UserSyncError},
		}
	}
	var objects []entity.ObjectSyncState
	if err := r.db.WithContext(ctx).Where("connection_id IN ? AND is_deleted=0", ids).Find(&objects).Error; err != nil {
		return nil, err
	}
	for _, state := range objects {
		if states[state.ConnectionId] != nil {
			states[state.ConnectionId][state.Kind] = entity.DirectoryStatus{Status: state.Status, AttemptAt: state.AttemptAt, SuccessAt: state.SuccessAt, Count: state.Count, ReasonCode: state.ReasonCode}
		}
	}
	for i := range rows {
		rows[i].SyncStates = states[rows[i].ConnectionID]
		if rows[i].SyncStates == nil {
			rows[i].SyncStates = map[string]entity.DirectoryStatus{}
		}
		for _, kind := range []string{"applications", "users", "role", "group", "permission"} {
			state := rows[i].SyncStates[kind]
			if state.Status == "" {
				state.Status = "never"
				rows[i].SyncStates[kind] = state
			}
		}
	}
	return rows, nil
}
func (r *serviceRepository) Organization(ctx context.Context, service, id uint64) (*entity.ServiceOrganization, error) {
	row := new(entity.ServiceOrganization)
	err := r.db.WithContext(ctx).Where("id=? AND service_id=? AND is_deleted=0 AND present=1", id, service).First(row).Error
	return row, err
}
func (r *serviceRepository) Failure(ctx context.Context, id, revision uint64, reason string) error {
	result := r.db.WithContext(ctx).Model(&entity.Service{}).Where("id=? AND id IN (?)", id, r.db.Model(&entity.ManagementCredential{}).Select("service_id").Where("service_id=? AND revision=? AND is_deleted=0", id, revision)).Updates(map[string]any{"status": "failed", "reason_code": reason})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("service_configuration_changed")
	}
	return nil
}
func (r *serviceRepository) Save(ctx context.Context, service *entity.Service, credential *entity.ManagementCredential, orgs []directory.Organization, legacyID uint64) error {
	expectedRevision := credential.Revision
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if credential.Id != 0 {
			result := tx.Model(&entity.ManagementCredential{}).Where("id=? AND service_id=? AND revision=? AND is_deleted=0", credential.Id, service.Id, expectedRevision).Updates(map[string]any{"client_id": credential.ClientID, "secret_enc": credential.SecretEnc, "revision": expectedRevision + 1})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("service_configuration_changed")
			}
		}
		service.FillBaseInfo(contextx.GetLoginAccount(ctx))
		service.Status = "verified"
		service.ReasonCode = ""
		now := time.Now().UTC()
		service.VerifiedAt = &now
		if err := tx.Save(service).Error; err != nil {
			return err
		}
		credential.ServiceID = service.Id
		if credential.Id == 0 {
			credential.FillBaseInfo(contextx.GetLoginAccount(ctx))
			credential.Revision = expectedRevision + 1
			if err := tx.Create(credential).Error; err != nil {
				return err
			}
		}
		return saveServiceOrganizations(tx, service, orgs, legacyID)
	})
	if err == nil {
		credential.Revision = expectedRevision + 1
		service.CredentialRevision = credential.Revision
	}
	return err
}
func (r *serviceRepository) Discover(ctx context.Context, service *entity.Service, orgs []directory.Organization) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var credential entity.ManagementCredential
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("service_id=? AND is_deleted=0", service.Id).First(&credential).Error; err != nil {
			return err
		}
		if credential.Revision != service.CredentialRevision {
			return fmt.Errorf("service_configuration_changed")
		}
		if err := saveServiceOrganizations(tx, service, orgs, 0); err != nil {
			return err
		}
		return tx.Model(&entity.Service{}).Where("id=?", service.Id).Updates(map[string]any{"status": "verified", "reason_code": "", "verified_at": time.Now().UTC()}).Error
	})
}
func saveServiceOrganizations(tx *gorm.DB, service *entity.Service, orgs []directory.Organization, legacyID uint64) error {
	seen := map[string]bool{}
	var legacy entity.Connection
	if legacyID != 0 {
		if err := tx.Where("id=? AND is_deleted=0 AND service_id=0", legacyID).First(&legacy).Error; err != nil {
			return err
		}
		if legacy.ServerURL != service.ServerURL {
			return fmt.Errorf("legacy_target_mismatch")
		}
	}
	for _, org := range orgs {
		if org.Name == "" || seen[org.Name] {
			return fmt.Errorf("invalid_organization_identity")
		}
		seen[org.Name] = true
	}
	if legacyID != 0 && !seen[legacy.Organization] {
		return fmt.Errorf("legacy_organization_not_discovered")
	}
	if err := tx.Model(&entity.ServiceOrganization{}).Where("service_id=?", service.Id).Update("present", -1).Error; err != nil {
		return err
	}
	for _, org := range orgs {
		var row entity.ServiceOrganization
		err := tx.Where("service_id=? AND name=?", service.Id, org.Name).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			context := entity.Connection{Name: fmt.Sprintf("casdoor:%d", service.Id), ServerURL: service.ServerURL, Organization: org.Name, ServiceID: service.Id, TimeoutSec: service.TimeoutSec, SyncIntervalSec: service.SyncIntervalSec, TLSVerify: service.TLSVerify}
			if legacyID != 0 && org.Name == legacy.Organization {
				context.Id = legacyID
				if err := tx.Model(&entity.Connection{}).Where("id=?", legacyID).Update("service_id", service.Id).Error; err != nil {
					return err
				}
			} else {
				context.FillBaseInfo(nil)
				if err := tx.Create(&context).Error; err != nil {
					return err
				}
			}
			row = entity.ServiceOrganization{ServiceID: service.Id, Name: org.Name, ConnectionID: context.Id}
			row.FillBaseInfo(nil)
		} else if err != nil {
			return err
		}
		now := time.Now().UTC()
		row.SourceID = org.SourceID
		row.DisplayName = org.DisplayName
		row.Present = 1
		row.LastSyncAt = &now
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
