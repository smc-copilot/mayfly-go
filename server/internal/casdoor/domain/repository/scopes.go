package repository

import (
	"context"

	"mayfly-go/internal/casdoor/domain/entity"
)

// Scopes 查询 Mayfly 账号对 Casdoor 连接的可见范围。
type Scopes interface {
	GetScope(ctx context.Context, accountID, connectionID uint64) (*entity.ConnectionScope, error)
}
