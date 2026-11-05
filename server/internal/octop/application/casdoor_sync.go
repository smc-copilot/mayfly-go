package application

import (
	"context"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/infra/casdoor"
)

// SyncDirectory 分别同步应用、用户与可确认关系。
// 每一类必须完整读完才提交快照和缺失项；中途失败保留该类上次成功快照。
// 应用失败不中止用户同步，反之亦然。
func SyncDirectory(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, apps repository.CasdoorApp, users repository.CasdoorUser, auths repository.CasdoorAuth) dto.CasdoorSyncResult {
	var res dto.CasdoorSyncResult
	var appItems []casdoor.Application
	res.Apps, appItems = syncAppsCategory(ctx, client, conn, apps)
	res.Users = syncUsersCategory(ctx, client, conn, users)
	if res.Apps.Status == entity.SyncStatusSuccess {
		res.Relations = syncAuthsCategory(ctx, client, conn, appItems, apps, auths)
	} else {
		now := time.Now().UTC()
		res.Relations = dto.CategorySyncResult{
			Name:      "relations",
			Status:    staleOrFailed(conn.RelationSuccessAt),
			Error:     "apps were not fully read; relation snapshot kept",
			AttemptAt: &now,
			SuccessAt: conn.RelationSuccessAt,
		}
	}
	return res
}

func syncAppsCategory(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, apps repository.CasdoorApp) (dto.CategorySyncResult, []casdoor.Application) {
	start := time.Now().UTC()
	items, err := fetchAllApps(ctx, client)
	if err != nil {
		return categoryFail("apps", start, err, conn.AppSuccessAt), nil
	}
	scoped := make([]casdoor.Application, 0, len(items))
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.Organization != "" && item.Organization != conn.Organization {
			continue
		}
		scoped = append(scoped, item)
		ids = append(ids, item.SourceID)
	}
	now := time.Now().UTC()
	for _, item := range scoped {
		if err := upsertApp(ctx, apps, conn, item, now); err != nil {
			return categoryFail("apps", start, err, conn.AppSuccessAt), nil
		}
	}
	if err := apps.MarkMissingAbsent(ctx, conn.Id, ids, now); err != nil {
		return categoryFail("apps", start, err, conn.AppSuccessAt), nil
	}
	return categoryOK("apps", start, len(scoped)), scoped
}

func syncUsersCategory(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, users repository.CasdoorUser) dto.CategorySyncResult {
	start := time.Now().UTC()
	items, err := fetchAllUsers(ctx, client)
	if err != nil {
		return categoryFail("users", start, err, conn.UserSuccessAt)
	}
	ids := make([]string, 0, len(items))
	now := time.Now().UTC()
	for _, item := range items {
		if item.Organization != "" && item.Organization != conn.Organization {
			continue
		}
		ids = append(ids, item.SourceID)
		if err := upsertUser(ctx, users, conn, item, now); err != nil {
			return categoryFail("users", start, err, conn.UserSuccessAt)
		}
	}
	if err := users.MarkMissingDisabled(ctx, conn.Id, ids, now); err != nil {
		return categoryFail("users", start, err, conn.UserSuccessAt)
	}
	return categoryOK("users", start, len(ids))
}

func syncAuthsCategory(ctx context.Context, client casdoor.Client, conn *entity.CasdoorConnection, appItems []casdoor.Application, apps repository.CasdoorApp, auths repository.CasdoorAuth) dto.CategorySyncResult {
	start := time.Now().UTC()
	now := start
	keep := make([]repository.AuthKey, 0)
	verified := make([]string, 0)
	confirmed, unverified, failed := 0, 0, 0
	for _, app := range appItems {
		read := client.GetApplicationUsers(ctx, firstNonEmpty(app.Owner, conn.AppOwner), app.Name)
		if err := setAppRelationStatus(ctx, apps, conn.Id, app.SourceID, read.Status); err != nil {
			return categoryFail("relations", start, err, conn.RelationSuccessAt)
		}
		switch read.Status {
		case entity.AuthConfirmed:
			confirmed++
			verified = append(verified, app.SourceID)
			for _, uid := range read.UserIDs {
				if uid == "" {
					continue
				}
				keep = append(keep, repository.AuthKey{AppSourceId: app.SourceID, SubjectType: "user", SubjectSourceId: uid})
				if err := upsertConfirmedAuth(ctx, auths, conn.Id, app.SourceID, uid, now); err != nil {
					return categoryFail("relations", start, err, conn.RelationSuccessAt)
				}
			}
		default:
			if read.Failed {
				failed++
			} else {
				unverified++
			}
		}
	}
	if err := auths.MarkMissingRevoked(ctx, conn.Id, verified, keep, now); err != nil {
		return categoryFail("relations", start, err, conn.RelationSuccessAt)
	}
	res := categoryOK("relations", start, confirmed)
	res.Confirmed = confirmed
	res.Unverified = unverified
	res.Failed = failed
	if confirmed == 0 && unverified+failed > 0 {
		res.Error = "auto-confirm is not supported or was not verified for some applications"
	}
	return res
}

