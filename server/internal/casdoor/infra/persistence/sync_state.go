package persistence

import (
	"context"
	"fmt"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"

	"gorm.io/gorm"
)

type syncStateRepository struct{ db *gorm.DB }

func NewSyncStateRepository(db *gorm.DB) repository.SyncStateRecorder {
	return &syncStateRepository{db: db}
}

func (r *syncStateRepository) RecordSyncState(ctx context.Context, id uint64, state repository.SyncState) error {
	if id == 0 {
		return fmt.Errorf("invalid connection")
	}
	updates := map[string]any{}
	addCategoryUpdates(updates, "app", state.Applications)
	addCategoryUpdates(updates, "user", state.Users)
	result := r.db.WithContext(ctx).Model(&entity.Connection{}).Where("id = ? AND is_deleted = 0", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func addCategoryUpdates(updates map[string]any, prefix string, category repository.SyncCategory) {
	updates[prefix+"_sync_status"] = category.Status
	updates[prefix+"_sync_at"] = category.AttemptAt
	updates[prefix+"_sync_error"] = category.ReasonCode
	if category.Status == "success" {
		updates[prefix+"_success_at"] = category.SuccessAt
		updates[prefix+"_sync_count"] = category.Count
	}
}
