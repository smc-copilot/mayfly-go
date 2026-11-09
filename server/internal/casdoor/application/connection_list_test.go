package application

import (
	"context"
	"testing"

	"mayfly-go/internal/casdoor/domain/entity"
)

type fakeConnectionList struct{ actor uint64 }

func (f *fakeConnectionList) ListVisibleConnections(_ context.Context, id uint64) ([]entity.Connection, error) {
	f.actor = id
	return []entity.Connection{}, nil
}

func TestConnectionListRequiresActor(t *testing.T) {
	repo := &fakeConnectionList{}
	svc := NewConnectionListService(repo)
	if _, err := svc.List(context.Background()); err == nil || repo.actor != 0 {
		t.Fatal("无账号不能读取连接列表")
	}
	if _, err := svc.List(adminContext()); err != nil || repo.actor != 1 {
		t.Fatalf("应使用当前账号过滤: %v", err)
	}
}
