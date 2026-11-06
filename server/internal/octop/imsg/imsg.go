package imsg

import (
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/i18n"
)

func init() {
	i18n.AppendLangMsg(i18n.Zh_CN, Zh_CN)
	i18n.AppendLangMsg(i18n.En, En)
}

const (
	LogOctopInstanceSave = iota + consts.ImsgNumOctop
	LogOctopInstanceDelete
	LogOctopInstanceRestart
	LogOctopCasdoorSave
	LogOctopCasdoorSync
	LogOctopAppBind
	LogOctopIdentityLink
	LogOctopEventExport
	LogOctopRecollect
	LogOctopSettingsSave

	ErrInstanceNameRequired
	ErrInstanceMachineRequired
	ErrInstanceNotFound
	ErrInstanceNoPermission
	ErrRestartNotSystemd
	ErrRestartUnitInvalid
	ErrRestartDenied
	ErrProcessMatchInvalid
	ErrCasdoorURLRequired
	ErrCasdoorOrgRequired
	ErrCasdoorCredentialRequired
	ErrCasdoorNotFound
	ErrCasdoorSyncBusy
	ErrIdentityStableIdRequired
	ErrIdentityDuplicate
	ErrEventNoPermission
	ErrExportNoPermission
	ErrCollectUnsupported
	ErrBindOverwriteRequired
	ErrBindAppNotFound
	ErrOctopUserInstanceRequired
)
