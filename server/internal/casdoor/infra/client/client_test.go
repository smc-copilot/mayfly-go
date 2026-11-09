package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mayfly-go/internal/casdoor/domain/directory"
)

var _ directory.Reader = (Client)(nil)

func TestListApplicationsUsesOrganizationAndOwner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/get-organization-applications" ||
			r.URL.Query().Get("organization") != "org" || r.URL.Query().Get("owner") != "admin" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "client" || password != "test-secret" {
			t.Error("missing management authentication")
		}
		fmt.Fprint(w, `{"status":"ok","data":[{"owner":"admin","name":"app","organization":"org"}],"data2":1}`)
	}))
	defer server.Close()
	client := New(Config{ServerURL: server.URL, Organization: "org", AppOwner: "admin", ClientID: "client", ClientSecret: "test-secret"})
	items, total, err := client.ListApplications(context.Background(), 1, 20)
	if err != nil || total != 1 || len(items) != 1 || items[0].SourceID != "admin/app" {
		t.Fatalf("unexpected result: items=%v total=%d err=%v", items, total, err)
	}
}

func TestListApplicationsRejectsBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"error","msg":"permission denied","data":null}`)
	}))
	defer server.Close()
	client := New(Config{ServerURL: server.URL})
	_, _, err := client.ListApplications(context.Background(), 1, 20)
	if err == nil {
		t.Fatal("HTTP 200 中的业务错误不能当成成功")
	}
}

func TestErrorDoesNotExposeClientSecretEchoedByServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"error","msg":"bad credential test-secret"}`)
	}))
	defer server.Close()
	client := New(Config{ServerURL: server.URL, ClientSecret: "test-secret"})
	_, _, err := client.ListApplications(context.Background(), 1, 20)
	if err == nil || strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("错误信息泄露凭证: %v", err)
	}
}

func TestListUsersKeepsStableIDAndObjectIDSeparate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok","data":[{"id":"org/alice","uuid":"stable-id","owner":"org","name":"alice"}],"data2":1}`)
	}))
	defer server.Close()
	client := New(Config{ServerURL: server.URL, Organization: "org"})
	items, _, err := client.ListUsers(context.Background(), 1, 20)
	if err != nil || len(items) != 1 || items[0].SourceID != "stable-id" || items[0].ObjectID != "org/alice" {
		t.Fatalf("用户稳定 ID 与 API 对象 ID 未正确区分: items=%v err=%v", items, err)
	}
}

func TestClientDoesNotFollowRedirectWithCredentials(t *testing.T) {
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
		fmt.Fprint(w, `{"status":"ok","data":[]}`)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()
	client := New(Config{ServerURL: server.URL, ClientID: "client", ClientSecret: "test-secret"})
	_, _, err := client.ListApplications(context.Background(), 1, 20)
	if err == nil || redirected {
		t.Fatal("管理凭证请求不得自动跟随重定向")
	}
}

func TestPaginationRequiresEmptyPageToConfirmCompletion(t *testing.T) {
	if ShouldStopPaging(2, 2, 1, 100, 2) {
		t.Fatal("返回总数可能表示页数，短页不能证明已读完")
	}
	if !ShouldStopPaging(3, 2, 3, 100, 0) {
		t.Fatal("空页应结束同步")
	}
}
