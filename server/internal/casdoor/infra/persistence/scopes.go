package persistence

import (
	"context"
	"errors"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"

	"gorm.io/gorm"
)

type scopeRepository struct {
	db *gorm.DB
}

func NewScopeRepository(db *gorm.DB) repository.Scopes {
	return &scopeRepository{db: db}
}

func (r *scopeRepository) GetScope(ctx context.Context, accountID, connectionID uint64) (*entity.ConnectionScope, error) {
	var scope entity.ConnectionScope
	err := r.db.WithContext(ctx).
		Where("account_id = ? AND connection_id = ? AND is_deleted = 0", accountID, connectionID).
		First(&scope).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scope, nil
}
