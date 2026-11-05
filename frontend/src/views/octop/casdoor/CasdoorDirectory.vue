<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-medium">{{ $t('octop.casdoorDirectory') }}</h2>
            <el-button v-auth="'octop:casdoor:save'" type="primary" @click="onOpenConn">{{ $t('octop.connection') }}</el-button>
        </div>

        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.directoryReadOnlyHint')" />
        <el-alert type="warning" :closable="false" class="mb-3" :title="$t('octop.noAutoMerge')" />
        <el-alert v-if="isHttpEndpoint" type="warning" :closable="false" class="mb-3" :title="$t('octop.httpNoTlsHint')" />
        <el-alert v-if="tlsOffOnHttps" type="warning" :closable="false" class="mb-3" :title="$t('octop.tlsVerifyOffHint')" />
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.syncIntervalHint')" />

        <el-card class="mb-4" shadow="never">
            <div v-if="conn" class="text-sm space-y-2">
                <div>{{ conn.name }} · {{ conn.serverUrl }} / {{ conn.organization }} · {{ $t('octop.appOwner') }}: {{ conn.appOwner || $t('octop.notProvided') }}</div>
                <div>
                    {{ $t('octop.clientSecret') }}:
                    {{ conn.credentialConfigured === 1 ? $t('octop.secretConfigured') : $t('octop.secretEmpty') }}
                    · {{ $t('octop.rotatedAt') }}: {{ formatShanghai(conn.credentialRotatedAt) }}
                </div>
                <div class="grid gap-3 md:grid-cols-3">
                    <div class="rounded border border-gray-200 dark:border-gray-700 p-3 space-y-1">
                        <div class="flex items-center justify-between gap-2">
                            <span class="font-medium">{{ $t('octop.apps') }}</span>
                            <enum-tag :enums="OctopSyncStatusEnum" :value="conn.appSyncStatus || 'never'" />
                        </div>
                        <div class="text-xs text-gray-500">{{ $t('octop.lastAttempt') }}: {{ formatShanghai(conn.appSyncAt) }}</div>
                        <div class="text-xs text-gray-500">{{ $t('octop.lastSuccess') }}: {{ formatShanghai(conn.appSuccessAt) }}</div>
                        <div class="text-xs text-gray-500">{{ $t('octop.syncCount') }}: {{ conn.appSyncCount || 0 }} · {{ $t('octop.syncDuration') }}: {{ conn.appSyncDurationMs || 0 }}ms</div>
                        <div v-if="conn.appSyncError" class="text-xs text-red-500 break-all">{{ conn.appSyncError }}</div>
                    </div>
                    <div class="rounded border border-gray-200 dark:border-gray-700 p-3 space-y-1">
                        <div class="flex items-center justify-between gap-2">
                            <span class="font-medium">{{ $t('octop.users') }}</span>
                            <enum-tag :enums="OctopSyncStatusEnum" :value="conn.userSyncStatus || 'never'" />
                        </div>
                        <div class="text-xs text-gray-500">{{ $t('octop.lastAttempt') }}: {{ formatShanghai(conn.userSyncAt) }}</div>
                        <div class="text-xs text-gray-500">{{ $t('octop.lastSuccess') }}: {{ formatShanghai(conn.userSuccessAt) }}</div>
                        <div class="text-xs text-gray-500">{{ $t('octop.syncCount') }}: {{ conn.userSyncCount || 0 }} · {{ $t('octop.syncDuration') }}: {{ conn.userSyncDurationMs || 0 }}ms</div>
                        <div v-if="conn.userSyncError" class="text-xs text-red-500 break-all">{{ conn.userSyncError }}</div>
                    </div>
                    <div class="rounded border border-gray-200 dark:border-gray-700 p-3 space-y-1">
                        <div class="flex items-center justify-between gap-2">
                            <span class="font-medium">{{ $t('octop.relations') }}</span>
                            <enum-tag :enums="OctopSyncStatusEnum" :value="conn.relationSyncStatus || 'never'" />
                        </div>
                        <div class="text-xs text-gray-500">{{ $t('octop.lastAttempt') }}: {{ formatShanghai(conn.relationSyncAt) }}</div>
                        <div class="text-xs text-gray-500">{{ $t('octop.lastSuccess') }}: {{ formatShanghai(conn.relationSuccessAt) }}</div>
                        <div class="text-xs text-gray-500">{{ $t('octop.relationStats', { confirmed: conn.relationConfirmedCount || 0, unverified: conn.relationUnverifiedCount || 0, failed: conn.relationFailedCount || 0 }) }}</div>
                        <div class="text-xs text-gray-500">{{ $t('octop.syncDuration') }}: {{ conn.relationSyncDurationMs || 0 }}ms</div>
                        <div v-if="conn.relationSyncError" class="text-xs text-red-500 break-all">{{ conn.relationSyncError }}</div>
                    </div>
                </div>
                <div v-if="testResult" class="pt-2 space-y-1">
                    <div class="font-medium">{{ $t('octop.testResult') }}</div>
                    <div class="flex flex-wrap gap-4">
                        <div class="flex items-center gap-2">
                            <span>{{ $t('octop.testService') }}</span>
                            <enum-tag :enums="OctopProbeEnum" :value="testResult.service?.status || 'unknown'" />
                            <span v-if="testResult.service?.reason" class="text-xs text-red-500 break-all">{{ testResult.service.reason }}</span>
                        </div>
                        <div class="flex items-center gap-2">
                            <span>{{ $t('octop.testApps') }}</span>
                            <enum-tag :enums="OctopProbeEnum" :value="testResult.apps?.status || 'unknown'" />
                            <span v-if="testResult.apps?.reason" class="text-xs text-red-500 break-all">{{ testResult.apps.reason }}</span>
                        </div>
                        <div class="flex items-center gap-2">
                            <span>{{ $t('octop.testUsers') }}</span>
                            <enum-tag :enums="OctopProbeEnum" :value="testResult.users?.status || 'unknown'" />
                            <span v-if="testResult.users?.reason" class="text-xs text-red-500 break-all">{{ testResult.users.reason }}</span>
                        </div>
                    </div>
                </div>
                <div class="pt-2 flex gap-2">
                    <el-button v-auth="'octop:casdoor:save'" @click="onTest">{{ $t('octop.testRead') }}</el-button>
                    <el-button v-auth="'octop:casdoor:save'" type="primary" @click="onSync">{{ $t('octop.syncNow') }}</el-button>
                </div>
            </div>
            <div v-else class="text-gray-500">{{ $t('octop.secretEmpty') }}</div>
        </el-card>

        <el-tabs v-model="tab">
            <el-tab-pane :label="$t('octop.apps')" name="apps">
                <data-table ref="appTableRef" :page-api="octopCasdoorApi.apps" :search-items="appSearch" v-model:query-form="appQuery" :columns="appColumns">
                    <template #enabled="{ data }">
                        <enum-tag :enums="enableEnum" :value="data.enabled" />
                    </template>
                    <template #present="{ data }">
                        {{ data.present === 1 ? $t('octop.present') : $t('octop.absent') }}
                    </template>
                    <template #relationStatus="{ data }">
                        <enum-tag :enums="OctopAuthStatusEnum" :value="data.relationStatus || 'unverified'" />
                    </template>
                    <template #lastSyncAt="{ data }">
                        {{ formatShanghai(data.lastSyncAt) }}
                    </template>
                </data-table>
            </el-tab-pane>
            <el-tab-pane :label="$t('octop.users')" name="users">
                <data-table ref="userTableRef" :page-api="octopCasdoorApi.users" :search-items="userSearch" v-model:query-form="userQuery" :columns="userColumns">
                    <template #enabled="{ data }">
                        <enum-tag :enums="enableEnum" :value="data.enabled" />
                    </template>
                    <template #email="{ data }">
                        {{ data.email || $t('octop.notProvided') }}
                    </template>
                    <template #lastSyncAt="{ data }">
                        {{ formatShanghai(data.lastSyncAt) }}
                    </template>
                </data-table>
            </el-tab-pane>
            <el-tab-pane :label="$t('octop.authRelations')" name="auths">
                <div class="mb-2 flex gap-2">
                    <el-select v-model="authAppId" filterable :placeholder="$t('octop.apps')" class="w-64" @change="onLoadAuths">
                        <el-option v-for="item in appOptions" :key="item.sourceId" :label="item.displayName || item.name" :value="item.sourceId" />
                    </el-select>
                </div>
                <el-table :data="auths" size="small">
                    <el-table-column prop="subjectType" :label="$t('octop.category')" />
                    <el-table-column prop="subjectSourceId" :label="$t('octop.casdoorUserId')" />
                    <el-table-column :label="$t('octop.status')">
                        <template #default="{ row }">
                            <enum-tag :enums="OctopAuthStatusEnum" :value="row.status" />
                        </template>
                    </el-table-column>
                    <el-table-column :label="$t('octop.lastConfirmed')">
                        <template #default="{ row }">
                            {{ formatShanghai(row.lastConfirmedAt) }}
                        </template>
                    </el-table-column>
                </el-table>
            </el-tab-pane>
            <el-tab-pane :label="$t('octop.identityLink')" name="links">
                <div class="mb-2 flex gap-2">
                    <el-select v-model="linkInstanceId" :placeholder="$t('octop.selectInstance')" class="w-64" @change="onLoadLinks">
                        <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                    <el-button v-auth="'octop:casdoor:save'" type="primary" @click="linkVisible = true">{{ $t('common.create') }}</el-button>
                </div>
                <el-table :data="links" size="small">
                    <el-table-column prop="casdoorUserSourceId" :label="$t('octop.casdoorUserId')" />
                    <el-table-column prop="octopUserId" :label="$t('octop.octopUserId')" />
                    <el-table-column prop="evidence" :label="$t('octop.evidence')" />
                    <el-table-column :label="$t('common.operation')">
                        <template #default="{ row }">
                            <el-button v-auth="'octop:casdoor:save'" link type="danger" @click="onDeleteLink(row.id)">{{ $t('common.delete') }}</el-button>
                        </template>
                    </el-table-column>
                </el-table>
            </el-tab-pane>
        </el-tabs>

        <el-dialog v-model="editConnVisible" :title="$t('octop.connection')" width="560px" destroy-on-close>
            <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.directoryReadOnlyHint')" />
            <el-alert v-if="formIsHttp" type="warning" :closable="false" class="mb-3" :title="$t('octop.httpNoTlsHint')" />
            <el-form :model="connForm" label-width="140px">
                <el-form-item :label="$t('octop.name')"><el-input v-model="connForm.name" /></el-form-item>
                <el-form-item :label="$t('octop.serverUrl')"><el-input v-model="connForm.serverUrl" /></el-form-item>
                <el-form-item :label="$t('octop.organization')"><el-input v-model="connForm.organization" /></el-form-item>
                <el-form-item :label="$t('octop.appOwner')">
                    <el-input v-model="connForm.appOwner" />
                    <div class="text-xs text-gray-500 mt-1">{{ $t('octop.appOwnerHint') }}</div>
                </el-form-item>
                <el-form-item :label="$t('octop.clientId')"><el-input v-model="connForm.clientId" /></el-form-item>
                <el-form-item :label="$t('octop.clientSecret')">
                    <el-input v-model="connForm.clientSecret" type="password" show-password autocomplete="new-password" :placeholder="$t('octop.secretKeepHint')" />
                </el-form-item>
                <el-form-item :label="$t('octop.timeoutSec')"><el-input-number v-model="connForm.timeoutSec" :min="3" :max="120" /></el-form-item>
                <el-form-item :label="$t('octop.syncInterval')">
                    <el-input-number v-model="connForm.syncIntervalSec" :min="60" />
                    <div class="text-xs text-gray-500 mt-1">{{ $t('octop.syncIntervalHint') }}</div>
                </el-form-item>
                <el-form-item :label="$t('octop.tlsVerify')"><el-switch v-model="tlsOn" /></el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="editConnVisible = false">{{ $t('common.cancel') }}</el-button>
                <el-button v-auth="'octop:casdoor:save'" @click="onTestForm">{{ $t('octop.testRead') }}</el-button>
                <el-button type="primary" @click="onSaveConn">{{ $t('common.save') }}</el-button>
            </template>
        </el-dialog>

        <el-dialog v-model="linkVisible" :title="$t('octop.identityLink')" width="520px" destroy-on-close>
            <el-form :model="linkForm" label-width="160px">
                <el-form-item :label="$t('octop.casdoorUserId')"><el-input v-model="linkForm.casdoorUserSourceId" /></el-form-item>
                <el-form-item :label="$t('octop.octopUserId')"><el-input v-model="linkForm.octopUserId" /></el-form-item>
                <el-form-item :label="$t('octop.evidence')"><el-input v-model="linkForm.evidence" type="textarea" /></el-form-item>
            </el-form>
            <p class="text-xs text-gray-500">{{ $t('octop.candidates') }}</p>
            <template #footer>
                <el-button @click="linkVisible = false">{{ $t('common.cancel') }}</el-button>
                <el-button type="primary" @click="onSaveLink">{{ $t('common.save') }}</el-button>
            </template>
        </el-dialog>

        <el-dialog v-model="bindVisible" :title="$t('octop.bindInstance')" width="480px">
            <el-select v-model="bindInstanceId" class="w-full">
                <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
            <template #footer>
                <el-button @click="bindVisible = false">{{ $t('common.cancel') }}</el-button>
                <el-button type="primary" @click="onBind">{{ $t('common.save') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { TableColumn } from '@/components/data-table';
import DataTable from '@/components/data-table/DataTable.vue';
import { SearchItem } from '@/components/data-table/SearchForm';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import EnumValue from '@/common/Enum';
import { Msg, useI18nDeleteConfirm } from '@/hooks/useI18n';
import { octopCasdoorApi, octopInstanceApi } from '../api';
import { OctopAuthStatusEnum, OctopProbeEnum, OctopSyncStatusEnum } from '../enums';
import type { CasdoorApp, CasdoorAuth, CasdoorConnection, CasdoorTestResult, IdentityLink, InstanceOverview } from '../types';
import { formatShanghai } from '../time';

const enableEnum = {
    Enable: EnumValue.of(1, 'common.enable').tagTypeSuccess(),
    Disable: EnumValue.of(-1, 'common.disable').tagTypeDanger(),
    Unknown: EnumValue.of(0, 'octop.statusUnknown').tagTypeInfo(),
};

const conn = ref<CasdoorConnection | null>(null);
const tab = ref('apps');
const appTableRef = ref();
const userTableRef = ref();
const editConnVisible = ref(false);
const auths = ref<CasdoorAuth[]>([]);
const authAppId = ref('');
const appOptions = ref<CasdoorApp[]>([]);
const instances = ref<InstanceOverview[]>([]);
const links = ref<IdentityLink[]>([]);
const linkInstanceId = ref<number>();
const linkVisible = ref(false);
const bindVisible = ref(false);
const bindAppId = ref(0);
const bindInstanceId = ref<number>();
const testResult = ref<CasdoorTestResult | null>(null);

const connForm = reactive({
    id: 0,
    name: '',
    serverUrl: '',
    organization: '',
    appOwner: '',
    clientId: '',
    clientSecret: '',
    timeoutSec: 15,
    syncIntervalSec: 3600,
    tlsVerify: 1,
});
const tlsOn = computed({
    get: () => connForm.tlsVerify !== -1,
    set: (v) => {
        connForm.tlsVerify = v ? 1 : -1;
    },
});
const isHttpEndpoint = computed(() => (conn.value?.serverUrl || '').toLowerCase().startsWith('http://'));
const formIsHttp = computed(() => connForm.serverUrl.toLowerCase().startsWith('http://'));
const tlsOffOnHttps = computed(() => (conn.value?.serverUrl || '').toLowerCase().startsWith('https://') && conn.value?.tlsVerify === -1);
const linkForm = reactive({ casdoorUserSourceId: '', octopUserId: '', evidence: '' });

const appQuery = ref({ connectionId: 0, keyword: '', enabled: undefined as number | undefined, pageNum: 1, pageSize: 10 });
const userQuery = ref({ connectionId: 0, keyword: '', enabled: undefined as number | undefined, pageNum: 1, pageSize: 10 });
const enabledOptions = [
    { value: 1, label: 'common.enable' },
    { value: -1, label: 'common.disable' },
    { value: 0, label: 'octop.statusUnknown' },
];
const appSearch = [SearchItem.input('keyword', 'octop.keyword'), SearchItem.select('enabled', 'octop.enabled').withOptions(enabledOptions)];
const userSearch = [SearchItem.input('keyword', 'octop.keyword'), SearchItem.select('enabled', 'octop.enabled').withOptions(enabledOptions)];

const appColumns = [
    TableColumn.new('sourceId', 'octop.sourceId'),
    TableColumn.new('name', 'octop.name'),
    TableColumn.new('displayName', 'octop.displayName'),
    TableColumn.new('organization', 'octop.organization'),
    TableColumn.new('appOwner', 'octop.appOwner'),
    TableColumn.new('enabled', 'octop.enabled').isSlot(),
    TableColumn.new('relationStatus', 'octop.relationStatus').isSlot(),
    TableColumn.new('present', 'octop.present').isSlot(),
    TableColumn.new('lastSyncAt', 'octop.lastSuccess').isSlot(),
    TableColumn.new('action', 'common.operation')
        .setActions<CasdoorApp>([{ key: 'bind', label: 'octop.bindInstance', perm: 'octop:casdoor:save', onClick: (row) => onOpenBind(row) }])
        .fixedRight(),
];
const userColumns = [
    TableColumn.new('sourceId', 'octop.casdoorUserId'),
    TableColumn.new('username', 'octop.user'),
    TableColumn.new('displayName', 'octop.displayName'),
    TableColumn.new('organization', 'octop.organization'),
    TableColumn.new('email', 'common.email').isSlot(),
    TableColumn.new('enabled', 'octop.enabled').isSlot(),
    TableColumn.new('lastSyncAt', 'octop.lastSuccess').isSlot(),
];

const loadConn = async () => {
    const list = (await octopCasdoorApi.connections.request()) || [];
    conn.value = list[0] || null;
    if (conn.value) {
        appQuery.value.connectionId = conn.value.id;
        userQuery.value.connectionId = conn.value.id;
        Object.assign(connForm, conn.value, { clientSecret: '' });
        const apps = await octopCasdoorApi.apps.request({ connectionId: conn.value.id, pageNum: 1, pageSize: 200 });
        appOptions.value = apps?.list || [];
    }
    appTableRef.value?.search();
    userTableRef.value?.search();
};

const onLoadAuths = async () => {
    if (!conn.value || !authAppId.value) {
        auths.value = [];
        return;
    }
    auths.value = (await octopCasdoorApi.auths.request({ connectionId: conn.value.id, applicationSourceId: authAppId.value })) || [];
};

const onTest = async () => {
    if (!conn.value) return;
    testResult.value = await octopCasdoorApi.test.request({ ...conn.value, clientSecret: '' });
};

const onTestForm = async () => {
    testResult.value = await octopCasdoorApi.test.request({ ...connForm });
};

const onSync = async () => {
    if (!conn.value) return;
    try {
        await octopCasdoorApi.sync.request({ id: conn.value.id });
        Msg.success('common.success');
    } finally {
        await loadConn();
        await onLoadAuths();
    }
};

const onOpenConn = () => {
    if (conn.value) {
        Object.assign(connForm, conn.value, { clientSecret: '' });
    }
    editConnVisible.value = true;
};

const onSaveConn = async () => {
    await octopCasdoorApi.save.request(connForm);
    editConnVisible.value = false;
    await loadConn();
};

const onLoadLinks = async () => {
    if (!linkInstanceId.value) return;
    links.value = (await octopCasdoorApi.links.request({ instanceId: linkInstanceId.value })) || [];
};

const onSaveLink = async () => {
    if (!conn.value || !linkInstanceId.value) return;
    await octopCasdoorApi.saveLink.request({
        connectionId: conn.value.id,
        octopInstanceId: linkInstanceId.value,
        ...linkForm,
    });
    linkVisible.value = false;
    await onLoadLinks();
};

const onDeleteLink = async (id: number) => {
    if (!(await useI18nDeleteConfirm(''))) return;
    await octopCasdoorApi.deleteLink.request({ id });
    await onLoadLinks();
};

const onOpenBind = (row: CasdoorApp) => {
    bindAppId.value = row.id;
    bindVisible.value = true;
};

const onBind = async () => {
    if (!bindInstanceId.value) return;
    await octopInstanceApi.bindApp.request({ instanceId: bindInstanceId.value, appSnapshotId: bindAppId.value });
    bindVisible.value = false;
    Msg.success('common.success');
};

onMounted(async () => {
    await loadConn();
    instances.value = (await octopInstanceApi.overview.request()) || [];
    if (instances.value[0]) {
        linkInstanceId.value = instances.value[0].id;
        await onLoadLinks();
    }
});
</script>
