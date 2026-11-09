package repository

import (
	"context"
	"time"

	"mayfly-go/internal/casdoor/domain/entity"
)

// Snapshots 在完整读取后原子替换单连接的某一类目录快照。
type Snapshots interface {
	ReplaceApplications(ctx context.Context, connectionID uint64, rows []entity.Application, at time.Time) error
	MergeApplications(ctx context.Context, connectionID uint64, rows []entity.Application, at time.Time) error
	ReplaceUsers(ctx context.Context, connectionID uint64, rows []entity.User, at time.Time) error
}
