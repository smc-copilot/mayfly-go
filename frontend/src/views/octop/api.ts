import Api from '@/common/Api';
import type { PageResult } from '@/types/common';
import type {
    BizDetail,
    BizExpert,
    BizExpertTemplate,
    BizFileContent,
    BizPage,
    BizSkill,
    BizSkillPackage,
    BizThread,
    BizUser,
    BizUserDetail,
    BizUserOption,
    Capabilities,
    CasdoorApp,
    CasdoorAuth,
    CasdoorConnection,
    CasdoorSyncResult,
    CasdoorTestResult,
    CasdoorUser,
    ConnTestResult,
    EventPage,
    EventSummary,
    IdentityLink,
    InstanceDetail,
    InstanceOverview,
    MetricSample,
    OctopInstance,
    OctopSettings,
    ReadConnectionSaveForm,
    ReadConnectionTestResult,
    ReadConnectionView,
} from './types';

export const octopInstanceApi = {
    overview: Api.newGet<InstanceOverview[]>('/octop/overview'),
    list: Api.newGet<PageResult<InstanceOverview>>('/octop/instances'),
    detail: Api.newGet<InstanceDetail>('/octop/instances/{id}'),
    metrics: Api.newGet<MetricSample[]>('/octop/instances/{id}/metrics'),
    testConn: Api.newPost<ConnTestResult>('/octop/instances/test-conn'),
    save: Api.newPost<number, Partial<OctopInstance>>('/octop/instances'),
    del: Api.newDelete<void>('/octop/instances/{id}'),
    restart: Api.newPost<Record<string, unknown>>('/octop/instances/{id}/restart'),
    bindApp: Api.newPost<void>('/octop/instances/bind-app'),
    settings: Api.newGet<OctopSettings>('/octop/settings'),
    saveSettings: Api.newPost<void, Partial<OctopSettings>>('/octop/settings'),
};

export const octopCasdoorApi = {
    connections: Api.newGet<CasdoorConnection[]>('/octop/casdoor/connections'),
    test: Api.newPost<CasdoorTestResult>('/octop/casdoor/connections/test'),
    save: Api.newPost<number>('/octop/casdoor/connections'),
    sync: Api.newPost<CasdoorSyncResult>('/octop/casdoor/connections/{id}/sync'),
    apps: Api.newGet<PageResult<CasdoorApp>>('/octop/casdoor/apps'),
    users: Api.newGet<PageResult<CasdoorUser>>('/octop/casdoor/users'),
    auths: Api.newGet<CasdoorAuth[]>('/octop/casdoor/auths'),
    links: Api.newGet<IdentityLink[]>('/octop/casdoor/identity-links'),
    candidates: Api.newGet<CasdoorUser[]>('/octop/casdoor/identity-candidates'),
    saveLink: Api.newPost<void>('/octop/casdoor/identity-links'),
    deleteLink: Api.newDelete<void>('/octop/casdoor/identity-links/{id}'),
};

export const octopEventApi = {
    query: Api.newGet<EventPage>('/octop/events'),
    summary: Api.newGet<EventSummary[]>('/octop/events/summary'),
    recollect: Api.newPost<void>('/octop/events/recollect'),
};

const BIZ = '/octop/instances/{id}/business';
const BIZ_USER = `${BIZ}/users/{userId}`;

/**
 * 业务读取（只读）。路径参数 {id}=实例 ID、{userId}=Octop 用户 ID 等，由 Api 模板解析，
 * 其余参数作为 query（pageNum/pageSize/keyword 等）。
 */
export const octopBusinessApi = {
    capabilities: Api.newGet<Capabilities>(`${BIZ}/capabilities`),
    userOptions: Api.newGet<BizPage<BizUserOption>>(`${BIZ}/user-options`),
    users: Api.newGet<BizPage<BizUser>>(`${BIZ}/users`),
    user: Api.newGet<BizUserDetail>(BIZ_USER),
    experts: Api.newGet<BizPage<BizExpert>>(`${BIZ_USER}/experts`),
    expert: Api.newGet<BizDetail<BizExpert>>(`${BIZ_USER}/experts/{expertId}`),
    threads: Api.newGet<BizPage<BizThread>>(`${BIZ_USER}/threads`),
    thread: Api.newGet<BizDetail<BizThread>>(`${BIZ_USER}/threads/{threadId}`),
    expertTemplates: Api.newGet<BizPage<BizExpertTemplate>>(`${BIZ_USER}/expert-templates`),
    expertTemplate: Api.newGet<BizDetail<BizExpertTemplate>>(`${BIZ_USER}/expert-templates/{templateId}`),
    expertTemplateFile: Api.newGet<BizFileContent>(`${BIZ_USER}/expert-templates/{templateId}/files/{fileId}/content`),
    skillPackages: Api.newGet<BizPage<BizSkillPackage>>(`${BIZ_USER}/skill-packages`),
    skillPackage: Api.newGet<BizDetail<BizSkillPackage>>(`${BIZ_USER}/skill-packages/{packageId}`),
    skills: Api.newGet<BizPage<BizSkill>>(`${BIZ_USER}/skill-packages/{packageId}/skills`),
    skill: Api.newGet<BizDetail<BizSkill>>(`${BIZ_USER}/skill-packages/{packageId}/skills/{slug}`),
    skillFile: Api.newGet<BizFileContent>(`${BIZ_USER}/skill-packages/{packageId}/skills/{slug}/files/{fileId}/content`),
};

export const octopReadConnectionApi = {
    /** 带 type 返回单个；不带 type 返回两类配置数组 */
    get: Api.newGet<ReadConnectionView | ReadConnectionView[]>('/octop/instances/{id}/read-connection'),
    save: Api.newPost<ReadConnectionView, ReadConnectionSaveForm & { id: number }>('/octop/instances/{id}/read-connection'),
    test: Api.newPost<ReadConnectionTestResult, ReadConnectionSaveForm & { id: number }>('/octop/instances/{id}/read-connection/test'),
};
