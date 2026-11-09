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

// Casdoor 会将超出末页的请求钳制到末页；有显式总数时必须按总数结束，无总数才读到空页。
func fetchDirectoryPages[T any](ctx context.Context, pageSize int, read func(context.Context, int, int) ([]T, int, error), id func(T) string) ([]T, error) {
	if pageSize <= 0 {
		return nil, fmt.Errorf("directory page size must be positive")
	}
	items := make([]T, 0)
	seen := make(map[string]struct{})
	expected := -1
	for page := 1; page <= maxDirectoryPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, total, err := read(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		if total >= 0 {
			if expected >= 0 && total != expected {
				return nil, fmt.Errorf("directory total changed during pagination")
			}
			expected = total
		}
		if len(rows) == 0 {
			if expected >= 0 && len(items) != expected {
				return nil, fmt.Errorf("directory ended before declared total")
			}
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
		if expected >= 0 {
			if len(items) > expected {
				return nil, fmt.Errorf("directory exceeded declared total")
			}
			if len(items) == expected {
				return items, nil
			}
		}
	}
	return nil, fmt.Errorf("directory exceeds maximum page count %d", maxDirectoryPages)
}
