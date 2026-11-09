package application

import (
	"context"
	"errors"
	"testing"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
)

type fakeSyncRecorder struct{ calls int }

func (f *fakeSyncRecorder) RecordSyncState(_ context.Context, _ uint64, _ repository.SyncState) error {
	f.calls++
	return nil
}

type badDecrypt struct{ fakeSecrets }

func (badDecrypt) Decrypt(string) (string, error) { return "", errors.New("fixture secret error") }

func TestManualSyncRecordsCredentialFailure(t *testing.T) {
	conn := &entity.Connection{Organization: "org", ClientSecretEnc: "encrypted"}
	conn.SetId(1)
	recorder := &fakeSyncRecorder{}
	svc := NewSyncService(&fakeConnections{existing: conn}, badDecrypt{}, NewScopeAuthorizer(nil), &fakeSnapshots{}, recorder, &fakeObjects{}, &fakeObjectStateRecorder{}, NewConnectionLocks(), func(directory.ProbeConfig) directory.Reader { return &fakeCompleteReader{} })
	if _, err := svc.Sync(adminContext(), 1); err == nil || recorder.calls != 1 {
		t.Fatal("凭证错误必须记录失败状态并报告错误")
	}
}

type fakeObjectStateRecorder struct{ calls int }

func (f *fakeObjectStateRecorder) RecordObjectSyncState(context.Context, uint64, string, repository.SyncCategory) error {
	f.calls++
	return nil
}

type fakeCompleteReader struct{ fakeSyncReader }

func (f *fakeCompleteReader) ListObjects(context.Context, string, int, int) ([]directory.Object, int, error) {
	return nil, 0, nil
}

func TestManualSyncRequiresConnectionScope(t *testing.T) {
	conn := &entity.Connection{Organization: "org", ClientSecretEnc: "encrypted"}
	conn.SetId(1)
	readers := 0
	recorder := &fakeSyncRecorder{}
	objectStore, objectState := &fakeObjects{}, &fakeObjectStateRecorder{}
	svc := NewSyncService(&fakeConnections{existing: conn}, fakeSecrets{}, NewScopeAuthorizer(nil), &fakeSnapshots{}, recorder, objectStore, objectState, NewConnectionLocks(), func(directory.ProbeConfig) directory.Reader { readers++; return &fakeCompleteReader{} })
	if _, err := svc.Sync(context.Background(), 1); err == nil || readers != 0 {
		t.Fatal("未授权账号不能启动同步")
	}
	res, err := svc.Sync(adminContext(), 1)
	if err != nil || res.Apps.Status != "success" || res.Users.Status != "success" || readers != 1 || recorder.calls != 1 || objectStore.calls != 3 || objectState.calls != 3 {
		t.Fatalf("管理员同步失败: %+v %v", res, err)
	}
}
