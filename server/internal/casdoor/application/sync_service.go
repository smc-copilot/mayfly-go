package application

import (
	"context"
	"fmt"
	"time"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/repository"
)

type ReaderFactory func(directory.ProbeConfig) directory.Reader

type SyncService struct {
	connections repository.Connections
	secrets     repository.Secrets
	access      *ScopeAuthorizer
	snapshots   repository.Snapshots
	state       repository.SyncStateRecorder
	objects     repository.ObjectSnapshots
	objectState repository.ObjectSyncStates
	locks       *ConnectionLocks
	reader      ReaderFactory
}

func NewSyncService(c repository.Connections, secrets repository.Secrets, access *ScopeAuthorizer, snapshots repository.Snapshots, state repository.SyncStateRecorder, objects repository.ObjectSnapshots, objectState repository.ObjectSyncStates, locks *ConnectionLocks, reader ReaderFactory) *SyncService {
	return &SyncService{connections: c, secrets: secrets, access: access, snapshots: snapshots, state: state, objects: objects, objectState: objectState, locks: locks, reader: reader}
}

func (s *SyncService) Sync(ctx context.Context, id uint64) (*DirectorySyncResult, error) {
	if s == nil || s.connections == nil || s.secrets == nil || s.access == nil || s.snapshots == nil || s.state == nil || s.objects == nil || s.objectState == nil || s.locks == nil || s.reader == nil {
		return nil, fmt.Errorf("sync service unavailable")
	}
	if err := s.access.AssertConnectionAccess(ctx, id, true); err != nil {
		return nil, err
	}
	release, err := s.locks.Acquire(ctx, id)
	if err != nil {
		return nil, err
	}
	defer release()
	conn, err := s.connections.GetConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	if conn.ServiceID != 0 {
		return nil, fmt.Errorf("use_service_organization_sync")
	}
	secret, err := s.secrets.Decrypt(conn.ClientSecretEnc)
	if err != nil {
		err := fmt.Errorf("credential unavailable")
		state := repository.SyncState{
			Applications: syncCategoryState(categoryFailure(conn.AppSuccessAt, err)),
			Users:        syncCategoryState(categoryFailure(conn.UserSuccessAt, err)),
		}
		if saveErr := s.state.RecordSyncState(ctx, id, state); saveErr != nil {
			return nil, saveErr
		}
		for _, kind := range []string{"role", "group", "permission"} {
			if saveErr := s.objectState.RecordObjectSyncState(ctx, id, kind, syncCategoryState(categoryFailure(nil, err))); saveErr != nil {
				return nil, saveErr
			}
		}
		return nil, err
	}
	reader := s.reader(directory.ProbeConfig{ServerURL: conn.ServerURL, Organization: conn.Organization, AppOwner: conn.AppOwner, ClientID: conn.ClientId, ClientSecret: secret, Timeout: time.Duration(conn.TimeoutSec) * time.Second, TLSVerify: conn.TLSVerify != -1})
	if reader == nil {
		return nil, fmt.Errorf("directory reader unavailable")
	}
	result := SyncDirectory(ctx, conn, reader, s.snapshots, 100)
	state := repository.SyncState{Applications: syncCategoryState(result.Apps), Users: syncCategoryState(result.Users)}
	if err := s.state.RecordSyncState(ctx, id, state); err != nil {
		return nil, err
	}
	objectReader, ok := reader.(ObjectReader)
	if !ok {
		return nil, fmt.Errorf("object reader unavailable")
	}
	result.Objects = make(map[string]DirectoryCategoryResult, 3)
	for _, kind := range []string{"role", "group", "permission"} {
		objectResult := SyncObjectCategory(ctx, conn, objectReader, s.objects, kind, 100)
		if err := s.objectState.RecordObjectSyncState(ctx, id, kind, syncCategoryState(objectResult)); err != nil {
			return nil, err
		}
		result.Objects[kind] = objectResult
	}
	return &result, nil
}

func syncCategoryState(result DirectoryCategoryResult) repository.SyncCategory {
	state := repository.SyncCategory{Status: result.Status, AttemptAt: result.AttemptAt, SuccessAt: result.SuccessAt, Count: result.Count}
	if result.Err != nil {
		state.ReasonCode = "sync_failed"
	}
	return state
}
