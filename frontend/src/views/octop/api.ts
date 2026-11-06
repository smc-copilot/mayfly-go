import Api from '@/common/Api';
import type { PageResult } from '@/types/common';
import type {
    CasdoorApp,
    CasdoorAuth,
    CasdoorConnection,
    CasdoorSyncResult,
    CasdoorTestResult,
    AppBindMatch,
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
    OctopUserPage,
} from './types';

export const octopInstanceApi = {
    overview: Api.newGet<InstanceOverview[]>('/octop/overview'),
    list: Api.newGet<PageResult<InstanceOverview>>('/octop/instances'),
    detail: Api.newGet<InstanceDetail>('/octop/instances/{id}'),
    metrics: Api.newGet<MetricSample[]>('/octop/instances/{id}/metrics'),
    testConn: Api.newPost<ConnTestResult>('/octop/instances/test-conn'),
    save: Api.newPost<number, Partial<OctopInstance> & { apiToken?: string }>('/octop/instances'),
    del: Api.newDelete<void>('/octop/instances/{id}'),
    restart: Api.newPost<Record<string, unknown>>('/octop/instances/{id}/restart'),
    bindApp: Api.newPost<void, { instanceId?: number; appSnapshotId?: number; confirmOverwrite?: boolean }>('/octop/instances/bind-app'),
    users: Api.newGet<OctopUserPage>('/octop/instances/{id}/users'),
    settings: Api.newGet<OctopSettings>('/octop/settings'),
    saveSettings: Api.newPost<void, Partial<OctopSettings>>('/octop/settings'),
};

export const octopCasdoorApi = {
    connections: Api.newGet<CasdoorConnection[]>('/octop/casdoor/connections'),
    test: Api.newPost<CasdoorTestResult>('/octop/casdoor/connections/test'),
    save: Api.newPost<number>('/octop/casdoor/connections'),
    sync: Api.newPost<CasdoorSyncResult>('/octop/casdoor/connections/{id}/sync'),
    apps: Api.newGet<PageResult<CasdoorApp>>('/octop/casdoor/apps'),
    bindMatches: Api.newGet<AppBindMatch[]>('/octop/casdoor/bind-matches'),
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
