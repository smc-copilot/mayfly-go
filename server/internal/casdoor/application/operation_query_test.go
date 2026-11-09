package application

import (
	"context"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"
)

type fakeOperations struct{ called bool }

func (f *fakeOperations) ListOperations(context.Context, uint64, int, int) ([]entity.Change, int64, error) {
	f.called = true
	return []entity.Change{{Action: "user.update"}}, 1, nil
}

func TestOperationListRequiresConnectionScope(t *testing.T) {
	repo := &fakeOperations{}
	svc := NewOperationQueryService(repo, NewScopeAuthorizer(nil))
	if _, _, err := svc.List(context.Background(), 1, 1, 10); err == nil || repo.called {
		t.Fatal("无账号不能读取操作记录")
	}
	rows, count, err := svc.List(adminContext(), 1, 1, 10)
	if err != nil || count != 1 || len(rows) != 1 {
		t.Fatalf("管理员读取失败: %v", err)
	}
}
