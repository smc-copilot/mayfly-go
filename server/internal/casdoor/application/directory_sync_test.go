package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
)

type fakeSyncReader struct {
	apps        [][]directory.Application
	users       [][]directory.User
	failAppPage int
}

func (f *fakeSyncReader) ListApplications(_ context.Context, page, _ int) ([]directory.Application, int, error) {
	if page == f.failAppPage {
		return nil, 0, errors.New("application page failed")
	}
	if page > len(f.apps) {
		return nil, -1, nil
	}
	return f.apps[page-1], -1, nil
}

func (f *fakeSyncReader) ListUsers(_ context.Context, page, _ int) ([]directory.User, int, error) {
	if page > len(f.users) {
		return nil, -1, nil
	}
	return f.users[page-1], -1, nil
}

type fakeSnapshots struct {
	appCalls, userCalls int
	partial             bool
	apps                []entity.Application
	users               []entity.User
}

func (f *fakeSnapshots) MergeApplications(ctx context.Context, id uint64, rows []entity.Application, at time.Time) error {
	f.partial = true
	return f.ReplaceApplications(ctx, id, rows, at)
}

func TestSyncDirectoryWithOwnerUsesPartialSnapshot(t *testing.T) {
	conn := &entity.Connection{Organization: "org", AppOwner: "octop"}
	conn.SetId(1)
	store := &fakeSnapshots{}
	result := SyncDirectory(context.Background(), conn, &fakeSyncReader{}, store, 100)
	if result.Apps.Err != nil || !store.partial || !result.Apps.ScopeLimited {
		t.Fatalf("限定 Owner 的同步必须使用部分合并: %+v", result.Apps)
	}
}

func (f *fakeSnapshots) ReplaceApplications(_ context.Context, _ uint64, rows []entity.Application, _ time.Time) error {
	f.appCalls++
	f.apps = rows
	return nil
}

func (f *fakeSnapshots) ReplaceUsers(_ context.Context, _ uint64, rows []entity.User, _ time.Time) error {
	f.userCalls++
	f.users = rows
	return nil
}

func TestSyncDirectoryKeepsFailedCategoryAndContinuesOther(t *testing.T) {
	last := time.Now().Add(-time.Hour)
	conn := &entity.Connection{Organization: "org", AppSuccessAt: &last}
	conn.SetId(1)
	reader := &fakeSyncReader{
		apps:        [][]directory.Application{{{SourceID: "admin/app", Organization: "org"}}},
		users:       [][]directory.User{{{SourceID: "uuid", ObjectID: "org/user", Username: "user", Organization: "org"}}},
		failAppPage: 2,
	}
	store := &fakeSnapshots{}
	result := SyncDirectory(context.Background(), conn, reader, store, 100)
	if result.Apps.Status != "stale" || result.Apps.Err == nil || store.appCalls != 0 {
		t.Fatalf("应用分页失败时必须保留旧快照: %+v", result.Apps)
	}
	if result.Users.Status != "success" || store.userCalls != 1 || len(store.users) != 1 {
		t.Fatalf("用户采集应独立成功: %+v", result.Users)
	}
}

func TestSyncDirectoryFiltersOtherOrganizationsAndKeepsUnknownState(t *testing.T) {
	conn := &entity.Connection{Organization: "org"}
	conn.SetId(1)
	reader := &fakeSyncReader{
		apps: [][]directory.Application{{
			{SourceID: "admin/app", Name: "app", Organization: "org"},
			{SourceID: "admin/other", Name: "other", Organization: "other"},
		}},
		users: [][]directory.User{{{
			SourceID: "uuid", ObjectID: "org/user", Username: "user", Organization: "org",
		}}},
	}
	store := &fakeSnapshots{}
	result := SyncDirectory(context.Background(), conn, reader, store, 100)
	if result.Apps.Status != "success" || len(store.apps) != 1 || store.apps[0].Enabled != 0 {
		t.Fatalf("应过滤其他组织并保留未知状态: %+v", store.apps)
	}
	if result.Users.Status != "success" || len(store.users) != 1 || store.users[0].ObjectId != "org/user" {
		t.Fatalf("用户对象 ID 应被保存: %+v", store.users)
	}
}
