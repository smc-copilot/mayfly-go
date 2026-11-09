package application

import (
	"context"
	"fmt"

	"mayfly-go/internal/casdoor/domain/entity"
)

type ApplicationQuery interface {
	ListApplications(context.Context, uint64, string, int, int) ([]entity.Application, int64, error)
	ListUsers(context.Context, uint64, string, int, int) ([]entity.User, int64, error)
	ListObjects(context.Context, uint64, string, int, int) ([]entity.DirectoryObject, int64, error)
}

func (s *DirectoryQueryService) Objects(ctx context.Context, connectionID uint64, kind string, page, size int) ([]entity.DirectoryObject, int64, error) {
	if s == nil || s.query == nil || s.access == nil {
		return nil, 0, fmt.Errorf("directory service unavailable")
	}
	if err := s.access.AssertConnectionAccess(ctx, connectionID, false); err != nil {
		return nil, 0, err
	}
	return s.query.ListObjects(ctx, connectionID, kind, page, size)
}

func (s *DirectoryQueryService) Users(ctx context.Context, connectionID uint64, keyword string, page, size int) ([]entity.User, int64, error) {
	if s == nil || s.query == nil || s.access == nil {
		return nil, 0, fmt.Errorf("directory service unavailable")
	}
	if err := s.access.AssertConnectionAccess(ctx, connectionID, false); err != nil {
		return nil, 0, err
	}
	return s.query.ListUsers(ctx, connectionID, keyword, page, size)
}

type DirectoryQueryService struct {
	query  ApplicationQuery
	access *ScopeAuthorizer
}

func NewDirectoryQueryService(query ApplicationQuery, access *ScopeAuthorizer) *DirectoryQueryService {
	return &DirectoryQueryService{query: query, access: access}
}

func (s *DirectoryQueryService) Applications(ctx context.Context, connectionID uint64, keyword string, page, size int) ([]entity.Application, int64, error) {
	if s == nil || s.query == nil || s.access == nil {
		return nil, 0, fmt.Errorf("directory service unavailable")
	}
	if err := s.access.AssertConnectionAccess(ctx, connectionID, false); err != nil {
		return nil, 0, err
	}
	return s.query.ListApplications(ctx, connectionID, keyword, page, size)
}
