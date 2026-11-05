package application

import (
	"context"
	"strings"
	"sync"
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

var casdoorSyncLocks sync.Map

type Casdoor interface {
	base.App[*entity.CasdoorConnection]
	SaveConnection(ctx context.Context, conn *entity.CasdoorConnection, secret string) error
	TestConnection(ctx context.Context, conn *entity.CasdoorConnection, secret string) (*dto.CasdoorTestResult, error)
	Sync(ctx context.Context, id uint64) (*dto.CasdoorSyncResult, error)
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

	appRepo   repository.CasdoorApp   `inject:"T"`
	userRepo  repository.CasdoorUser  `inject:"T"`
	authRepo  repository.CasdoorAuth  `inject:"T"`
	linkRepo  repository.IdentityLink `inject:"T"`
	newClient func(casdoor.Config) casdoor.Client
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
		preserveConnectionSecrets(conn, old)
		preserveConnectionSync(conn, old)
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
	now := time.Now()
	res := &dto.CasdoorTestResult{
		Service: dto.ProbeItem{Name: "service", At: &now},
		Apps:    dto.ProbeItem{Name: "apps", At: &now},
		Users:   dto.ProbeItem{Name: "users", At: &now},
	}
	client, err := a.buildClient(conn, secret)
	if err != nil {
		res.Service.Status = entity.ProbeFail
		res.Service.Reason = trimErr(err)
		res.Apps.Status = entity.ProbeFail
		res.Apps.Reason = trimErr(err)
		res.Users.Status = entity.ProbeFail
		res.Users.Reason = trimErr(err)
		return res, nil
	}
	setProbe(&res.Service, client.TestService(ctx))
	setProbe(&res.Apps, client.TestApps(ctx))
	setProbe(&res.Users, client.TestUsers(ctx))
	return res, nil
}

func (a *casdoorAppImpl) Sync(ctx context.Context, id uint64) (*dto.CasdoorSyncResult, error) {
	unlock, ok := tryLockCasdoorSync(id)
	if !ok {
		return nil, errorx.NewBizI(ctx, imsg.ErrCasdoorSyncBusy)
	}
	defer unlock()

	conn, err := a.GetById(id)
	if err != nil {
		if model.IsNotFound(err) {
			return nil, errorx.NewBizI(ctx, imsg.ErrCasdoorNotFound)
		}
		return nil, err
	}
	client, err := a.buildClient(conn, "")
	if err != nil {
		res := dto.CasdoorSyncResult{}
		failAllCategories(&res, err)
		applyCategoryToConn(conn, res)
		if uerr := a.saveSyncState(ctx, conn); uerr != nil {
			logx.ErrorTrace("save casdoor sync state", uerr)
		}
		return &res, nil
	}
	if err := a.markSyncing(ctx, conn); err != nil {
		return nil, err
	}
	res := SyncDirectory(ctx, client, conn, a.appRepo, a.userRepo, a.authRepo)
	applyCategoryToConn(conn, res)
	if err := a.saveSyncState(ctx, conn); err != nil {
		return &res, err
	}
	return &res, nil
}

func (a *casdoorAppImpl) markSyncing(ctx context.Context, conn *entity.CasdoorConnection) error {
	now := time.Now()
	conn.LastSyncStatus = entity.SyncStatusSyncing
	conn.LastSyncAt = &now
	conn.AppSyncStatus = entity.SyncStatusSyncing
	conn.UserSyncStatus = entity.SyncStatusSyncing
	conn.RelationSyncStatus = entity.SyncStatusSyncing
	return a.saveSyncState(ctx, conn)
}

func (a *casdoorAppImpl) saveSyncState(ctx context.Context, conn *entity.CasdoorConnection) error {
	return a.GetRepo().UpdateById(ctx, conn,
		"last_sync_status", "last_sync_at", "last_sync_error", "last_success_at",
		"app_sync_status", "app_sync_at", "app_success_at", "app_sync_error", "app_sync_count", "app_sync_duration_ms",
		"user_sync_status", "user_sync_at", "user_success_at", "user_sync_error", "user_sync_count", "user_sync_duration_ms",
		"relation_sync_status", "relation_sync_at", "relation_success_at", "relation_sync_error",
		"relation_confirmed_count", "relation_unverified_count", "relation_failed_count", "relation_sync_duration_ms",
	)
}

func (a *casdoorAppImpl) PageApps(ctx context.Context, q *entity.CasdoorAppQuery) (*model.PageResult[*entity.CasdoorApplicationSnapshot], error) {
	_ = ctx
	qd := model.NewCond().Eq("connection_id", q.ConnectionId).Eq("organization", q.Organization)
	if q.Enabled != nil {
		qd.EqStrict("enabled", *q.Enabled)
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
		qd.EqStrict("enabled", *q.Enabled)
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
			if !categoryDue(conn.AppSuccessAt, interval, now) && !categoryDue(conn.UserSuccessAt, interval, now) {
				continue
			}
			res, err := a.Sync(ctx, conn.Id)
			if err != nil {
				logx.Errorf("casdoor sync id=%d err=%v", conn.Id, err)
				continue
			}
			if res != nil && (res.Apps.Error != "" || res.Users.Error != "") {
				logx.Errorf("casdoor sync id=%d apps=%s users=%s", conn.Id, res.Apps.Error, res.Users.Error)
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
		AppOwner:     conn.AppOwner,
		ClientID:     conn.ClientId,
		ClientSecret: plain,
		Timeout:      timeout,
		TLSVerify:    conn.TLSVerify != -1,
	}), nil
}

func tryLockCasdoorSync(id uint64) (func(), bool) {
	v, _ := casdoorSyncLocks.LoadOrStore(id, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	if !mu.TryLock() {
		return nil, false
	}
	return mu.Unlock, true
}

func setProbe(item *dto.ProbeItem, err error) {
	if err != nil {
		item.Status = entity.ProbeFail
		item.Reason = trimErr(err)
		return
	}
	item.Status = entity.ProbeOK
}

func failAllCategories(res *dto.CasdoorSyncResult, err error) {
	now := time.Now().UTC()
	msg := trimErr(err)
	res.Apps = dto.CategorySyncResult{Name: "apps", Status: entity.SyncStatusFailed, Error: msg, AttemptAt: &now}
	res.Users = dto.CategorySyncResult{Name: "users", Status: entity.SyncStatusFailed, Error: msg, AttemptAt: &now}
	res.Relations = dto.CategorySyncResult{Name: "relations", Status: entity.SyncStatusFailed, Error: msg, AttemptAt: &now}
}

func applyCategoryToConn(conn *entity.CasdoorConnection, res dto.CasdoorSyncResult) {
	now := time.Now()
	conn.LastSyncAt = &now
	applyAppSync(conn, res.Apps)
	applyUserSync(conn, res.Users)
	applyRelationSync(conn, res.Relations)
	conn.LastSyncStatus = rollupStatus(res.Apps.Status, res.Users.Status)
	conn.LastSyncError = firstNonEmpty(res.Apps.Error, res.Users.Error, res.Relations.Error)
	if res.Apps.Status == entity.SyncStatusSuccess || res.Users.Status == entity.SyncStatusSuccess {
		conn.LastSuccessAt = &now
	}
}

func applyAppSync(conn *entity.CasdoorConnection, r dto.CategorySyncResult) {
	conn.AppSyncStatus = r.Status
	conn.AppSyncAt = r.AttemptAt
	conn.AppSyncError = r.Error
	conn.AppSyncCount = r.Count
	conn.AppSyncDurationMs = r.DurationMs
	if r.SuccessAt != nil {
		conn.AppSuccessAt = r.SuccessAt
	}
}

func applyUserSync(conn *entity.CasdoorConnection, r dto.CategorySyncResult) {
	conn.UserSyncStatus = r.Status
	conn.UserSyncAt = r.AttemptAt
	conn.UserSyncError = r.Error
	conn.UserSyncCount = r.Count
	conn.UserSyncDurationMs = r.DurationMs
	if r.SuccessAt != nil {
		conn.UserSuccessAt = r.SuccessAt
	}
}

func applyRelationSync(conn *entity.CasdoorConnection, r dto.CategorySyncResult) {
	conn.RelationSyncStatus = r.Status
	conn.RelationSyncAt = r.AttemptAt
	conn.RelationSyncError = r.Error
	conn.RelationConfirmedCount = r.Confirmed
	conn.RelationUnverifiedCount = r.Unverified
	conn.RelationFailedCount = r.Failed
	conn.RelationSyncDurationMs = r.DurationMs
	if r.SuccessAt != nil {
		conn.RelationSuccessAt = r.SuccessAt
	}
}

func rollupStatus(appStatus, userStatus string) string {
	if appStatus == entity.SyncStatusSuccess && userStatus == entity.SyncStatusSuccess {
		return entity.SyncStatusSuccess
	}
	if appStatus == entity.SyncStatusSuccess || userStatus == entity.SyncStatusSuccess ||
		appStatus == entity.SyncStatusStale || userStatus == entity.SyncStatusStale {
		return entity.SyncStatusStale
	}
	if appStatus == entity.SyncStatusFailed || userStatus == entity.SyncStatusFailed {
		return entity.SyncStatusFailed
	}
	return entity.SyncStatusNever
}

func categoryDue(lastSuccess *time.Time, interval time.Duration, now time.Time) bool {
	if lastSuccess == nil {
		return true
	}
	return now.Sub(*lastSuccess) >= interval
}

func preserveConnectionSecrets(conn, old *entity.CasdoorConnection) {
	conn.ClientSecretEnc = old.ClientSecretEnc
	conn.CredentialConfigured = old.CredentialConfigured
	conn.CredentialRotatedAt = old.CredentialRotatedAt
	conn.Certificate = old.Certificate
}

func preserveConnectionSync(conn, old *entity.CasdoorConnection) {
	conn.LastSyncStatus = old.LastSyncStatus
	conn.LastSyncAt = old.LastSyncAt
	conn.LastSyncError = old.LastSyncError
	conn.LastSuccessAt = old.LastSuccessAt
	conn.AppSyncStatus = old.AppSyncStatus
	conn.AppSyncAt = old.AppSyncAt
	conn.AppSuccessAt = old.AppSuccessAt
	conn.AppSyncError = old.AppSyncError
	conn.AppSyncCount = old.AppSyncCount
	conn.AppSyncDurationMs = old.AppSyncDurationMs
	conn.UserSyncStatus = old.UserSyncStatus
	conn.UserSyncAt = old.UserSyncAt
	conn.UserSuccessAt = old.UserSuccessAt
	conn.UserSyncError = old.UserSyncError
	conn.UserSyncCount = old.UserSyncCount
	conn.UserSyncDurationMs = old.UserSyncDurationMs
	conn.RelationSyncStatus = old.RelationSyncStatus
	conn.RelationSyncAt = old.RelationSyncAt
	conn.RelationSuccessAt = old.RelationSuccessAt
	conn.RelationSyncError = old.RelationSyncError
	conn.RelationConfirmedCount = old.RelationConfirmedCount
	conn.RelationUnverifiedCount = old.RelationUnverifiedCount
	conn.RelationFailedCount = old.RelationFailedCount
	conn.RelationSyncDurationMs = old.RelationSyncDurationMs
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
