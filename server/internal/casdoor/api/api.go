package api

import (
	"mayfly-go/internal/casdoor/application"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/pkg/biz"
	"mayfly-go/pkg/model"
	"mayfly-go/pkg/req"

	"github.com/spf13/cast"
)

type DirectoryAPI struct {
	ListService *application.ConnectionListService
	Connections *application.ConnectionService
	Tester      *application.ConnectionTester
	Syncer      *application.SyncService
	Directory   *application.DirectoryQueryService
	Operations  *application.OperationQueryService
}

func (a *DirectoryAPI) ReqConfs() *req.Confs {
	return req.NewConfs("/casdoor",
		req.NewGet("/connections", a.List).RequiredPermissionCode("casdoor:read"),
		req.NewPost("/connections", a.Save).RequiredPermissionCode("casdoor:connection:manage"),
		req.NewPost("/connections/test", a.Test).RequiredPermissionCode("casdoor:connection:manage"),
		req.NewPost("/connections/:id/sync", a.Sync).RequiredPermissionCode("casdoor:sync"),
		req.NewGet("/applications", a.Applications).RequiredPermissionCode("casdoor:read"),
		req.NewGet("/users", a.Users).RequiredPermissionCode("casdoor:read"),
		req.NewGet("/objects", a.Objects).RequiredPermissionCode("casdoor:read"),
		req.NewGet("/operations", a.OperationList).RequiredPermissionCode("casdoor:audit:read"),
	)
}

func (a *DirectoryAPI) OperationList(rc *req.Ctx) {
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Operations.List(rc.MetaCtx, q.ConnectionId, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[entity.Change]{List: rows, Total: total, PageNum: page, PageSize: size}
}

type ConnectionForm struct {
	Id                uint64 `json:"id"`
	Name              string `json:"name"`
	ServerURL         string `json:"serverUrl"`
	Organization      string `json:"organization"`
	AppOwner          string `json:"appOwner"`
	ClientId          string `json:"clientId"`
	ClientSecret      string `json:"clientSecret"`
	TimeoutSec        int    `json:"timeoutSec"`
	SyncIntervalSec   int    `json:"syncIntervalSec"`
	TLSVerify         int8   `json:"tlsVerify"`
	ManagementEnabled *bool  `json:"managementEnabled"`
}

func (f *ConnectionForm) connection() *entity.Connection {
	c := &entity.Connection{Name: f.Name, ServerURL: f.ServerURL, Organization: f.Organization, AppOwner: f.AppOwner, ClientId: f.ClientId, TimeoutSec: f.TimeoutSec, SyncIntervalSec: f.SyncIntervalSec, TLSVerify: f.TLSVerify}
	c.SetId(f.Id)
	return c
}

func (a *DirectoryAPI) List(rc *req.Ctx) {
	rows, err := a.ListService.List(rc.MetaCtx)
	biz.ErrIsNil(err)
	rc.ResData = rows
}

func (a *DirectoryAPI) Save(rc *req.Ctx) {
	f := rc.BindJson[ConnectionForm]()
	conn := f.connection()
	biz.ErrIsNil(a.Connections.Save(rc.MetaCtx, conn, f.ClientSecret, f.ManagementEnabled))
	rc.ResData = conn.Id
}

func (a *DirectoryAPI) Test(rc *req.Ctx) {
	f := rc.BindJson[ConnectionForm]()
	result, err := a.Tester.Test(rc.MetaCtx, f.connection(), f.ClientSecret)
	biz.ErrIsNil(err)
	rc.ResData = result
}

type syncCategory struct {
	Status       string `json:"status"`
	ScopeLimited bool   `json:"scopeLimited"`
	Count        int    `json:"count"`
	ReasonCode   string `json:"reasonCode,omitempty"`
}

func category(result application.DirectoryCategoryResult) syncCategory {
	c := syncCategory{Status: result.Status, ScopeLimited: result.ScopeLimited, Count: result.Count}
	if result.Err != nil {
		c.ReasonCode = "sync_failed"
	}
	return c
}

func (a *DirectoryAPI) Sync(rc *req.Ctx) {
	result, err := a.Syncer.Sync(rc.MetaCtx, cast.ToUint64(rc.PathParam("id")))
	biz.ErrIsNil(err)
	response := map[string]syncCategory{"applications": category(result.Apps), "users": category(result.Users)}
	for kind, value := range result.Objects {
		response[kind] = category(value)
	}
	rc.ResData = response
}

type DirectoryQuery struct {
	ConnectionId uint64 `form:"connectionId"`
	Kind         string `form:"kind"`
	Keyword      string `form:"keyword"`
	PageNum      int    `form:"pageNum"`
	PageSize     int    `form:"pageSize"`
}

func (a *DirectoryAPI) Objects(rc *req.Ctx) {
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Directory.Objects(rc.MetaCtx, q.ConnectionId, q.Kind, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[entity.DirectoryObject]{List: rows, Total: total, PageNum: page, PageSize: size}
}

func (q *DirectoryQuery) page() (int, int) {
	page, size := (model.PageParam{PageNum: q.PageNum, PageSize: q.PageSize}).Normalize()
	if size > 100 {
		size = 100
	}
	return page, size
}

func (a *DirectoryAPI) Applications(rc *req.Ctx) {
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Directory.Applications(rc.MetaCtx, q.ConnectionId, q.Keyword, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[entity.Application]{List: rows, Total: total, PageNum: page, PageSize: size}
}

func (a *DirectoryAPI) Users(rc *req.Ctx) {
	q := rc.BindQuery[DirectoryQuery]()
	page, size := q.page()
	rows, total, err := a.Directory.Users(rc.MetaCtx, q.ConnectionId, q.Keyword, page, size)
	biz.ErrIsNil(err)
	rc.ResData = model.PageResult[entity.User]{List: rows, Total: total, PageNum: page, PageSize: size}
}
