package application

import (
	"context"
	"strings"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/casdoor"
	"mayfly-go/internal/pkg/utils"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/logx"
	"mayfly-go/pkg/model"
	"mayfly-go/pkg/taskx"
)

const casdoorCronKey = "octop-casdoor-sync"

type Casdoor interface {
	base.App[*entity.CasdoorConnection]
	SaveConnection(ctx context.Context, conn *entity.CasdoorConnection, secret string) error
	TestConnection(ctx context.Context, conn *entity.CasdoorConnection, secret string) (*dto.CasdoorTestResult, error)
	Sync(ctx context.Context, id uint64) error
	PageApps(ctx context.Context, q *entity.CasdoorAppQuery) (*model.PageResult[*entity.CasdoorApplicationSnapshot], error)
	PageUsers(ctx context.Context, q *entity.CasdoorUserQuery) (*model.PageResult[*entity.CasdoorUserSnapshot], error)
	ListAuths(ctx context.Context, connectionId uint64, appSourceId string) ([]*entity.CasdoorAuthRelation, error)
	SaveIdentityLink(ctx context.Context, link *entity.OctopIdentityLink) error
	ListIdentityLinks(ctx context.Context, instanceId uint64) ([]*entity.OctopIdentityLink, error)
	DeleteIdentityLink(ctx context.Context, id uint64) error
	MatchCandidates(ctx context.Context, q *entity.IdentityCandidateQuery) ([]*entity.CasdoorUserSnapshot, error)
	InitCron()
}

type casdoorAppImpl struct {
	base.AppImpl[*entity.CasdoorConnection, repository.CasdoorConnection]

	appRepo      repository.CasdoorApp  `inject:"T"`
	userRepo     repository.CasdoorUser `inject:"T"`
	authRepo     repository.CasdoorAuth `inject:"T"`
	linkRepo     repository.IdentityLink `inject:"T"`
	newClient    func(casdoor.Config) casdoor.Client
}

var _ Casdoor = (*casdoorAppImpl)(nil)

func (a *casdoorAppImpl) clientFactory() func(casdoor.Config) casdoor.Client {
	if a.newClient != nil {
		return a.newClient
	}
	return casdoor.New
}

func (a *casdoorAppImpl) SaveConnection(ctx context.Context, conn *entity.CasdoorConnection, secret string) error {
	if strings.TrimSpace(conn.ServerURL) == "" {
		return errorx.NewBizI(ctx, imsg.ErrCasdoorURLRequired)
	}
	if strings.TrimSpace(conn.Organization) == "" {
		return errorx.NewBizI(ctx, imsg.ErrCasdoorOrgRequired)
	}
	if conn.TimeoutSec <= 0 {
		conn.TimeoutSec = 15
	}
	if conn.SyncIntervalSec <= 0 {
		conn.SyncIntervalSec = 3600
	}
	if conn.Id > 0 {
		old, err := a.GetById(conn.Id)
		if err != nil {
			return err
		}
		conn.ClientSecretEnc = old.ClientSecretEnc
		conn.CredentialConfigured = old.CredentialConfigured
		conn.CredentialRotatedAt = old.CredentialRotatedAt
		conn.Certificate = old.Certificate
	}
	if secret != "" {
		enc, err := utils.PwdAesEncrypt(secret)
		if err != nil {
			return err
		}
		conn.ClientSecretEnc = enc
		conn.CredentialConfigured = 1
		now := time.Now()
		conn.CredentialRotatedAt = &now
	}
	if conn.ClientSecretEnc == "" {
		return errorx.NewBizI(ctx, imsg.ErrCasdoorCredentialRequired)
	}
	return a.Save(ctx, conn)
}

