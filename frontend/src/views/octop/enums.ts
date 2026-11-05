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
    Revoked: EnumValue.of('revoked', 'octop.revoked').tagTypeDanger(),
};

export const OctopSyncStatusEnum = {
    Never: EnumValue.of('never', 'octop.neverCollected').tagTypeInfo(),
    Ok: EnumValue.of('ok', 'octop.normal').tagTypeSuccess(),
    Expired: EnumValue.of('expired', 'octop.syncExpired').tagTypeWarning(),
    Running: EnumValue.of('running', 'octop.syncStatus').tagTypeInfo(),
};

export const OctopGapEnum = {
    None: EnumValue.of('', 'octop.gapNone'),
    CollectFail: EnumValue.of('collect_fail', 'octop.gapCollectFail').tagTypeDanger(),
    SourcePurged: EnumValue.of('source_purged', 'octop.gapSourcePurged').tagTypeWarning(),
    Unsupported: EnumValue.of('unsupported', 'octop.gapUnsupported').tagTypeInfo(),
};
