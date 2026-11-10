import EnumValue from '@/common/Enum';

export const OctopHealthEnum = {
    Normal: EnumValue.of('normal', 'octop.normal').tagTypeSuccess(),
    Warning: EnumValue.of('warning', 'octop.warning').tagTypeWarning(),
    Critical: EnumValue.of('critical', 'octop.critical').tagTypeDanger(),
    Unknown: EnumValue.of('unknown', 'octop.unknown').tagTypeInfo(),
    Maintenance: EnumValue.of('maintenance', 'octop.maintenanceStatus').tagTypeInfo(),
};

export const OctopProbeEnum = {
    Ok: EnumValue.of('ok', 'octop.normal').tagTypeSuccess(),
    Fail: EnumValue.of('fail', 'octop.critical').tagTypeDanger(),
    Unknown: EnumValue.of('unknown', 'octop.unknown').tagTypeInfo(),
    Unsupported: EnumValue.of('unsupported', 'octop.unsupported').tagTypeWarning(),
    Skipped: EnumValue.of('skipped', 'octop.unsupported').tagTypeInfo(),
};

export const OctopStartModeEnum = {
    Systemd: EnumValue.of('systemd', 'octop.systemd'),
    CLI: EnumValue.of('cli', 'octop.cli'),
};

export const OctopEventCategoryEnum = {
    Login: EnumValue.of('login', 'octop.login'),
    Operation: EnumValue.of('operation', 'octop.operation'),
    Call: EnumValue.of('call', 'octop.call'),
    Trajectory: EnumValue.of('trajectory', 'octop.trajectory'),
};

export const OctopAuthStatusEnum = {
    Confirmed: EnumValue.of('confirmed', 'octop.confirmed').tagTypeSuccess(),
    Unverified: EnumValue.of('unverified', 'octop.unverified').tagTypeWarning(),
    Unsupported: EnumValue.of('unsupported', 'octop.relationUnsupported').tagTypeInfo(),
    Revoked: EnumValue.of('revoked', 'octop.revoked').tagTypeDanger(),
};

export const OctopSyncStatusEnum = {
    Never: EnumValue.of('never', 'octop.syncNever').tagTypeInfo(),
    Syncing: EnumValue.of('syncing', 'octop.syncing').tagTypeInfo(),
    Success: EnumValue.of('success', 'octop.syncSuccess').tagTypeSuccess(),
    Failed: EnumValue.of('failed', 'octop.syncFailed').tagTypeDanger(),
    Stale: EnumValue.of('stale', 'octop.syncStale').tagTypeWarning(),
    Ok: EnumValue.of('ok', 'octop.syncSuccess').tagTypeSuccess(),
    Expired: EnumValue.of('expired', 'octop.syncStale').tagTypeWarning(),
    Running: EnumValue.of('running', 'octop.syncing').tagTypeInfo(),
};

export const OctopGapEnum = {
    None: EnumValue.of('', 'octop.gapNone'),
    CollectFail: EnumValue.of('collect_fail', 'octop.gapCollectFail').tagTypeDanger(),
    SourcePurged: EnumValue.of('source_purged', 'octop.gapSourcePurged').tagTypeWarning(),
    Unsupported: EnumValue.of('unsupported', 'octop.gapUnsupported').tagTypeInfo(),
};

export const OctopSourceStateEnum = {
    Ready: EnumValue.of('ready', 'octop.stateReady').tagTypeSuccess(),
    NotConfigured: EnumValue.of('not_configured', 'octop.stateNotConfigured').tagTypeInfo(),
    NotVerified: EnumValue.of('not_verified', 'octop.stateNotVerified').tagTypeWarning(),
    Unsupported: EnumValue.of('unsupported', 'octop.stateUnsupported').tagTypeInfo(),
    Forbidden: EnumValue.of('forbidden', 'octop.stateForbidden').tagTypeDanger(),
    Failed: EnumValue.of('failed', 'octop.stateFailed').tagTypeDanger(),
    Partial: EnumValue.of('partial', 'octop.statePartial').tagTypeWarning(),
};

/** Octop 用户/专家启用状态：与后端 int8 一致（1 / -1 / 0） */
export const OctopUserStatusEnum = {
    Enabled: EnumValue.of(1, 'octop.userEnabled').tagTypeSuccess(),
    Disabled: EnumValue.of(-1, 'octop.userDisabled').tagTypeDanger(),
    Unknown: EnumValue.of(0, 'octop.statusUnknown').tagTypeInfo(),
};
