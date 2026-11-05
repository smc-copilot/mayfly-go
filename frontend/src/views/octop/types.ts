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

export interface CasdoorConnection {
    id: number;
    name: string;
    serverUrl: string;
    organization: string;
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
}

export interface CasdoorApp {
    id: number;
    connectionId: number;
    sourceId: string;
    name: string;
    displayName: string;
    organization: string;
    enabled: number;
    appType: string;
    homepageUrl: string;
    present: number;
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

export interface OctopSettings {
    eventRetainDays: number;
    metricSampleSec: number;
    collectConcurrency: number;
    collectTimeoutSec: number;
}
