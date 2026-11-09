package application

import (
	"context"
	"errors"
	"testing"

	"mayfly-go/internal/casdoor/domain/directory"
)

type fakeDirectoryReader struct {
	pages      [][]directory.Application
	failPage   int
	calls      int
	knownTotal *int
	clamp      bool
}

func (f *fakeDirectoryReader) ListApplications(_ context.Context, page, _ int) ([]directory.Application, int, error) {
	f.calls++
	total := -1
	if f.knownTotal != nil {
		total = *f.knownTotal
	}
	if f.clamp && page > len(f.pages) {
		page = len(f.pages)
	}
	if page == f.failPage {
		return nil, 0, errors.New("page failed")
	}
	if page > len(f.pages) {
		return nil, total, nil
	}
	return f.pages[page-1], total, nil
}

func TestFetchApplicationsStopsAtDeclaredTotalWhenServerClampsLastPage(t *testing.T) {
	total := 2
	reader := &fakeDirectoryReader{pages: [][]directory.Application{{{SourceID: "admin/one"}}, {{SourceID: "admin/two"}}}, knownTotal: &total, clamp: true}
	items, err := FetchApplications(context.Background(), reader, 1)
	if err != nil || len(items) != 2 || reader.calls != 2 {
		t.Fatalf("declared total must end paging: len=%d calls=%d err=%v", len(items), reader.calls, err)
	}
}

func TestFetchApplicationsRejectsEmptyPageBeforeDeclaredTotal(t *testing.T) {
	total := 2
	reader := &fakeDirectoryReader{pages: [][]directory.Application{{{SourceID: "admin/one"}}}, knownTotal: &total}
	items, err := FetchApplications(context.Background(), reader, 1)
	if err == nil || items != nil {
		t.Fatalf("short read must not replace snapshot: %v %v", items, err)
	}
}

func (f *fakeDirectoryReader) ListUsers(context.Context, int, int) ([]directory.User, int, error) {
	return nil, 0, nil
}

func TestFetchApplicationsReadsUntilEmptyPage(t *testing.T) {
	reader := &fakeDirectoryReader{pages: [][]directory.Application{
		{{SourceID: "admin/one"}}, {{SourceID: "admin/two"}},
	}}
	items, err := FetchApplications(context.Background(), reader, 100)
	if err != nil || len(items) != 2 || reader.calls != 3 {
		t.Fatalf("必须读取到空页: len=%d calls=%d err=%v", len(items), reader.calls, err)
	}
}

func TestFetchApplicationsReturnsNoPartialDataOnFailure(t *testing.T) {
	reader := &fakeDirectoryReader{pages: [][]directory.Application{{{SourceID: "admin/one"}}}, failPage: 2}
	items, err := FetchApplications(context.Background(), reader, 100)
	if err == nil || items != nil {
		t.Fatalf("分页失败时不得提交部分结果: items=%v err=%v", items, err)
	}
}

func TestFetchApplicationsRejectsRepeatedRecords(t *testing.T) {
	reader := &fakeDirectoryReader{pages: [][]directory.Application{
		{{SourceID: "admin/one"}}, {{SourceID: "admin/one"}},
	}}
	items, err := FetchApplications(context.Background(), reader, 100)
	if err == nil || items != nil {
		t.Fatalf("重复记录可能表示分页异常，不得提交快照: items=%v err=%v", items, err)
	}
}
