package application

import (
	"context"

	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/internal/casdoor/imsg"
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
)

type ScopeAuthorizer struct {
	scopes repository.Scopes
}

func NewScopeAuthorizer(scopes repository.Scopes) *ScopeAuthorizer {
	return &ScopeAuthorizer{scopes: scopes}
}

// AssertConnectionAccess 校验账号范围；接口动作权限由路由单独校验。
func (a *ScopeAuthorizer) AssertConnectionAccess(ctx context.Context, connectionID uint64, write bool) error {
	account := contextx.GetLoginAccount(ctx)
	if account == nil || connectionID == 0 {
		return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
	}
	if account.Id == consts.AdminId {
		return nil
	}
	if a == nil || a.scopes == nil {
		return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
	}
	scope, err := a.scopes.GetScope(ctx, account.Id, connectionID)
	if err != nil {
		return err
	}
	if scope == nil || scope.IsDeleted != 0 || scope.AccountId != account.Id || scope.ConnectionId != connectionID {
		return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
	}
	if scope.Scope == "manage" || (!write && scope.Scope == "read") {
		return nil
	}
	return errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
}
