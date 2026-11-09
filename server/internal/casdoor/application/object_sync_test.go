package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
)

type fakeObjectReader struct{ failedPage int }

func (f fakeObjectReader) ListObjects(_ context.Context, _ string, page, _ int) ([]directory.Object, int, error) {
	if page == f.failedPage {
		return nil, 0, errors.New("page failed")
	}
	if page == 1 {
		return []directory.Object{{SourceID: "org/role", Projection: `{}`}}, -1, nil
	}
	return nil, -1, nil
}

type fakeObjects struct{ calls int }

func (f *fakeObjects) ReplaceObjects(context.Context, uint64, string, []entity.DirectoryObject, time.Time) error {
	f.calls++
	return nil
}

func TestObjectSyncRetainsSnapshotWhenPaginationFails(t *testing.T) {
	conn := &entity.Connection{Organization: "org"}
	conn.SetId(1)
	store := &fakeObjects{}
	result := SyncObjectCategory(context.Background(), conn, fakeObjectReader{failedPage: 2}, store, "role", 100)
	if result.Err == nil || store.calls != 0 {
		t.Fatal("分页失败必须保留最后成功的对象快照")
	}
	result = SyncObjectCategory(context.Background(), conn, fakeObjectReader{}, store, "role", 100)
	if result.Err != nil || result.Count != 1 || store.calls != 1 {
		t.Fatalf("完整采集应提交快照: %+v", result)
	}
}
