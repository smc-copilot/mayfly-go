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
)

func init() {
	i18n.AppendLangMsg(i18n.Zh_CN, map[i18n.MsgId]string{
		ErrConnectionNoAccess:        "无权访问该 Casdoor 连接",
		ErrConnectionInvalid:         "Casdoor 连接配置无效",
		ErrConnectionNotFound:        "Casdoor 连接不存在",
		ErrConnectionSecretRequired:  "请提供管理应用的 Client Secret",
		ErrConnectionTargetImmutable: "已保存的连接不能更改服务地址或组织，请新建连接",
		ErrManagementUnverified:      "管理写入能力尚未验证",
	})
	i18n.AppendLangMsg(i18n.En, map[i18n.MsgId]string{
		ErrConnectionNoAccess:        "You do not have access to this Casdoor connection",
		ErrConnectionInvalid:         "Invalid Casdoor connection settings",
		ErrConnectionNotFound:        "Casdoor connection not found",
		ErrConnectionSecretRequired:  "The management application client secret is required",
		ErrConnectionTargetImmutable: "Create a new connection to change the server or organization",
		ErrManagementUnverified:      "Management write capability has not been verified",
	})
}
