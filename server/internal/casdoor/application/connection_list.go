package application

import (
	"context"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/imsg"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
)

type ConnectionListQuery interface {
	ListVisibleConnections(context.Context, uint64) ([]entity.Connection, error)
}

type ConnectionListService struct{ query ConnectionListQuery }

func NewConnectionListService(query ConnectionListQuery) *ConnectionListService {
	return &ConnectionListService{query: query}
}

func (s *ConnectionListService) List(ctx context.Context) ([]entity.Connection, error) {
	actor := contextx.GetLoginAccount(ctx)
	if s == nil || s.query == nil || actor == nil || actor.Id == 0 {
		return nil, errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
	}
	return s.query.ListVisibleConnections(ctx, actor.Id)
}
