package application

import (
	"context"
	"errors"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/model"
)

type fakeScopes struct {
	scope *entity.ConnectionScope
	err   error
}

func (f *fakeScopes) GetScope(context.Context, uint64, uint64) (*entity.ConnectionScope, error) {
	return f.scope, f.err
}

func TestConnectionScopeRequiresActorAndCorrectAccess(t *testing.T) {
	service := NewScopeAuthorizer(&fakeScopes{scope: &entity.ConnectionScope{AccountId: 2, ConnectionId: 10, Scope: "read"}})
	if err := service.AssertConnectionAccess(context.Background(), 10, false); err == nil {
		t.Fatal("没有登录账号不能访问连接")
	}
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 2})
	if err := service.AssertConnectionAccess(ctx, 10, false); err != nil {
		t.Fatalf("只读范围应允许读取: %v", err)
	}
	if err := service.AssertConnectionAccess(ctx, 10, true); err == nil {
		t.Fatal("只读范围不能写入")
	}
}

func TestConnectionScopeRejectsMissingAndPropagatesStoreError(t *testing.T) {
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 2})
	if err := NewScopeAuthorizer(&fakeScopes{}).AssertConnectionAccess(ctx, 10, false); err == nil {
		t.Fatal("没有范围不能访问")
	}
	sourceErr := errors.New("scope storage failed")
	if err := NewScopeAuthorizer(&fakeScopes{err: sourceErr}).AssertConnectionAccess(ctx, 10, false); !errors.Is(err, sourceErr) {
		t.Fatalf("范围查询错误必须返回: %v", err)
	}
}

func TestAdminStillRequiresValidConnectionID(t *testing.T) {
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 1})
	service := NewScopeAuthorizer(&fakeScopes{})
	if err := service.AssertConnectionAccess(ctx, 0, false); err == nil {
		t.Fatal("管理员也不能访问无效连接")
	}
	if err := service.AssertConnectionAccess(ctx, 10, true); err != nil {
		t.Fatalf("管理员应能管理有效连接: %v", err)
	}
}

func TestConnectionScopeRejectsMismatchedRepositoryRow(t *testing.T) {
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 2})
	service := NewScopeAuthorizer(&fakeScopes{scope: &entity.ConnectionScope{AccountId: 2, ConnectionId: 11, Scope: "manage"}})
	if err := service.AssertConnectionAccess(ctx, 10, true); err == nil {
		t.Fatal("其他连接的范围不能用于当前连接")
	}
}

func TestConnectionScopeRejectsMissingRepository(t *testing.T) {
	ctx := contextx.WithLoginAccount(context.Background(), &model.LoginAccount{Id: 2})
	if err := NewScopeAuthorizer(nil).AssertConnectionAccess(ctx, 10, false); err == nil {
		t.Fatal("缺少范围仓储必须拒绝访问")
	}
}
