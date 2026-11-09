import Api from '@/common/Api';
import type { PageResult } from '@/types/common';
import type { Application, User, AuthorizationObject, Operation } from './api';

export interface Service {
    id: number; name: string; serverUrl: string; clientId: string; authType: string;
    organizationCount: number; status: string; reasonCode: string; verifiedAt?: string;
    timeoutSec: number; syncIntervalSec: number; tlsVerify: number;
}
export interface ServiceForm {
    id?: number; name: string; serverUrl: string; clientId: string; clientSecret: string;
    timeoutSec: number; syncIntervalSec: number; tlsVerify: number; legacyConnectionId?: number;
}
export interface DirectoryStatus { status: string; attemptAt?: string; successAt?: string; count: number; reasonCode?: string }
export interface Organization { id: number; serviceId: number; name: string; displayName: string; present: number; lastSyncAt?: string; syncStates?: Record<string, DirectoryStatus> }
export interface DiscoveredOrganization { name: string; sourceId: string; displayName: string }
export interface ServiceContext { serviceId: number; organizationId: number }
export interface ServiceQuery extends ServiceContext { keyword?: string; pageNum: number; pageSize: number; kind?: string }
export interface CategoryResult { status: string; count: number; reasonCode?: string }
const base = '/casdoor/services/{serviceId}/organizations/{organizationId}';
export const serviceApi = {
    capabilities: Api.newGet<{ canConfigure: boolean }>('/casdoor/service-capabilities'),
    list: Api.newGet<Service[]>('/casdoor/services'),
    save: Api.newPost<number, ServiceForm>('/casdoor/services'),
    test: Api.newPost<DiscoveredOrganization[], ServiceForm>('/casdoor/services/test'),
    organizations: Api.newGet<Organization[], { serviceId: number }>('/casdoor/services/{serviceId}/organizations'),
    discover: Api.newPost<Organization[], { serviceId: number }>('/casdoor/services/{serviceId}/discover'),
    sync: Api.newPost<Record<string, CategoryResult>, ServiceContext>(base + '/sync'),
    applications: Api.newGet<PageResult<Application>, ServiceQuery>(base + '/applications'),
    users: Api.newGet<PageResult<User>, ServiceQuery>(base + '/users'),
    objects: Api.newGet<PageResult<AuthorizationObject>, ServiceQuery>(base + '/objects'),
    operations: Api.newGet<PageResult<Operation>, ServiceQuery>(base + '/operations'),
};