func (a *casdoorAppImpl) TestConnection(ctx context.Context, conn *entity.CasdoorConnection, secret string) (*dto.CasdoorTestResult, error) {
	client, err := a.buildClient(conn, secret)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	res := &dto.CasdoorTestResult{
		Apps:  dto.ProbeItem{Name: "apps", At: &now},
		Users: dto.ProbeItem{Name: "users", At: &now},
	}
	if err := client.TestApps(ctx); err != nil {
		res.Apps.Status = entity.ProbeFail
		res.Apps.Reason = err.Error()
	} else {
		res.Apps.Status = entity.ProbeOK
	}
	if err := client.TestUsers(ctx); err != nil {
		res.Users.Status = entity.ProbeFail
		res.Users.Reason = err.Error()
	} else {
		res.Users.Status = entity.ProbeOK
	}
	return res, nil
}

func (a *casdoorAppImpl) Sync(ctx context.Context, id uint64) error {
	conn, err := a.GetById(id)
	if err != nil {
		if model.IsNotFound(err) {
			return errorx.NewBizI(ctx, imsg.ErrCasdoorNotFound)
		}
		return err
	}
	client, err := a.buildClient(conn, "")
	if err != nil {
		a.markExpired(ctx, conn, err)
		return err
	}
	if err := SyncDirectory(ctx, client, conn, a.appRepo, a.userRepo, a.authRepo); err != nil {
		a.markExpired(ctx, conn, err)
		return err
	}
	now := time.Now()
	conn.LastSyncStatus = entity.SyncStatusOK
	conn.LastSyncAt = &now
	conn.LastSuccessAt = &now
	conn.LastSyncError = ""
	return a.UpdateById(ctx, conn)
}

func (a *casdoorAppImpl) markExpired(ctx context.Context, conn *entity.CasdoorConnection, err error) {
	now := time.Now()
	conn.LastSyncStatus = entity.SyncStatusExpired
	conn.LastSyncAt = &now
	conn.LastSyncError = trimErr(err)
	if uerr := a.UpdateById(ctx, conn); uerr != nil {
		logx.ErrorTrace("mark casdoor sync expired", uerr)
	}
}

func (a *casdoorAppImpl) PageApps(ctx context.Context, q *entity.CasdoorAppQuery) (*model.PageResult[*entity.CasdoorApplicationSnapshot], error) {
	_ = ctx
	qd := model.NewCond().Eq("connection_id", q.ConnectionId).Eq("organization", q.Organization)
	if q.Enabled != nil {
		qd.Eq("enabled", *q.Enabled)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		qd.And("(name LIKE ? OR display_name LIKE ? OR source_id LIKE ?)", like, like, like)
	}
	return a.appRepo.PageByCond(qd.OrderByDesc("id"), q.PageParam)
}

func (a *casdoorAppImpl) PageUsers(ctx context.Context, q *entity.CasdoorUserQuery) (*model.PageResult[*entity.CasdoorUserSnapshot], error) {
	_ = ctx
	qd := model.NewCond().Eq("connection_id", q.ConnectionId).Eq("organization", q.Organization)
	if q.Enabled != nil {
		qd.Eq("enabled", *q.Enabled)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		qd.And("(username LIKE ? OR display_name LIKE ? OR source_id LIKE ? OR email LIKE ?)", like, like, like, like)
	}
	return a.userRepo.PageByCond(qd.OrderByDesc("id"), q.PageParam)
}

func (a *casdoorAppImpl) ListAuths(ctx context.Context, connectionId uint64, appSourceId string) ([]*entity.CasdoorAuthRelation, error) {
	_ = ctx
	return a.authRepo.SelectByCond(model.NewCond().Eq("connection_id", connectionId).Eq("application_source_id", appSourceId))
}

