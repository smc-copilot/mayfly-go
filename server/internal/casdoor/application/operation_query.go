package application

import (
	"context"
	"fmt"

	"mayfly-go/internal/casdoor/domain/entity"
)

type OperationQuery interface {
	ListOperations(context.Context, uint64, int, int) ([]entity.Change, int64, error)
}

type OperationQueryService struct {
	query  OperationQuery
	access *ScopeAuthorizer
}

func NewOperationQueryService(query OperationQuery, access *ScopeAuthorizer) *OperationQueryService {
	return &OperationQueryService{query: query, access: access}
}

func (s *OperationQueryService) List(ctx context.Context, connectionID uint64, page, size int) ([]entity.Change, int64, error) {
	if s == nil || s.query == nil || s.access == nil {
		return nil, 0, fmt.Errorf("operation service unavailable")
	}
	if err := s.access.AssertConnectionAccess(ctx, connectionID, false); err != nil {
		return nil, 0, err
	}
	return s.query.ListOperations(ctx, connectionID, page, size)
}
