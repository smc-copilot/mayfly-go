package repository

import (
	"context"
	"time"

	"mayfly-go/internal/casdoor/domain/entity"
)

type ObjectSnapshots interface {
	ReplaceObjects(context.Context, uint64, string, []entity.DirectoryObject, time.Time) error
}

type ObjectSyncStates interface {
	RecordObjectSyncState(context.Context, uint64, string, SyncCategory) error
}
