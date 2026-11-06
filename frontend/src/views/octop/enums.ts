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

export const OctopBindMatchEnum = {
    Unique: EnumValue.of('unique', 'octop.matchUnique').tagTypeSuccess(),
    Ambiguous: EnumValue.of('ambiguous', 'octop.matchAmbiguous').tagTypeWarning(),
    None: EnumValue.of('none', 'octop.matchNone').tagTypeInfo(),
    Unverified: EnumValue.of('unverified', 'octop.matchUnverified').tagTypeWarning(),
};

export const OctopUserListStatusEnum = {
    Ok: EnumValue.of('ok', 'octop.userListOk').tagTypeSuccess(),
    Unconfigured: EnumValue.of('unconfigured', 'octop.userListUnconfigured').tagTypeWarning(),
    Forbidden: EnumValue.of('forbidden', 'octop.userListForbidden').tagTypeDanger(),
    Unreachable: EnumValue.of('unreachable', 'octop.userListUnreachable').tagTypeDanger(),
    Unexpected: EnumValue.of('unexpected', 'octop.userListUnexpected').tagTypeWarning(),
};

export const OctopUserStatusEnum = {
    Enabled: EnumValue.of('enabled', 'common.enable').tagTypeSuccess(),
    Disabled: EnumValue.of('disabled', 'common.disable').tagTypeDanger(),
    Unknown: EnumValue.of('unknown', 'octop.statusUnknown').tagTypeInfo(),
};

export const OctopGapEnum = {
    None: EnumValue.of('', 'octop.gapNone'),
    CollectFail: EnumValue.of('collect_fail', 'octop.gapCollectFail').tagTypeDanger(),
    SourcePurged: EnumValue.of('source_purged', 'octop.gapSourcePurged').tagTypeWarning(),
    Unsupported: EnumValue.of('unsupported', 'octop.gapUnsupported').tagTypeInfo(),
};
