package application

import (
	"context"
	"fmt"

	"mayfly-go/internal/casdoor/domain/directory"
)

const maxDirectoryPages = 10000

func FetchApplications(ctx context.Context, reader directory.Reader, pageSize int) ([]directory.Application, error) {
	return fetchDirectoryPages(ctx, pageSize, reader.ListApplications, func(item directory.Application) string { return item.SourceID })
}

func FetchUsers(ctx context.Context, reader directory.Reader, pageSize int) ([]directory.User, error) {
	return fetchDirectoryPages(ctx, pageSize, reader.ListUsers, func(item directory.User) string { return item.SourceID })
}

// fetchDirectoryPages 必须读到空页。任何错误都不能把部分结果提交为完整快照。
func fetchDirectoryPages[T any](ctx context.Context, pageSize int, read func(context.Context, int, int) ([]T, int, error), id func(T) string) ([]T, error) {
	if pageSize <= 0 {
		return nil, fmt.Errorf("directory page size must be positive")
	}
	items := make([]T, 0)
	seen := make(map[string]struct{})
	for page := 1; page <= maxDirectoryPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, _, err := read(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return items, nil
		}
		for _, row := range rows {
			sourceID := id(row)
			if sourceID == "" {
				return nil, fmt.Errorf("directory page %d contains a record without stable id", page)
			}
			if _, exists := seen[sourceID]; exists {
				return nil, fmt.Errorf("directory page %d contains a duplicate stable id", page)
			}
			seen[sourceID] = struct{}{}
			items = append(items, row)
		}
	}
	return nil, fmt.Errorf("directory exceeds maximum page count %d", maxDirectoryPages)
}
