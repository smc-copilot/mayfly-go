package application

import (
	"context"
	"fmt"
	"time"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
)

type ObjectReader interface {
	ListObjects(context.Context, string, int, int) ([]directory.Object, int, error)
}

func SyncObjectCategory(ctx context.Context, conn *entity.Connection, reader ObjectReader, store repository.ObjectSnapshots, kind string, pageSize int) DirectoryCategoryResult {
	if conn == nil || conn.Id == 0 || reader == nil || store == nil || (kind != "role" && kind != "group" && kind != "permission") {
		return categoryFailure(nil, fmt.Errorf("invalid object sync request"))
	}
	items, err := fetchDirectoryPages(ctx, pageSize, func(ctx context.Context, page, size int) ([]directory.Object, int, error) {
		return reader.ListObjects(ctx, kind, page, size)
	}, func(item directory.Object) string { return item.SourceID })
	if err != nil {
		return categoryFailure(nil, err)
	}
	rows := make([]entity.DirectoryObject, 0, len(items))
	for _, item := range items {
		rows = append(rows, entity.DirectoryObject{ConnectionId: conn.Id, Kind: kind, SourceId: item.SourceID, Projection: item.Projection})
	}
	now := time.Now().UTC()
	if err := store.ReplaceObjects(ctx, conn.Id, kind, rows, now); err != nil {
		return categoryFailure(nil, err)
	}
	return categorySuccess(len(rows), now)
}
