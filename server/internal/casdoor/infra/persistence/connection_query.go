package persistence

import (
	"context"
	"fmt"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/pkg/consts"

	"gorm.io/gorm"
)

type ConnectionQuery struct{ db *gorm.DB }

func NewConnectionQuery(db *gorm.DB) *ConnectionQuery { return &ConnectionQuery{db: db} }

func (r *ConnectionQuery) ListVisibleConnections(ctx context.Context, accountID uint64) ([]entity.Connection, error) {
	if accountID == 0 {
		return nil, fmt.Errorf("account required")
	}
	query := r.db.WithContext(ctx).Where("t_casdoor_connection.is_deleted = 0 AND t_casdoor_connection.service_id = 0")
	if accountID != consts.AdminId {
		subquery := r.db.Model(&entity.ConnectionScope{}).Select("connection_id").
			Where("account_id = ? AND is_deleted = 0 AND scope IN ?", accountID, []string{"read", "manage"})
		query = query.Where("t_casdoor_connection.id IN (?)", subquery)
	}
	var rows []entity.Connection
	if err := query.Order("t_casdoor_connection.id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