func (a *casdoorAppImpl) SaveIdentityLink(ctx context.Context, link *entity.OctopIdentityLink) error {
	if link.CasdoorUserSourceId == "" || link.OctopUserId == "" || link.OctopInstanceId == 0 {
		return errorx.NewBizI(ctx, imsg.ErrIdentityStableIdRequired)
	}
	exist, err := a.linkRepo.GetByCasdoorUser(link.ConnectionId, link.CasdoorUserSourceId, link.OctopInstanceId)
	if err != nil {
		return err
	}
	if exist != nil && exist.Id != link.Id {
		return errorx.NewBizI(ctx, imsg.ErrIdentityDuplicate)
	}
	other, err := a.linkRepo.GetByOctopUser(link.OctopInstanceId, link.OctopUserId)
	if err != nil {
		return err
	}
	if other != nil && other.Id != link.Id {
		return errorx.NewBizI(ctx, imsg.ErrIdentityDuplicate)
	}
	return a.linkRepo.Save(ctx, link)
}

func (a *casdoorAppImpl) ListIdentityLinks(ctx context.Context, instanceId uint64) ([]*entity.OctopIdentityLink, error) {
	_ = ctx
	return a.linkRepo.SelectByCond(model.NewCond().Eq("octop_instance_id", instanceId))
}

func (a *casdoorAppImpl) DeleteIdentityLink(ctx context.Context, id uint64) error {
	return a.linkRepo.DeleteById(ctx, id)
}

func (a *casdoorAppImpl) MatchCandidates(ctx context.Context, q *entity.IdentityCandidateQuery) ([]*entity.CasdoorUserSnapshot, error) {
	_ = ctx
	// 仅提示，不自动合并。按用户名/邮箱等值查找稳定 ID。
	qd := model.NewCond().Eq("connection_id", q.ConnectionId)
	if q.Username != "" {
		qd.Eq("username", q.Username)
	}
	if q.Email != "" {
		qd.Eq("email", q.Email)
	}
	return a.userRepo.SelectByCond(qd)
}

func (a *casdoorAppImpl) InitCron() {
	_ = taskx.BindCronTaskWithLock(casdoorCronKey, "0 */30 * * * *", 10*time.Minute, true, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cancel()
		conns, err := a.ListByCond(model.NewCond())
		if err != nil {
			logx.ErrorTrace("list casdoor connections", err)
			return
		}
		now := time.Now()
		for _, conn := range conns {
			interval := time.Duration(conn.SyncIntervalSec) * time.Second
			if interval <= 0 {
				interval = time.Hour
			}
			if conn.LastSuccessAt != nil && now.Sub(*conn.LastSuccessAt) < interval {
				continue
			}
			if err := a.Sync(ctx, conn.Id); err != nil {
				logx.Errorf("casdoor sync id=%d err=%v", conn.Id, err)
			}
		}
	})
}

func (a *casdoorAppImpl) buildClient(conn *entity.CasdoorConnection, secret string) (casdoor.Client, error) {
	plain := secret
	if plain == "" {
		dec, err := utils.PwdAesDecrypt(conn.ClientSecretEnc)
		if err != nil {
			return nil, err
		}
		plain = dec
	}
	if plain == "" {
		return nil, errorx.NewBizI(context.Background(), imsg.ErrCasdoorCredentialRequired)
	}
	timeout := time.Duration(conn.TimeoutSec) * time.Second
	return a.clientFactory()(casdoor.Config{
		ServerURL:    conn.ServerURL,
		Organization: conn.Organization,
		ClientID:     conn.ClientId,
		ClientSecret: plain,
		Timeout:      timeout,
		TLSVerify:    conn.TLSVerify != -1,
	}), nil
}

func trimErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 250 {
		return s[:250]
	}
	return s
}

// SyncDirectory 分页拉取应用/用户并按稳定 ID upsert；成功后把未出现的标为禁用/撤销。
// 失败时由调用方保留旧快照并标过期，本函数不删除已有目录。
func SyncDirectory(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, apps repository.CasdoorApp, users repository.CasdoorUser, auths repository.CasdoorAuth) error {
	now := time.Now().UTC()
	appIDs, err := syncApps(ctx, client, conn, apps, now)
	if err != nil {
		return err
	}
	userIDs, err := syncUsers(ctx, client, conn, users, now)
	if err != nil {
		return err
	}
	if err := apps.MarkMissingAbsent(ctx, conn.Id, appIDs, now); err != nil {
		return err
	}
	if err := users.MarkMissingDisabled(ctx, conn.Id, userIDs, now); err != nil {
		return err
	}
	return syncAuths(ctx, client, conn, appIDs, auths, now)
}

