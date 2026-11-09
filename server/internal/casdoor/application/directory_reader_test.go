package application

import (
	"context"
	"errors"
	"testing"

	"mayfly-go/internal/casdoor/domain/directory"
)

type fakeDirectoryReader struct {
	pages    [][]directory.Application
	failPage int
	calls    int
}

func (f *fakeDirectoryReader) ListApplications(_ context.Context, page, _ int) ([]directory.Application, int, error) {
	f.calls++
	if page == f.failPage {
		return nil, 0, errors.New("page failed")
	}
	if page > len(f.pages) {
		return nil, 2, nil
	}
	return f.pages[page-1], 2, nil
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
