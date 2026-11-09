package imsg

import "mayfly-go/pkg/i18n"

// Casdoor 独立模块消息范围；不复用 Octop 的消息编号。
const (
	ErrConnectionNoAccess i18n.MsgId = 200001 + iota
	ErrConnectionInvalid
	ErrConnectionNotFound
	ErrConnectionSecretRequired
	ErrConnectionTargetImmutable
	ErrManagementUnverified
	ErrServiceInvalid
	ErrServiceProbeFailed
	ErrServiceSaveFailed
	ErrServiceOrganizationInvalid
)

func init() {
	i18n.AppendLangMsg(i18n.Zh_CN, map[i18n.MsgId]string{
		ErrConnectionNoAccess:         "无权访问该 Casdoor 连接",
		ErrConnectionInvalid:          "Casdoor 连接配置无效",
		ErrConnectionNotFound:         "Casdoor 连接不存在",
		ErrConnectionSecretRequired:   "请提供管理应用的 Client Secret",
		ErrConnectionTargetImmutable:  "已保存的连接不能更改服务地址或组织，请新建连接",
		ErrManagementUnverified:       "管理写入能力尚未验证",
		ErrServiceInvalid:             "Casdoor 服务配置或凭证无效，已保存服务不能更换目标地址",
		ErrServiceProbeFailed:         "管理身份验证失败，请检查令牌配置及内置组织管理范围",
		ErrServiceSaveFailed:          "服务保存失败，请检查是否已登记相同地址或历史迁移目标不一致",
		ErrServiceOrganizationInvalid: "组织不属于当前服务、已缺失或不可访问",
	})
	i18n.AppendLangMsg(i18n.En, map[i18n.MsgId]string{
		ErrConnectionNoAccess:         "You do not have access to this Casdoor connection",
		ErrConnectionInvalid:          "Invalid Casdoor connection settings",
		ErrConnectionNotFound:         "Casdoor connection not found",
		ErrConnectionSecretRequired:   "The management application client secret is required",
		ErrConnectionTargetImmutable:  "Create a new connection to change the server or organization",
		ErrManagementUnverified:       "Management write capability has not been verified",
		ErrServiceInvalid:             "Invalid service settings or credential; saved targets cannot be changed",
		ErrServiceProbeFailed:         "Management identity validation failed; check token settings and built-in organization access",
		ErrServiceSaveFailed:          "Could not save service; check duplicate addresses and legacy migration targets",
		ErrServiceOrganizationInvalid: "Organization does not belong to the service or is no longer available",
	})
}
