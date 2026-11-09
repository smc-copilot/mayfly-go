import Api from '@/common/Api';
import type { PageResult } from '@/types/common';

export interface Connection {
    id: number;
    name: string;
    serverUrl: string;
    organization: string;
    appOwner: string;
    clientId: string;
    credentialConfigured: number;
    managementEnabled: boolean;
    timeoutSec: number;
    syncIntervalSec: number;
    tlsVerify: number;
    appSyncStatus: string;
    userSyncStatus: string;
}

export type ConnectionForm = Partial<Connection> & { clientSecret: string };
export interface ProbeItem { status: string; reasonCode: string }
export interface ProbeResult { service: ProbeItem; apps: ProbeItem; users: ProbeItem }
export interface DirectoryQuery { connectionId: number; keyword: string; pageNum: number; pageSize: number }
export interface Application { id: number; name: string; displayName: string; organization: string; appOwner: string; clientId: string; enabled: number; present: number; lastSyncAt: string }
export interface User { id: number; username: string; displayName: string; email: string; organization: string; sourceId: string; enabled: number; present: number; lastSyncAt: string }
export interface Operation { id: number; actorId: number; action: string; status: string; submittedAt: string; verifiedAt: string }
export interface AuthorizationObject { id: number; sourceId: string; projection: string; present: number; lastSyncAt: string }
export interface ObjectQuery { connectionId: number; kind: 'role' | 'group' | 'permission'; pageNum: number; pageSize: number }

export const casdoorApi = {
    connections: Api.newGet<Connection[]>('/casdoor/connections'),
    save: Api.newPost<number, ConnectionForm>('/casdoor/connections'),
    test: Api.newPost<ProbeResult, ConnectionForm>('/casdoor/connections/test'),
    sync: Api.newPost<Record<string, unknown>, { id: number }>('/casdoor/connections/{id}/sync'),
    applications: Api.newGet<PageResult<Application>, DirectoryQuery>('/casdoor/applications'),
    users: Api.newGet<PageResult<User>, DirectoryQuery>('/casdoor/users'),
    operations: Api.newGet<PageResult<Operation>, DirectoryQuery>('/casdoor/operations'),
    objects: Api.newGet<PageResult<AuthorizationObject>, ObjectQuery>('/casdoor/objects'),
};
