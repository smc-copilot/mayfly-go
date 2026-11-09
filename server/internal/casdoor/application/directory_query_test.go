package application

import (
	"context"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"
)

type fakeDirectoryQuery struct{ called bool }

func (f *fakeDirectoryQuery) ListApplications(context.Context, uint64, string, int, int) ([]entity.Application, int64, error) {
	f.called = true
	return []entity.Application{{Name: "app"}}, 1, nil
}

func (f *fakeDirectoryQuery) ListUsers(context.Context, uint64, string, int, int) ([]entity.User, int64, error) {
	f.called = true
	return []entity.User{{Username: "user"}}, 1, nil
}

func (f *fakeDirectoryQuery) ListObjects(context.Context, uint64, string, int, int) ([]entity.DirectoryObject, int64, error) {
	f.called = true
	return []entity.DirectoryObject{{Kind: "role"}}, 1, nil
}

func TestDirectoryQueryRequiresConnectionAccess(t *testing.T) {
	repo := &fakeDirectoryQuery{}
	svc := NewDirectoryQueryService(repo, NewScopeAuthorizer(nil))
	if _, _, err := svc.Applications(context.Background(), 1, "", 1, 20); err == nil || repo.called {
		t.Fatal("未登录账号不得读取应用目录")
	}
	rows, count, err := svc.Applications(adminContext(), 1, "", 1, 20)
	if err != nil || count != 1 || len(rows) != 1 || !repo.called {
		t.Fatalf("管理员应能读取连接目录: %v", err)
	}
}

func TestUserDirectoryRequiresConnectionAccess(t *testing.T) {
	repo := &fakeDirectoryQuery{}
	svc := NewDirectoryQueryService(repo, NewScopeAuthorizer(nil))
	if _, _, err := svc.Users(context.Background(), 1, "", 1, 20); err == nil || repo.called {
		t.Fatal("未登录账号不得读取用户目录")
	}
	rows, count, err := svc.Users(adminContext(), 1, "", 1, 20)
	if err != nil || count != 1 || len(rows) != 1 || !repo.called {
		t.Fatalf("管理员应能读取用户目录: %v", err)
	}
}

func TestObjectDirectoryRequiresConnectionAccess(t *testing.T) {
	repo := &fakeDirectoryQuery{}
	svc := NewDirectoryQueryService(repo, NewScopeAuthorizer(nil))
	if _, _, err := svc.Objects(context.Background(), 1, "role", 1, 20); err == nil || repo.called {
		t.Fatal("未登录账号不得读取授权对象")
	}
	rows, count, err := svc.Objects(adminContext(), 1, "role", 1, 20)
	if err != nil || count != 1 || len(rows) != 1 || !repo.called {
		t.Fatalf("管理员应能读取授权对象: %v", err)
	}
}
