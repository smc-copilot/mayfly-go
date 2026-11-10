<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-medium">{{ $t('octop.overview') }}</h2>
            <div class="flex gap-2">
                <el-button v-auth="'octop:config:save'" @click="settingsVisible = true">{{ $t('octop.settings') }}</el-button>
                <el-button v-auth="'octop:instance:save'" type="primary" icon="plus" @click="onAdd">{{ $t('octop.registerInstance') }}</el-button>
                <el-button link type="primary" icon="refresh" @click="load">{{ $t('common.refresh') }}</el-button>
            </div>
        </div>

        <el-alert type="info" :closable="false" class="mb-4" :title="$t('octop.gapHint')" />

        <el-row :gutter="12" class="mb-4">
            <el-col v-for="card in cards" :key="card.status" :xs="12" :sm="8" :md="4">
                <div class="rounded border border-gray-200 dark:border-gray-700 p-3 mb-2">
                    <div class="text-2xl font-semibold">{{ card.count }}</div>
                    <div class="text-sm text-gray-500">{{ $t(card.label) }}</div>
                </div>
            </el-col>
        </el-row>

        <data-table ref="tableRef" :page-api="octopInstanceApi.list" :search-items="searchItems" v-model:query-form="query" :columns="columns">
            <template #healthStatus="{ data }">
                <enum-tag :enums="OctopHealthEnum" :value="data.healthStatus" />
                <div class="text-xs text-gray-500 mt-1">{{ data.healthReason || $t('octop.neverCollected') }}</div>
            </template>
            <template #lastCollectAt="{ data }">
                <span>{{ formatShanghai(data.lastCollectAt) }}</span>
            </template>
        </data-table>

        <InstanceEdit v-model:visible="editVisible" :data="editData" @success="onSaved" />

        <el-dialog v-model="settingsVisible" :title="$t('octop.settings')" width="480px" destroy-on-close>
            <el-form :model="settings" label-width="160px">
                <el-form-item :label="$t('octop.eventRetainDays')">
                    <el-input-number v-model="settings.eventRetainDays" :min="1" :max="3650" />
                </el-form-item>
                <el-form-item :label="$t('octop.collectConcurrency')">
                    <el-input-number v-model="settings.collectConcurrency" :min="1" :max="8" />
                </el-form-item>
                <el-form-item :label="$t('octop.collectTimeoutSec')">
                    <el-input-number v-model="settings.collectTimeoutSec" :min="10" :max="300" />
                </el-form-item>
                <el-alert type="warning" :closable="false" :title="$t('octop.sourceRetainHint')" />
            </el-form>
            <template #footer>
                <el-button @click="settingsVisible = false">{{ $t('common.cancel') }}</el-button>
                <el-button type="primary" @click="saveSettings">{{ $t('common.save') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref, useTemplateRef } from 'vue';
import { useRouter } from 'vue-router';
import { TableColumn } from '@/components/data-table';
import DataTable from '@/components/data-table/DataTable.vue';
import { SearchItem } from '@/components/data-table/SearchForm';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { Msg } from '@/hooks/useI18n';
import { octopInstanceApi } from '../api';
import { OctopHealthEnum } from '../enums';
import type { InstanceOverview, OctopSettings } from '../types';
import InstanceEdit from '../instance/InstanceEdit.vue';
import { formatShanghai } from '../time';

const router = useRouter();
const tableRef = useTemplateRef<InstanceType<typeof DataTable>>('tableRef');
const list = ref<InstanceOverview[]>([]);
const editVisible = ref(false);
const editData = ref<InstanceOverview | null>(null);
const settingsVisible = ref(false);
const settings = reactive<OctopSettings>({ eventRetainDays: 90, metricSampleSec: 120, collectConcurrency: 3, collectTimeoutSec: 45 });
const query = ref({ name: '', status: '', pageNum: 1, pageSize: 20 });

const searchItems = [
    SearchItem.input('keyword', 'octop.keyword'),
    SearchItem.select('status', 'octop.health').withOptions(Object.values(OctopHealthEnum).map((e) => ({ value: e.value, label: e.label }))),
];

const columns = [
    TableColumn.new('name', 'octop.name'),
    TableColumn.new('center', 'octop.center'),
    TableColumn.new('machineName', 'octop.machine'),
    TableColumn.new('healthStatus', 'octop.health').isSlot(),
    TableColumn.new('lastCollectAt', 'octop.lastCollect').isSlot(),
    TableColumn.new('casdoorAppName', 'octop.casdoorApp'),
    TableColumn.new('action', 'common.operation')
        .setActions<InstanceOverview>([
            { key: 'detail', label: 'octop.instanceDetail', type: 'primary', onClick: (row) => goDetail(row.id) },
            { key: 'users', label: 'octop.instanceUsers', type: 'primary', perm: 'octop:user:base', onClick: (row) => goUsers(row.id) },
            { key: 'library', label: 'octop.instanceLibrary', type: 'primary', perm: 'octop:library:base', onClick: (row) => goLibrary(row.id) },
            { key: 'edit', label: 'common.edit', type: 'primary', perm: 'octop:instance:save', onClick: (row) => onEdit(row) },
        ])
        .fixedRight(),
];

const cards = computed(() => {
    const count = (status: string) => list.value.filter((i) => i.healthStatus === status).length;
    return [
        { status: 'normal', label: 'octop.normal', count: count('normal') },
        { status: 'warning', label: 'octop.warning', count: count('warning') },
        { status: 'critical', label: 'octop.critical', count: count('critical') },
        { status: 'unknown', label: 'octop.unknown', count: count('unknown') },
        { status: 'maintenance', label: 'octop.maintenanceStatus', count: count('maintenance') },
    ];
});

const load = async () => {
    list.value = (await octopInstanceApi.overview.request()) || [];
    tableRef.value?.search();
};

const onAdd = () => {
    editData.value = null;
    editVisible.value = true;
};
const onEdit = (row: InstanceOverview) => {
    editData.value = row;
    editVisible.value = true;
};
const onSaved = () => {
    editVisible.value = false;
    load();
};
const goDetail = (id: number) => {
    router.push({ name: 'OctopInstanceDetail', query: { id: String(id) } });
};
const goUsers = (id: number) => {
    router.push({ name: 'OctopInstanceUsers', query: { instanceId: String(id) } });
};
const goLibrary = (id: number) => {
    router.push({ name: 'OctopInstanceLibrary', query: { instanceId: String(id) } });
};

const saveSettings = async () => {
    await octopInstanceApi.saveSettings.request(settings);
    Msg.success('common.success');
    settingsVisible.value = false;
};

onMounted(async () => {
    const s = await octopInstanceApi.settings.request();
    if (s) Object.assign(settings, s);
    await load();
});
</script>
