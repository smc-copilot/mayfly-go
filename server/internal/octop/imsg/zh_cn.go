package imsg

import "mayfly-go/pkg/i18n"

var Zh_CN = map[i18n.MsgId]string{
	LogOctopInstanceSave:    "Octop - 保存实例",
	LogOctopInstanceDelete:  "Octop - 删除实例",
	LogOctopInstanceRestart: "Octop - 重启实例",
	LogOctopCasdoorSave:     "Octop - 保存 Casdoor 连接",
	LogOctopCasdoorSync:     "Octop - 同步 Casdoor 目录",
	LogOctopAppBind:         "Octop - 绑定 Casdoor 应用",
	LogOctopIdentityLink:    "Octop - 确认身份关联",
	LogOctopEventExport:     "Octop - 导出活动日志",
	LogOctopRecollect:       "Octop - 手工补采",
	LogOctopSettingsSave:    "Octop - 保存采集配置",

	ErrInstanceNameRequired:      "实例名称不能为空",
	ErrInstanceMachineRequired:   "必须关联一台已有 Mayfly 主机",
	ErrInstanceNotFound:          "Octop 实例不存在",
	ErrInstanceNoPermission:      "无权访问该 Octop 实例",
	ErrRestartNotSystemd:         "仅已配置 systemd unit 的实例允许重启；命令行启动的实例只监测",
	ErrRestartUnitInvalid:        "systemd unit 名称不合法",
	ErrRestartDenied:             "无权执行重启操作",
	ErrProcessMatchInvalid:       "进程识别规则只允许字母、数字、点、下划线和短横线",
	ErrCasdoorURLRequired:        "Casdoor 服务地址不能为空",
	ErrCasdoorOrgRequired:        "Casdoor 组织范围不能为空",
	ErrCasdoorCredentialRequired: "Casdoor 服务端凭据未配置",
	ErrCasdoorNotFound:           "Casdoor 连接不存在",
	ErrCasdoorSyncBusy:           "该连接正在同步，请稍后重试",
	ErrIdentityStableIdRequired:  "身份关联必须使用 Casdoor 稳定 ID 与 Octop 用户 ID，不能按名称自动合并",
	ErrIdentityDuplicate:         "该身份关联已存在",
	ErrEventNoPermission:         "无权查询该实例的活动日志",
	ErrExportNoPermission:        "无权导出活动日志",
	ErrCollectUnsupported:        "该类别在当前实例不受支持或无只读权限",
}