func syncApps(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, apps repository.CasdoorApp, now time.Time) ([]string, error) {
	ids := make([]string, 0)
	page := 1
	pageSize := entity.DefaultSyncPageSize
	got := 0
	for {
		items, total, err := client.ListApplications(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		got += len(items)
		for _, item := range items {
			if item.Organization != "" && item.Organization != conn.Organization {
				continue // 无权访问其他组织
			}
			ids = append(ids, item.SourceID)
			row, err := apps.GetBySource(conn.Id, item.SourceID)
			if err != nil {
				return nil, err
			}
			if row == nil {
				row = &entity.CasdoorApplicationSnapshot{ConnectionId: conn.Id, SourceId: item.SourceID}
			}
			row.Name = item.Name
			row.DisplayName = item.DisplayName
			row.Organization = conn.Organization
			row.Enabled = boolToInt8(item.Enabled)
			row.AppType = item.AppType
			row.HomepageURL = item.HomepageURL
			row.Present = 1
			row.LastSyncAt = &now
			if err := apps.Save(ctx, row); err != nil {
				return nil, err
			}
		}
		if len(items) == 0 || got >= total || page >= 10000 {
			break
		}
		page++
	}
	return ids, nil
}

func syncUsers(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, users repository.CasdoorUser, now time.Time) ([]string, error) {
	ids := make([]string, 0)
	page := 1
	pageSize := entity.DefaultSyncPageSize
	got := 0
	for {
		items, total, err := client.ListUsers(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		got += len(items)
		for _, item := range items {
			if item.Organization != "" && item.Organization != conn.Organization {
				continue
			}
			ids = append(ids, item.SourceID)
			row, err := users.GetBySource(conn.Id, item.SourceID)
			if err != nil {
				return nil, err
			}
			if row == nil {
				row = &entity.CasdoorUserSnapshot{ConnectionId: conn.Id, SourceId: item.SourceID}
			}
			row.Username = item.Username
			row.DisplayName = item.DisplayName
			row.Organization = conn.Organization
			row.Email = item.Email
			row.Enabled = boolToInt8(item.Enabled)
			row.Present = 1
			row.LastSyncAt = &now
			if err := users.Save(ctx, row); err != nil {
				return nil, err
			}
		}
		if len(items) == 0 || got >= total || page >= 10000 {
			break
		}
		page++
	}
	return ids, nil
}

func syncAuths(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, appIDs []string, auths repository.CasdoorAuth, now time.Time) error {
	keep := make([]repository.AuthKey, 0)
	for _, appID := range appIDs {
		userIDs, readable, err := client.GetApplicationUsers(ctx, appID)
		if err != nil || !readable {
			continue // 未核实，不写 confirmed，也不撤销
		}
		for _, uid := range userIDs {
			keep = append(keep, repository.AuthKey{AppSourceId: appID, SubjectType: "user", SubjectSourceId: uid})
			row, err := auths.GetByKey(conn.Id, appID, "user", uid)
			if err != nil {
				return err
			}
			if row == nil {
				row = &entity.CasdoorAuthRelation{
					ConnectionId: conn.Id, ApplicationSourceId: appID, SubjectType: "user", SubjectSourceId: uid,
				}
			}
			row.Status = entity.AuthConfirmed
			row.LastSyncAt = &now
			if err := auths.Save(ctx, row); err != nil {
				return err
			}
		}
	}
	return auths.MarkMissingRevoked(ctx, conn.Id, keep, now)
}

func boolToInt8(v bool) int8 {
	if v {
		return 1
	}
	return -1
}
