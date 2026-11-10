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

export interface OctopSettings {
    eventRetainDays: number;
    metricSampleSec: number;
    collectConcurrency: number;
    collectTimeoutSec: number;
}

// ---------- Business (S1-S3) — 字段与后端 entity/dto JSON 对齐 ----------

/** 来源/功能状态：ready / not_configured / not_verified / unsupported / forbidden / failed / partial 等 */
export type OctopSourceState = string;

/** 统计值：value 为 null 表示不可用（失败/未配置/未支持），不得当 0 展示 */
export interface StatValue {
    value: number | null;
    state: OctopSourceState;
    asOf?: string;
    definition?: string;
}

export interface SourceMeta {
    source: string;
    state: OctopSourceState;
    readAt?: string;
    lastSuccessAt?: string;
    complete?: boolean;
    coverageStart?: string;
    message?: string;
}

/** 业务分页：list/total + 来源元数据 */
export interface BizPage<T> {
    list: T[];
    total: number;
    pageNum?: number;
    pageSize?: number;
    source?: SourceMeta;
}

/** 业务详情：item + 来源元数据 */
export interface BizDetail<T> {
    item?: T | null;
    source?: SourceMeta;
}

export interface CapabilityItem {
    code?: string;
    state: OctopSourceState;
    message?: string;
    verifiedAt?: string;
}

export interface Capabilities {
    instanceId?: number;
    businessDb?: CapabilityItem;
    octopPreview?: CapabilityItem;
    langfuse?: CapabilityItem;
}

export interface BizUserOption {
    userId: string;
    username: string;
    displayName: string;
}

export interface BizUser {
    userId: string;
    username: string;
    displayName: string;
    /** 1 启用 / -1 禁用 / 0 未知 */
    enabled: number;
    roleSummary?: string;
    expertCount: StatValue;
    threadCount: StatValue;
    lastActive?: string;
    createdAt?: string;
}

export interface BizUserDetail {
    user?: BizUser | null;
    expertCount: StatValue;
    threadCount: StatValue;
    templateCount: StatValue;
    skillPackageCount: StatValue;
    source?: SourceMeta;
}

export interface BizRefSummary {
    id: string;
    name?: string;
    /** ok / missing */
    status?: string;
}

export interface BizExpertConfig {
    description?: string;
    modelRef?: string;
    providerName?: string;
    providerId?: string;
    enabled?: number;
    lastState?: string;
    temperature?: number | null;
    topP?: number | null;
    maxTokens?: number | null;
    maxIterations?: number | null;
    inputLength?: number | null;
    trajectoryOn?: boolean | null;
    skillPackages?: BizRefSummary[];
    knowledgeBases?: BizRefSummary[];
    mcpConnections?: BizRefSummary[];
    subExperts?: BizRefSummary[];
    unsupported?: string[];
    /** 仅 content 权限按需返回 */
    promptPreview?: string;
}

export interface BizExpert {
    expertId: string;
    name: string;
    description?: string;
    model?: string;
    /** 1 启用 / -1 禁用 / 0 未知 */
    enabled: number;
    /** 数据库最近记录状态，不代表实时运行 */
    lastState?: string;
    sourceTemplate?: string;
    skillPkgCount?: number | null;
    createdAt?: string;
    updatedAt?: string;
    config?: BizExpertConfig;
}

export interface BizThread {
    threadId: string;
    title?: string;
    agentId?: string;
    agentName?: string;
    channel?: string;
    createdAt?: string;
    lastActive?: string;
    hasMessages?: boolean;
    /** 未查过不能声称无追踪 */
    traceStatus?: string;
}

export interface BizFileEntry {
    /** 服务端返回的安全文件条目 ID，不是路径 */
    fileId: string;
    name: string;
    path?: string;
    size?: number;
    mimeType?: string;
}

export interface BizFileContent {
    fileId: string;
    name?: string;
    content: string;
    truncated?: boolean;
    mimeType?: string;
}

export interface BizExpertTemplate {
    templateId: string;
    slug?: string;
    name: string;
    description?: string;
    avatar?: string;
    createdBy: string;
    sourceExpertId?: string;
    sourceAvailable?: boolean | null;
    createdAt?: string;
    updatedAt?: string;
    files?: BizFileEntry[];
}

export interface BizSkillPackage {
    packageId: string;
    name: string;
    description?: string;
    icon?: string;
    createdBy: string;
    skillCount?: StatValue;
    shared?: boolean;
    visibility?: string;
    copyPolicy?: string;
    downloadPolicy?: string;
    updatedAt?: string;
}

export interface BizSkill {
    slug: string;
    name: string;
    description?: string;
    source?: string;
    files?: BizFileEntry[];
}

export interface BizTrace {
    traceId: string;
    name?: string;
    sessionId?: string;
    userId?: string;
    startTime?: string;
    endTime?: string;
    latencyMs?: number | null;
    inputTokens?: number | null;
    outputTokens?: number | null;
    level?: string;
    hasError?: boolean;
    tags?: string[];
}

export interface BizObservation {
    observationId: string;
    traceId: string;
    parentId?: string;
    type: string;
    name?: string;
    startTime?: string;
    endTime?: string;
    latencyMs?: number | null;
    model?: string;
    inputTokens?: number | null;
    outputTokens?: number | null;
    level?: string;
    statusMessage?: string;
    hasContent?: boolean;
    orphan?: boolean;
}

export interface BizObservationContent {
    observationId: string;
    input?: string;
    output?: string;
    truncated?: boolean;
}

export interface TracePage<T> {
    list: T[];
    total: number;
    pageNum: number;
    pageSize: number;
    nextCursor?: string;
    hasMore?: boolean;
    source?: SourceMeta;
    association?: string;
    mapping?: string;
    coverageStart?: string;
    coverageEnd?: string;
    externalUrl?: string;
}

export interface ThreadTraceMeta {
    thread?: BizThread;
    source?: SourceMeta;
    association: string;
    mapping?: string;
    projectId?: string;
    externalUrl?: string;
    message?: string;
}

export type ReadConnectionType = 'octop_api' | 'langfuse';

export interface ReadConnectionView {
    id?: number;
    instanceId?: number;
    type: ReadConnectionType | string;
    baseUrl?: string;
    projectId?: string;
    publicKey?: string;
    credentialConfigured?: boolean;
    credentialRotatedAt?: string;
    revision?: number;
    verifyState?: OctopSourceState;
    verifiedAt?: string;
    verifyError?: string;
}

export interface ReadConnectionSaveForm {
    type: ReadConnectionType | string;
    baseUrl: string;
    projectId?: string;
    publicKey?: string;
    /** 留空保留已保存凭据 */
    secret?: string;
    clearSecret?: boolean;
    revision?: number;
}

export interface ReadConnectionTestResult {
    ok?: boolean;
    state: OctopSourceState;
    message?: string;
    at?: string;
}