func fetchAllApps(ctx context.Context, client casdoor.Client) ([]casdoor.Application, error) {
	out := make([]casdoor.Application, 0)
	page, pageSize, got := 1, entity.DefaultSyncPageSize, 0
	for {
		items, total, err := client.ListApplications(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		got += len(items)
		if casdoor.ShouldStopPaging(got, total, page, pageSize, len(items)) {
			return out, nil
		}
		page++
	}
}

func fetchAllUsers(ctx context.Context, client casdoor.Client) ([]casdoor.User, error) {
	out := make([]casdoor.User, 0)
	page, pageSize, got := 1, entity.DefaultSyncPageSize, 0
	for {
		items, total, err := client.ListUsers(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		got += len(items)
		if casdoor.ShouldStopPaging(got, total, page, pageSize, len(items)) {
			return out, nil
		}
		page++
	}
}

func upsertApp(ctx context.Context, apps repository.CasdoorApp, conn *entity.CasdoorConnection, item casdoor.Application, now time.Time) error {
	row, err := apps.GetBySource(conn.Id, item.SourceID)
	if err != nil {
		return err
	}
	if row == nil {
		row = &entity.CasdoorApplicationSnapshot{ConnectionId: conn.Id, SourceId: item.SourceID}
	}
	row.Name = item.Name
	row.DisplayName = item.DisplayName
	row.Organization = firstNonEmpty(item.Organization, conn.Organization)
	row.AppOwner = item.Owner
	row.Enabled = knownBoolToInt8(item.Enabled, item.EnabledKnown)
	row.AppType = item.AppType
	row.HomepageURL = item.HomepageURL
	row.Present = 1
	row.LastSyncAt = &now
	return apps.Save(ctx, row)
}

func upsertUser(ctx context.Context, users repository.CasdoorUser, conn *entity.CasdoorConnection, item casdoor.User, now time.Time) error {
	row, err := users.GetBySource(conn.Id, item.SourceID)
	if err != nil {
		return err
	}
	if row == nil {
		row = &entity.CasdoorUserSnapshot{ConnectionId: conn.Id, SourceId: item.SourceID}
	}
	row.Username = item.Username
	row.DisplayName = item.DisplayName
	row.Organization = firstNonEmpty(item.Organization, conn.Organization)
	row.Email = item.Email
	row.Enabled = knownBoolToInt8(item.Enabled, item.EnabledKnown)
	row.Present = 1
	row.LastSyncAt = &now
	return users.Save(ctx, row)
}

func upsertConfirmedAuth(ctx context.Context, auths repository.CasdoorAuth, connectionId uint64, appID, uid string, now time.Time) error {
	row, err := auths.GetByKey(connectionId, appID, "user", uid)
	if err != nil {
		return err
	}
	if row == nil {
		row = &entity.CasdoorAuthRelation{
			ConnectionId: connectionId, ApplicationSourceId: appID, SubjectType: "user", SubjectSourceId: uid,
		}
	}
	row.Status = entity.AuthConfirmed
	row.LastSyncAt = &now
	row.LastConfirmedAt = &now
	return auths.Save(ctx, row)
}

func setAppRelationStatus(ctx context.Context, apps repository.CasdoorApp, connectionId uint64, sourceID, status string) error {
	if status == "" {
		status = entity.AuthUnverified
	}
	row, err := apps.GetBySource(connectionId, sourceID)
	if err != nil || row == nil {
		return err
	}
	row.RelationStatus = status
	return apps.Save(ctx, row)
}

func categoryOK(name string, start time.Time, count int) dto.CategorySyncResult {
	now := time.Now().UTC()
	return dto.CategorySyncResult{
		Name:       name,
		Status:     entity.SyncStatusSuccess,
		Count:      count,
		DurationMs: time.Since(start).Milliseconds(),
		AttemptAt:  &now,
		SuccessAt:  &now,
	}
}

func categoryFail(name string, start time.Time, err error, lastSuccess *time.Time) dto.CategorySyncResult {
	now := time.Now().UTC()
	return dto.CategorySyncResult{
		Name:       name,
		Status:     staleOrFailed(lastSuccess),
		Error:      trimErr(err),
		DurationMs: time.Since(start).Milliseconds(),
		AttemptAt:  &now,
		SuccessAt:  lastSuccess,
	}
}

func staleOrFailed(lastSuccess *time.Time) string {
	if lastSuccess != nil {
		return entity.SyncStatusStale
	}
	return entity.SyncStatusFailed
}

func knownBoolToInt8(v, known bool) int8 {
	if !known {
		return 0
	}
	if v {
		return 1
	}
	return -1
}
