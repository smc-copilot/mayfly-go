package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
)

type DirectoryCategoryResult struct {
	Status       string
	ScopeLimited bool
	Count        int
	AttemptAt    time.Time
	SuccessAt    *time.Time
	Err          error
}

type DirectorySyncResult struct {
	Apps    DirectoryCategoryResult
	Users   DirectoryCategoryResult
	Objects map[string]DirectoryCategoryResult
}

// SyncDirectory 独立采集应用和用户；某类数据未完整读取时保留该类旧快照。
func SyncDirectory(ctx context.Context, conn *entity.Connection, reader directory.Reader, store repository.Snapshots, pageSize int) DirectorySyncResult {
	if conn == nil {
		err := fmt.Errorf("casdoor connection is required")
		return DirectorySyncResult{Apps: categoryFailure(nil, err), Users: categoryFailure(nil, err)}
	}
	if conn.Id == 0 || strings.TrimSpace(conn.Organization) == "" {
		err := fmt.Errorf("casdoor connection id and organization are required")
		return DirectorySyncResult{Apps: categoryFailure(conn.AppSuccessAt, err), Users: categoryFailure(conn.UserSuccessAt, err)}
	}
	return DirectorySyncResult{
		Apps:  syncApplications(ctx, conn, reader, store, pageSize),
		Users: syncUsers(ctx, conn, reader, store, pageSize),
	}
}

func syncApplications(ctx context.Context, conn *entity.Connection, reader directory.Reader, store repository.Snapshots, pageSize int) DirectoryCategoryResult {
	items, err := FetchApplications(ctx, reader, pageSize)
	if err != nil {
		return categoryFailure(conn.AppSuccessAt, err)
	}
	rows := make([]entity.Application, 0, len(items))
	for _, item := range items {
		if item.Organization != "" && item.Organization != conn.Organization {
			continue
		}
		rows = append(rows, entity.Application{
			ConnectionId: conn.Id, SourceId: item.SourceID, Name: item.Name,
			DisplayName: item.DisplayName, Organization: conn.Organization,
			AppOwner: item.Owner, ClientId: item.ClientID,
			Enabled: knownState(item.Enabled, item.EnabledKnown), AppType: item.AppType, HomepageURL: item.HomepageURL,
		})
	}
	now := time.Now().UTC()
	var saveErr error
	if strings.TrimSpace(conn.AppOwner) != "" {
		saveErr = store.MergeApplications(ctx, conn.Id, rows, now)
	} else {
		saveErr = store.ReplaceApplications(ctx, conn.Id, rows, now)
	}
	if saveErr != nil {
		return categoryFailure(conn.AppSuccessAt, saveErr)
	}
	result := categorySuccess(len(rows), now)
	result.ScopeLimited = strings.TrimSpace(conn.AppOwner) != ""
	return result
}

func syncUsers(ctx context.Context, conn *entity.Connection, reader directory.Reader, store repository.Snapshots, pageSize int) DirectoryCategoryResult {
	items, err := FetchUsers(ctx, reader, pageSize)
	if err != nil {
		return categoryFailure(conn.UserSuccessAt, err)
	}
	rows := make([]entity.User, 0, len(items))
	for _, item := range items {
		if item.Organization != "" && item.Organization != conn.Organization {
			continue
		}
		var sourceUpdatedAt *time.Time
		if item.UpdatedAt != "" {
			parsed, err := time.Parse(time.RFC3339Nano, item.UpdatedAt)
			if err != nil {
				return categoryFailure(conn.UserSuccessAt, fmt.Errorf("invalid user update time: %w", err))
			}
			sourceUpdatedAt = &parsed
		}
		rows = append(rows, entity.User{
			ConnectionId: conn.Id, SourceId: item.SourceID, ObjectId: item.ObjectID,
			Username: item.Username, DisplayName: item.DisplayName, Organization: conn.Organization,
			Email: item.Email, Enabled: knownState(item.Enabled, item.EnabledKnown), SourceUpdatedAt: sourceUpdatedAt,
		})
	}
	now := time.Now().UTC()
	if err := store.ReplaceUsers(ctx, conn.Id, rows, now); err != nil {
		return categoryFailure(conn.UserSuccessAt, err)
	}
	return categorySuccess(len(rows), now)
}

func knownState(value, known bool) int8 {
	if !known {
		return 0
	}
	if value {
		return 1
	}
	return -1
}

func categoryFailure(lastSuccess *time.Time, err error) DirectoryCategoryResult {
	status := "failed"
	if lastSuccess != nil {
		status = "stale"
	}
	return DirectoryCategoryResult{Status: status, AttemptAt: time.Now().UTC(), SuccessAt: lastSuccess, Err: err}
}

func categorySuccess(count int, at time.Time) DirectoryCategoryResult {
	return DirectoryCategoryResult{Status: "success", Count: count, AttemptAt: at, SuccessAt: &at}
}
