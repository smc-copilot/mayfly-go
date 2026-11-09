package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/pkg/model"

	"gorm.io/gorm"
)

type snapshotRepository struct {
	db *gorm.DB
}

func NewSnapshotRepository(db *gorm.DB) repository.Snapshots {
	return &snapshotRepository{db: db}
}

func (r *snapshotRepository) ReplaceApplications(ctx context.Context, connectionID uint64, rows []entity.Application, at time.Time) error {
	return r.saveApplications(ctx, connectionID, rows, at, true)
}

// MergeApplications 用于受限范围的读取，不对未返回的应用作不存在判断。
func (r *snapshotRepository) MergeApplications(ctx context.Context, connectionID uint64, rows []entity.Application, at time.Time) error {
	return r.saveApplications(ctx, connectionID, rows, at, false)
}

func (r *snapshotRepository) saveApplications(ctx context.Context, connectionID uint64, rows []entity.Application, at time.Time, complete bool) error {
	if connectionID == 0 {
		return fmt.Errorf("invalid application snapshot connection")
	}
	ids := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.ConnectionId != connectionID || row.SourceId == "" {
			return fmt.Errorf("invalid application snapshot connection or source id")
		}
		if _, exists := ids[row.SourceId]; exists {
			return fmt.Errorf("duplicate application snapshot source id")
		}
		ids[row.SourceId] = struct{}{}
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if complete {
			if err := tx.Model(&entity.Application{}).
				Where("connection_id = ? AND is_deleted = 0", connectionID).
				Updates(map[string]any{"present": -1, "last_sync_at": at}).Error; err != nil {
				return err
			}
		}
		for _, row := range rows {
			if err := saveApplication(tx, row, at); err != nil {
				return err
			}
		}
		return nil
	})
}

func saveApplication(tx *gorm.DB, row entity.Application, at time.Time) error {
	var current entity.Application
	err := tx.Where("connection_id = ? AND source_id = ?", row.ConnectionId, row.SourceId).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row.Model = model.Model{}
		row.FillBaseInfo(nil)
		row.Present = 1
		row.LastSyncAt = &at
		return tx.Create(&row).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&entity.Application{}).Where("id = ?", current.Id).Updates(map[string]any{
		"name": row.Name, "display_name": row.DisplayName, "organization": row.Organization,
		"app_owner": row.AppOwner, "client_id": row.ClientId, "enabled": row.Enabled,
		"app_type": row.AppType, "homepage_url": row.HomepageURL, "present": 1,
		"last_sync_at": at, "is_deleted": 0, "delete_time": nil,
	}).Error
}

func (r *snapshotRepository) ReplaceUsers(ctx context.Context, connectionID uint64, rows []entity.User, at time.Time) error {
	if connectionID == 0 {
		return fmt.Errorf("invalid user snapshot connection")
	}
	ids := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.ConnectionId != connectionID || row.SourceId == "" {
			return fmt.Errorf("invalid user snapshot connection or source id")
		}
		if _, exists := ids[row.SourceId]; exists {
			return fmt.Errorf("duplicate user snapshot source id")
		}
		ids[row.SourceId] = struct{}{}
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.User{}).
			Where("connection_id = ? AND is_deleted = 0", connectionID).
			Updates(map[string]any{"present": -1, "last_sync_at": at}).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := saveUser(tx, row, at); err != nil {
				return err
			}
		}
		return nil
	})
}

func saveUser(tx *gorm.DB, row entity.User, at time.Time) error {
	var current entity.User
	err := tx.Where("connection_id = ? AND source_id = ?", row.ConnectionId, row.SourceId).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row.Model = model.Model{}
		row.FillBaseInfo(nil)
		row.Present = 1
		row.LastSyncAt = &at
		return tx.Create(&row).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&entity.User{}).Where("id = ?", current.Id).Updates(map[string]any{
		"object_id": row.ObjectId, "username": row.Username, "display_name": row.DisplayName,
		"organization": row.Organization, "email": row.Email, "enabled": row.Enabled,
		"source_updated_at": row.SourceUpdatedAt, "present": 1, "last_sync_at": at,
		"is_deleted": 0, "delete_time": nil,
	}).Error
}
