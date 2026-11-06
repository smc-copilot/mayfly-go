package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/octopapi"
	"mayfly-go/internal/pkg/utils"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/logx"
)

const octopUserFilterNote = "Octop GET /api/users has no username/email query; Mayfly fetched the complete list then filtered."

func (a *instanceAppImpl) usersFetcher() func(context.Context, string, string) octopapi.UsersResult {
	if a.fetchUsers != nil {
		return a.fetchUsers
	}
	return func(ctx context.Context, apiURL, token string) octopapi.UsersResult {
		return octopapi.ListUsers(ctx, apiURL, token, 15*time.Second)
	}
}

func (a *instanceAppImpl) ListOctopUsers(ctx context.Context, q *entity.OctopUserQuery) (*dto.OctopUserPage, error) {
	if q == nil || q.InstanceId == 0 {
		return nil, errorx.NewBizI(ctx, imsg.ErrOctopUserInstanceRequired)
	}
	inst, err := a.mustGetVisible(ctx, q.InstanceId)
	if err != nil {
		return nil, err
	}
	pageNum, pageSize := q.Normalize()
	page := &dto.OctopUserPage{
		PageNum:    pageNum,
		PageSize:   pageSize,
		List:       []*dto.OctopUser{},
		FilterMode: entity.UserFilterModeLocalFull,
		FilterNote: octopUserFilterNote,
	}
	token, err := utils.PwdAesDecrypt(inst.ApiTokenEnc)
	if err != nil {
		logx.Errorf("octop user list decrypt token instance=%s failed", inst.Code)
		page.Status = entity.UserListUnconfigured
		page.Message = "stored octop api token cannot be decrypted"
		return page, nil
	}
	res := a.usersFetcher()(ctx, inst.ApiURL, token)
	page.Status = res.Status
	page.Message = res.Message
	if res.Status != entity.UserListOK {
		return page, nil
	}
	filtered := make([]*dto.OctopUser, 0, len(res.Rows))
	for _, row := range res.Rows {
		user := projectOctopUser(row)
		if user == nil {
			continue
		}
		if !userMatchesFilter(user, q.Username, q.Email) {
			continue
		}
		filtered = append(filtered, user)
	}
	page.Total = int64(len(filtered))
	start := (pageNum - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	page.List = filtered[start:end]
	return page, nil
}

func userMatchesFilter(user *dto.OctopUser, username, email string) bool {
	if username != "" && !strings.Contains(strings.ToLower(user.Username), strings.ToLower(strings.TrimSpace(username))) {
		return false
	}
	if email != "" && !strings.Contains(strings.ToLower(user.Email), strings.ToLower(strings.TrimSpace(email))) {
		return false
	}
	return true
}

func projectOctopUser(raw map[string]any) *dto.OctopUser {
	if raw == nil {
		return nil
	}
	id := firstNonEmpty(asString(raw["id"]), asString(raw["user_id"]), asString(raw["userId"]))
	if id == "" {
		return nil
	}
	status := entity.UserStatusUnknown
	if v, ok := raw["disabled"]; ok && v != nil {
		if disabled, ok := v.(bool); ok {
			if disabled {
				status = entity.UserStatusDisabled
			} else {
				status = entity.UserStatusEnabled
			}
		}
	}
	return &dto.OctopUser{
		ID:          id,
		Username:    asString(raw["username"]),
		DisplayName: firstNonEmpty(asString(raw["display_name"]), asString(raw["displayName"])),
		Email:       firstNonEmpty(asString(raw["email"]), asString(raw["mail"])),
		Status:      status,
	}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return strings.TrimSpace(fmt.Sprint(t))
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}
