export interface ProbeItem {
    name: string;
    status: string;
    reason: string;
    at?: string;
}

export interface OctopInstance {
    id: number;
    code: string;
    name: string;
    center: string;
    remark: string;
    machineId: number;
    apiUrl: string;
    callbackUrl?: string;
    apiTokenConfigured?: number;
    startMode: string;
    systemdUnit: string;
    processMatch: string;
    dbId: number;
    dbName: string;
    casdoorConnectionId: number;
    casdoorAppSnapshotId: number;
    collectEnabled: number;
    maintenance: number;
    sampleIntervalSec: number;
    cpuWarn: number;
    cpuCrit: number;
    memWarn: number;
    memCrit: number;
    diskWarn: number;
    diskCrit: number;
    httpFailThreshold: number;
    lastHealthStatus: string;
    lastHealthReason: string;
    lastCollectAt?: string;
}

export interface InstanceOverview extends OctopInstance {
    machineName: string;
    machineIp: string;
    machineOnline: string;
    octopProcess: string;
    httpHealth: string;
    pgConnect: string;
    healthStatus: string;
    healthReason: string;
    casdoorAppName: string;
}

export interface InstanceDetail {
    overview: InstanceOverview;
    probes: ProbeItem[];
    thresholds: OctopInstance;
    canRestart: boolean;
    restartHint: string;
    checkpoints: CollectorCheckpoint[];
    latestSample?: MetricSample;
}

export interface ConnTestResult {
    ssh: ProbeItem;
    http: ProbeItem;
    pg: ProbeItem;
    saved: string;
}

export interface MetricSample {
    instanceId: number;
    collectedAt: string;
    hostOnline?: number | null;
    hostCpu?: number | null;
    hostMem?: number | null;
    hostDisk?: number | null;
    octopProcess: string;
    httpHealth: string;
    pgConnect: string;
    healthStatus: string;
    healthReason: string;
}

export interface CollectorCheckpoint {
    instanceId: number;
    sourceTable: string;
    lastRecordId: string;
    lastSuccessAt?: string;
    retentionStart?: string;
    lastError: string;
    coverage: string;
}

export interface CategorySyncState {
    name: string;
    status: string;
    error: string;
    count: number;
    durationMs: number;
    attemptAt?: string;
    successAt?: string;
    confirmed?: number;
    unverified?: number;
    failed?: number;
}

export interface CasdoorTestResult {
    service: ProbeItem;
    apps: ProbeItem;
    users: ProbeItem;
}

export interface CasdoorSyncResult {
    apps: CategorySyncState;
    users: CategorySyncState;
    relations: CategorySyncState;
}

export interface CasdoorConnection {
    id: number;
    name: string;
    serverUrl: string;
    organization: string;
    appOwner: string;
    clientId: string;
    timeoutSec: number;
    syncIntervalSec: number;
    tlsVerify: number;
    credentialConfigured: number;
    credentialRotatedAt?: string;
    lastSyncStatus: string;
    lastSyncAt?: string;
    lastSyncError: string;
    lastSuccessAt?: string;
    appSyncStatus: string;
    appSyncAt?: string;
    appSuccessAt?: string;
    appSyncError: string;
    appSyncCount: number;
    appSyncDurationMs: number;
    userSyncStatus: string;
    userSyncAt?: string;
    userSuccessAt?: string;
    userSyncError: string;
    userSyncCount: number;
    userSyncDurationMs: number;
    relationSyncStatus: string;
    relationSyncAt?: string;
    relationSuccessAt?: string;
    relationSyncError: string;
    relationConfirmedCount: number;
    relationUnverifiedCount: number;
    relationFailedCount: number;
    relationSyncDurationMs: number;
}

export interface CasdoorApp {
    id: number;
    connectionId: number;
    sourceId: string;
    name: string;
    displayName: string;
    organization: string;
    appOwner: string;
    enabled: number;
    appType: string;
    homepageUrl: string;
    redirectUris: string;
    present: number;
    relationStatus: string;
    lastSyncAt?: string;
}

export interface CasdoorUser {
    id: number;
    connectionId: number;
    sourceId: string;
    username: string;
    displayName: string;
    organization: string;
    email: string;
    enabled: number;
    present: number;
    lastSyncAt?: string;
}

export interface CasdoorAuth {
    id: number;
    applicationSourceId: string;
    subjectType: string;
    subjectSourceId: string;
    status: string;
    lastSyncAt?: string;
    lastConfirmedAt?: string;
}

export interface IdentityLink {
    id: number;
    connectionId: number;
    casdoorUserSourceId: string;
    octopInstanceId: number;
    octopUserId: string;
    evidence: string;
    creator?: string;
}

export interface OctopEvent {
    id: number;
    sourceInstanceId: number;
    sourceTable: string;
    sourceRecordId: string;
    category: string;
    action: string;
    status: string;
    octopUserId: string;
    octopUsername: string;
    agentId: string;
    agentName: string;
    threadId: string;
    eventId: string;
    target: string;
    toolName: string;
    model: string;
    source: string;
    inputTokens: number;
    outputTokens: number;
    durationMs: number;
    identityStatus: string;
    casdoorUserId: string;
    sourceTime?: string;
    collectedAt: string;
    diagnostic: string;
    uncollected: number;
}

export interface EventGap {
    instanceId: number;
    sourceTable: string;
    coverage: string;
    reason: string;
    lastRecordId: string;
    lastSuccessAt?: string;
    retentionStart?: string;
}

export interface EventPage {
    total: number;
    list: OctopEvent[];
    pageNum: number;
    pageSize: number;
    gaps: EventGap[];
}

export interface EventSummary {
    groupKey: string;
    displayName: string;
    total: number;
    failCount: number;
    inputTokens: number;
    outputTokens: number;
    lastActivity?: string;
}

export interface ExistingBind {
    instanceId: number;
    instanceName: string;
    instanceCode: string;
}

export interface BindCandidate {
    instanceId: number;
    instanceName: string;
    instanceCode: string;
    callbackUrl: string;
    apiUrl: string;
    matchedRedirectUri: string;
    matchBasis: string;
    normalizedCallback: string;
    normalizedRedirect: string;
    currentAppSnapshotId: number;
}

export interface AppBindMatch {
    appSnapshotId: number;
    sourceId: string;
    name: string;
    displayName: string;
    redirectUris: string[];
    existing: ExistingBind[];
    matchStatus: string;
    candidates: BindCandidate[];
}

export interface OctopUser {
    id: string;
    username: string;
    displayName: string;
    email: string;
    status: string;
}

export interface OctopUserPage {
    total: number;
    list: OctopUser[];
    pageNum: number;
    pageSize: number;
    status: string;
    message: string;
    filterMode: string;
    filterNote: string;
}

export interface OctopSettings {
    eventRetainDays: number;
    metricSampleSec: number;
    collectConcurrency: number;
    collectTimeoutSec: number;
}
