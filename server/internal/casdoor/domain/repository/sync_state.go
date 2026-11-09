package repository

import (
	"context"
	"time"
)

type SyncCategory struct {
	Status     string
	AttemptAt  time.Time
	SuccessAt  *time.Time
	Count      int
	ReasonCode string
}

type SyncState struct {
	Applications SyncCategory
	Users        SyncCategory
}

type SyncStateRecorder interface {
	RecordSyncState(context.Context, uint64, SyncState) error
}
