package imsg

import "mayfly-go/pkg/i18n"

var En = map[i18n.MsgId]string{
	LogOctopInstanceSave:    "Octop - Save instance",
	LogOctopInstanceDelete:  "Octop - Delete instance",
	LogOctopInstanceRestart: "Octop - Restart instance",
	LogOctopCasdoorSave:     "Octop - Save Casdoor connection",
	LogOctopCasdoorSync:     "Octop - Sync Casdoor directory",
	LogOctopAppBind:         "Octop - Bind Casdoor application",
	LogOctopIdentityLink:    "Octop - Confirm identity link",
	LogOctopEventExport:     "Octop - Export activity logs",
	LogOctopRecollect:       "Octop - Recollect events",
	LogOctopSettingsSave:    "Octop - Save collector settings",

	ErrInstanceNameRequired:      "instance name is required",
	ErrInstanceMachineRequired:   "an existing Mayfly machine must be linked",
	ErrInstanceNotFound:          "Octop instance not found",
	ErrInstanceNoPermission:      "no permission to access this Octop instance",
	ErrRestartNotSystemd:         "restart is only allowed for systemd-managed instances; CLI-started instances are monitor-only",
	ErrRestartUnitInvalid:        "invalid systemd unit name",
	ErrRestartDenied:             "restart is not permitted",
	ErrProcessMatchInvalid:       "process match may only contain letters, digits, dots, underscores and hyphens",
	ErrCasdoorURLRequired:        "Casdoor server URL is required",
	ErrCasdoorOrgRequired:        "Casdoor organization is required",
	ErrCasdoorCredentialRequired: "Casdoor server credential is not configured",
	ErrCasdoorNotFound:           "Casdoor connection not found",
	ErrCasdoorSyncBusy:           "this Casdoor connection is already syncing",
	ErrIdentityStableIdRequired:  "identity links must use stable Casdoor and Octop IDs; names are not merged automatically",
	ErrIdentityDuplicate:         "this identity link already exists",
	ErrEventNoPermission:         "no permission to query activity logs of this instance",
	ErrExportNoPermission:        "no permission to export activity logs",
	ErrCollectUnsupported:        "this category is unsupported or has no read-only grant on the instance",
}
