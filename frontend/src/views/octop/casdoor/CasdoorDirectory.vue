<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-medium">{{ $t('octop.casdoorDirectory') }}</h2>
            <el-button v-auth="'octop:casdoor:save'" type="primary" @click="editConnVisible = true">{{ $t('octop.connection') }}</el-button>
        </div>

        <el-alert type="warning" :closable="false" class="mb-3" :title="$t('octop.noAutoMerge')" />

        <el-card class="mb-4" shadow="never">
            <div v-if="conn" class="text-sm space-y-1">
                <div>{{ conn.name }} · {{ conn.serverUrl }} / {{ conn.organization }}</div>
                <div>
                    {{ $t('octop.clientSecret') }}:
                    {{ conn.credentialConfigured === 1 ? $t('octop.secretConfigured') : $t('octop.secretEmpty') }}
                    · {{ $t('octop.rotatedAt') }}: {{ formatShanghai(conn.credentialRotatedAt) }}
                </div>
                <div>
                    {{ $t('octop.syncStatus') }}:
                    <enum-tag :enums="OctopSyncStatusEnum" :value="conn.lastSyncStatus || 'never'" />
                    {{ formatShanghai(conn.lastSyncAt) }}
                    <span v-if="conn.lastSyncError" class="text-red-500 ml-2">{{ conn.lastSyncError }}</span>
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
                </data-table>
            </el-tab-pane>
            <el-tab-pane :label="$t('octop.users')" name="users">
                <data-table ref="userTableRef" :page-api="octopCasdoorApi.users" :search-items="userSearch" v-model:query-form="userQuery" :columns="userColumns">
                    <template #enabled="{ data }">
                        <enum-tag :enums="enableEnum" :value="data.enabled" />
                    </template>
                </data-table>
            </el-tab-pane>
            <el-tab-pane :label="$t('octop.authRelations')" name="auths">
                <div class="mb-2 flex gap-2">
                    <el-select v-model="authAppId" filterable :placeholder="$t('octop.apps')" class="w-64" @change="loadAuths">
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
                </el-table>
            </el-tab-pane>
            <el-tab-pane :label="$t('octop.identityLink')" name="links">
                <div class="mb-2 flex gap-2">
                    <el-select v-model="linkInstanceId" :placeholder="$t('octop.selectInstance')" class="w-64" @change="loadLinks">
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
            <el-form :model="connForm" label-width="140px">
                <el-form-item :label="$t('octop.name')"><el-input v-model="connForm.name" /></el-form-item>
                <el-form-item :label="$t('octop.serverUrl')"><el-input v-model="connForm.serverUrl" /></el-form-item>
                <el-form-item :label="$t('octop.organization')"><el-input v-model="connForm.organization" /></el-form-item>
                <el-form-item :label="$t('octop.clientId')"><el-input v-model="connForm.clientId" /></el-form-item>
                <el-form-item :label="$t('octop.clientSecret')">
                    <el-input v-model="connForm.clientSecret" type="password" show-password :placeholder="$t('octop.secretConfigured')" />
                </el-form-item>
                <el-form-item :label="$t('octop.timeoutSec')"><el-input-number v-model="connForm.timeoutSec" :min="3" :max="120" /></el-form-item>
                <el-form-item :label="$t('octop.syncInterval')"><el-input-number v-model="connForm.syncIntervalSec" :min="60" /></el-form-item>
                <el-form-item :label="$t('octop.tlsVerify')"><el-switch v-model="tlsOn" /></el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="editConnVisible = false">{{ $t('common.cancel') }}</el-button>
                <el-button type="primary" @click="saveConn">{{ $t('common.save') }}</el-button>
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
                <el-button type="primary" @click="saveLink">{{ $t('common.save') }}</el-button>
            </template>
        </el-dialog>

        <el-dialog v-model="bindVisible" :title="$t('octop.bindInstance')" width="480px">
            <el-select v-model="bindInstanceId" class="w-full">
                <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
            <template #footer>
                <el-button @click="bindVisible = false">{{ $t('common.cancel') }}</el-button>
                <el-button type="primary" @click="doBind">{{ $t('common.save') }}</el-button>
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
import { OctopAuthStatusEnum, OctopSyncStatusEnum } from '../enums';
import type { CasdoorApp, CasdoorAuth, CasdoorConnection, IdentityLink, InstanceOverview } from '../types';
import { formatShanghai } from '../time';

const enableEnum = {
    Enable: EnumValue.of(1, 'common.enable').tagTypeSuccess(),
    Disable: EnumValue.of(-1, 'common.disable').tagTypeDanger(),
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

const connForm = reactive({
    id: 0,
    name: '',
    serverUrl: '',
    organization: '',
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
const linkForm = reactive({ casdoorUserSourceId: '', octopUserId: '', evidence: '' });

const appQuery = ref({ connectionId: 0, keyword: '', pageNum: 1, pageSize: 10 });
const userQuery = ref({ connectionId: 0, keyword: '', pageNum: 1, pageSize: 10 });
const appSearch = [SearchItem.input('keyword', 'octop.keyword')];
const userSearch = [SearchItem.input('keyword', 'octop.keyword')];

const appColumns = [
    TableColumn.new('sourceId', 'octop.sourceId'),
    TableColumn.new('name', 'octop.name'),
    TableColumn.new('displayName', 'octop.displayName'),
    TableColumn.new('enabled', 'octop.enabled').isSlot(),
    TableColumn.new('present', 'octop.present').isSlot(),
    TableColumn.new('action', 'common.operation')
        .setActions<CasdoorApp>([{ key: 'bind', label: 'octop.bindInstance', perm: 'octop:casdoor:save', onClick: (row) => openBind(row) }])
        .fixedRight(),
];
const userColumns = [
    TableColumn.new('sourceId', 'octop.casdoorUserId'),
    TableColumn.new('username', 'octop.user'),
    TableColumn.new('displayName', 'octop.displayName'),
    TableColumn.new('email', 'common.email'),
    TableColumn.new('enabled', 'octop.enabled').isSlot(),
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

const loadAuths = async () => {
    if (!conn.value || !authAppId.value) {
        auths.value = [];
        return;
    }
    auths.value = (await octopCasdoorApi.auths.request({ connectionId: conn.value.id, applicationSourceId: authAppId.value })) || [];
};

const onTest = async () => {
    if (!conn.value) return;
    await octopCasdoorApi.test.request({ ...conn.value, clientSecret: '' });
    Msg.success('common.success');
};
const onSync = async () => {
    if (!conn.value) return;
    await octopCasdoorApi.sync.request({ id: conn.value.id });
    Msg.success('common.success');
    await loadConn();
};
const saveConn = async () => {
    await octopCasdoorApi.save.request(connForm);
    editConnVisible.value = false;
    await loadConn();
};
const loadLinks = async () => {
    if (!linkInstanceId.value) return;
    links.value = (await octopCasdoorApi.links.request({ instanceId: linkInstanceId.value })) || [];
};
const saveLink = async () => {
    if (!conn.value || !linkInstanceId.value) return;
    await octopCasdoorApi.saveLink.request({
        connectionId: conn.value.id,
        octopInstanceId: linkInstanceId.value,
        ...linkForm,
    });
    linkVisible.value = false;
    await loadLinks();
};
const onDeleteLink = async (id: number) => {
    if (!(await useI18nDeleteConfirm(''))) return;
    await octopCasdoorApi.deleteLink.request({ id });
    await loadLinks();
};
const openBind = (row: CasdoorApp) => {
    bindAppId.value = row.id;
    bindVisible.value = true;
};
const doBind = async () => {
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
        await loadLinks();
    }
});
</script>
