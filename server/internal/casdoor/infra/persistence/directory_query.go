package persistence

import (
	"context"
	"fmt"
	"strings"

	"mayfly-go/internal/casdoor/domain/entity"

	"gorm.io/gorm"
)

type DirectoryQuery struct{ db *gorm.DB }

func NewDirectoryQuery(db *gorm.DB) *DirectoryQuery { return &DirectoryQuery{db: db} }

func (q *DirectoryQuery) ListApplications(ctx context.Context, connectionID uint64, keyword string, page, size int) ([]entity.Application, int64, error) {
	if connectionID == 0 || page < 1 || size < 1 || size > 100 {
		return nil, 0, fmt.Errorf("invalid directory query")
	}
	db := q.db.WithContext(ctx).Model(&entity.Application{}).Where("connection_id = ? AND is_deleted = 0 AND present = 1", connectionID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR display_name LIKE ? OR source_id LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []entity.Application
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (q *DirectoryQuery) ListUsers(ctx context.Context, connectionID uint64, keyword string, page, size int) ([]entity.User, int64, error) {
	if connectionID == 0 || page < 1 || size < 1 || size > 100 {
		return nil, 0, fmt.Errorf("invalid directory query")
	}
	db := q.db.WithContext(ctx).Model(&entity.User{}).Where("connection_id = ? AND is_deleted = 0 AND present = 1", connectionID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("username LIKE ? OR display_name LIKE ? OR source_id LIKE ? OR email LIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []entity.User
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (q *DirectoryQuery) ListObjects(ctx context.Context, connectionID uint64, kind string, page, size int) ([]entity.DirectoryObject, int64, error) {
	if connectionID == 0 || page < 1 || size < 1 || size > 100 || (kind != "role" && kind != "group" && kind != "permission") {
		return nil, 0, fmt.Errorf("invalid directory query")
	}
	db := q.db.WithContext(ctx).Model(&entity.DirectoryObject{}).Where("connection_id = ? AND kind = ? AND is_deleted = 0", connectionID, kind)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []entity.DirectoryObject
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
