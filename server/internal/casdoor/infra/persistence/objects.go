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

type objectSnapshotRepository struct{ db *gorm.DB }

func NewObjectSnapshotRepository(db *gorm.DB) repository.ObjectSnapshots {
	return &objectSnapshotRepository{db: db}
}

type objectSyncStateRepository struct{ db *gorm.DB }

func NewObjectSyncStateRepository(db *gorm.DB) repository.ObjectSyncStates {
	return &objectSyncStateRepository{db: db}
}

func (r *objectSyncStateRepository) RecordObjectSyncState(ctx context.Context, id uint64, kind string, category repository.SyncCategory) error {
	if id == 0 || (kind != "role" && kind != "group" && kind != "permission") {
		return fmt.Errorf("invalid object sync state")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state entity.ObjectSyncState
		err := tx.Where("connection_id = ? AND kind = ?", id, kind).First(&state).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			state = entity.ObjectSyncState{ConnectionId: id, Kind: kind}
			state.FillBaseInfo(nil)
		} else if err != nil {
			return err
		}
		state.Status, state.AttemptAt, state.ReasonCode = category.Status, &category.AttemptAt, category.ReasonCode
		if category.Status == "success" {
			state.SuccessAt, state.Count = category.SuccessAt, category.Count
		} else if state.SuccessAt != nil {
			state.Status = "stale"
		}
		return tx.Save(&state).Error
	})
}

func (r *objectSnapshotRepository) ReplaceObjects(ctx context.Context, connectionID uint64, kind string, rows []entity.DirectoryObject, at time.Time) error {
	if connectionID == 0 || (kind != "role" && kind != "group" && kind != "permission") {
		return fmt.Errorf("invalid object snapshot")
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ConnectionId != connectionID || row.Kind != kind || row.SourceId == "" || seen[row.SourceId] {
			return fmt.Errorf("invalid object snapshot identity")
		}
		seen[row.SourceId] = true
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.DirectoryObject{}).Where("connection_id = ? AND kind = ? AND is_deleted = 0", connectionID, kind).Updates(map[string]any{"present": -1}).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var current entity.DirectoryObject
			err := tx.Where("connection_id = ? AND kind = ? AND source_id = ?", connectionID, kind, row.SourceId).First(&current).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				row.Model = model.Model{}
				row.FillBaseInfo(nil)
				row.Present = 1
				row.LastSyncAt = &at
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if err := tx.Model(&entity.DirectoryObject{}).Where("id = ?", current.Id).Updates(map[string]any{"projection": row.Projection, "fingerprint": row.Fingerprint, "present": 1, "last_sync_at": at, "is_deleted": 0, "delete_time": nil}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
