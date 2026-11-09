package persistence

import (
	"context"
	"fmt"

	"mayfly-go/internal/casdoor/domain/entity"

	"gorm.io/gorm"
)

type OperationQuery struct{ db *gorm.DB }

func NewOperationQuery(db *gorm.DB) *OperationQuery { return &OperationQuery{db: db} }

func (q *OperationQuery) ListOperations(ctx context.Context, connectionID uint64, page, size int) ([]entity.Change, int64, error) {
	if connectionID == 0 || page < 1 || size < 1 || size > 100 {
		return nil, 0, fmt.Errorf("invalid operation query")
	}
	db := q.db.WithContext(ctx).Model(&entity.Change{}).Where("connection_id = ? AND is_deleted = 0 AND submitted_at IS NOT NULL", connectionID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []entity.Change
	if err := db.Order("submitted_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
