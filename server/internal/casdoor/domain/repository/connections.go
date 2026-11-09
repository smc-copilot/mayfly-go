package repository

import (
	"context"

	"mayfly-go/internal/casdoor/domain/entity"
)

type Connections interface {
	GetConnection(ctx context.Context, id uint64) (*entity.Connection, error)
	SaveConnection(ctx context.Context, connection *entity.Connection) error
}

type Secrets interface {
	Encrypt(value string) (string, error)
	Decrypt(value string) (string, error)
}
