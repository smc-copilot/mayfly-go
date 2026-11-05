<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-3">
            <h2 class="text-lg font-medium">{{ $t('octop.events') }}</h2>
            <div class="flex gap-2">
                <el-button v-auth="'octop:collect:recollect'" @click="onRecollect">{{ $t('octop.recollect') }}</el-button>
                <el-button v-auth="'octop:event:export'" type="primary" @click="onExport">{{ $t('octop.export') }}</el-button>
            </div>
        </div>
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.timezoneHint')" />

        <el-alert v-for="(g, idx) in gaps" :key="idx" class="mb-2" :type="gapType(g.coverage)" :closable="false" :title="gapTitle(g)" />

        <data-table ref="tableRef" :page-api="octopEventApi.query" :search-items="searchItems" v-model:query-form="query" :columns="columns" :data-handler-fn="onPage">
            <template #category="{ data }">
                <enum-tag :enums="OctopEventCategoryEnum" :value="data.category" />
            </template>
            <template #identityStatus="{ data }">
                {{ data.identityStatus === 'linked' ? $t('octop.identityLinked') : $t('octop.identityUnlinked') }}
            </template>
            <template #sourceTime="{ data }">
                {{ formatShanghai(data.sourceTime) }}
            </template>
            <template #collectedAt="{ data }">
                {{ formatShanghai(data.collectedAt) }}
            </template>
            <template #diagnostic="{ data }">
                {{ data.uncollected === 1 ? $t('octop.uncollected') : data.diagnostic }}
            </template>
        </data-table>

        <el-tabs class="mt-4">
            <el-tab-pane :label="$t('octop.summaryUser')">
                <el-table :data="userSummary" size="small">
                    <el-table-column prop="groupKey" :label="$t('octop.octopUserId')" />
                    <el-table-column prop="displayName" :label="$t('octop.user')" />
                    <el-table-column prop="total" :label="$t('octop.total')" />
                    <el-table-column prop="failCount" :label="$t('octop.failCount')" />
                    <el-table-column prop="inputTokens" :label="$t('octop.tokens')" />
                    <el-table-column :label="$t('octop.lastActivity')">
                        <template #default="{ row }">{{ formatShanghai(row.lastActivity) }}</template>
                    </el-table-column>
                </el-table>
            </el-tab-pane>
            <el-tab-pane :label="$t('octop.summaryAgent')">
                <el-table :data="agentSummary" size="small">
                    <el-table-column prop="groupKey" :label="$t('octop.expert')" />
                    <el-table-column prop="displayName" :label="$t('octop.displayName')" />
                    <el-table-column prop="total" :label="$t('octop.total')" />
                    <el-table-column prop="failCount" :label="$t('octop.failCount')" />
                    <el-table-column :label="$t('octop.lastActivity')">
                        <template #default="{ row }">{{ formatShanghai(row.lastActivity) }}</template>
                    </el-table-column>
                </el-table>
            </el-tab-pane>
        </el-tabs>
    </div>
</template>

<script lang="ts" setup>
import { onMounted, ref, watch } from 'vue';
import { TableColumn } from '@/components/data-table';
import DataTable from '@/components/data-table/DataTable.vue';
import { SearchItem } from '@/components/data-table/SearchForm';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import config from '@/common/config';
import { joinClientParams } from '@/common/request';
import { downloadFile } from '@/common/utils/file';
import { useI18nConfirm } from '@/hooks/useI18n';
import { octopEventApi, octopInstanceApi } from '../api';
import { OctopEventCategoryEnum } from '../enums';
import type { EventGap, EventPage, EventSummary, InstanceOverview } from '../types';
import { formatShanghai } from '../time';

const tableRef = ref();
const instances = ref<InstanceOverview[]>([]);
const gaps = ref<EventGap[]>([]);
const userSummary = ref<EventSummary[]>([]);
const agentSummary = ref<EventSummary[]>([]);
const query = ref({
    instanceId: undefined as number | undefined,
    category: '',
    action: '',
    status: '',
    octopUserId: '',
    agentId: '',
    keyword: '',
    startTime: undefined as number | undefined,
    endTime: undefined as number | undefined,
    pageNum: 1,
    pageSize: 10,
});

const searchItems = ref([SearchItem.input('keyword', 'octop.keyword')]);

const columns = [
    TableColumn.new('sourceInstanceId', 'octop.selectInstance'),
    TableColumn.new('category', 'octop.category').isSlot(),
    TableColumn.new('action', 'octop.action'),
    TableColumn.new('status', 'octop.status'),
    TableColumn.new('octopUsername', 'octop.user'),
    TableColumn.new('identityStatus', 'octop.identityLink').isSlot(),
    TableColumn.new('agentName', 'octop.expert'),
    TableColumn.new('sourceRecordId', 'octop.sourceId'),
    TableColumn.new('sourceTime', 'octop.sourceTime').isSlot(),
    TableColumn.new('collectedAt', 'octop.collectedAt').isSlot(),
    TableColumn.new('source', 'octop.sourceSystem'),
    TableColumn.new('diagnostic', 'octop.uncollected').isSlot(),
];

const onPage = (page: EventPage) => {
    gaps.value = page.gaps || [];
    return page;
};

const gapType = (coverage: string) => {
    if (coverage === 'collect_fail') return 'error';
    if (coverage === 'source_purged') return 'warning';
    if (coverage === 'unsupported') return 'info';
    return 'success';
};
const gapTitle = (g: EventGap) => `${g.sourceTable}: ${g.reason}`;

const loadSummary = async () => {
    userSummary.value = (await octopEventApi.summary.request({ ...query.value, groupBy: 'user' })) || [];
    agentSummary.value = (await octopEventApi.summary.request({ ...query.value, groupBy: 'agent' })) || [];
};

const onExport = () => {
    const params = new URLSearchParams();
    if (query.value.instanceId) params.set('instanceId', String(query.value.instanceId));
    if (query.value.category) params.set('category', query.value.category);
    if (query.value.action) params.set('action', query.value.action);
    if (query.value.status) params.set('status', query.value.status);
    if (query.value.octopUserId) params.set('octopUserId', query.value.octopUserId);
    if (query.value.agentId) params.set('agentId', query.value.agentId);
    if (query.value.keyword) params.set('keyword', query.value.keyword);
    if (query.value.startTime) params.set('startTime', String(query.value.startTime));
    if (query.value.endTime) params.set('endTime', String(query.value.endTime));
    downloadFile(`${config.baseApiUrl}/octop/events/export?${params.toString()}&${joinClientParams()}`);
};

const onRecollect = async () => {
    if (!query.value.instanceId) return;
    if (!(await useI18nConfirm('octop.recollectConfirm'))) return;
    await octopEventApi.recollect.request({ instanceId: query.value.instanceId, sourceTable: '' });
    tableRef.value?.search();
    await loadSummary();
};

watch(
    () => [query.value.instanceId, query.value.category],
    () => loadSummary()
);

onMounted(async () => {
    instances.value = (await octopInstanceApi.overview.request()) || [];
    searchItems.value = [
        SearchItem.select('instanceId', 'octop.selectInstance').withOptions(instances.value.map((i) => ({ value: i.id, label: i.name }))),
        SearchItem.select('category', 'octop.category').withOptions(Object.values(OctopEventCategoryEnum).map((e) => ({ value: e.value, label: e.label }))),
        SearchItem.input('octopUserId', 'octop.octopUserId'),
        SearchItem.input('agentId', 'octop.expert'),
        SearchItem.input('action', 'octop.action'),
        SearchItem.input('status', 'octop.status'),
        SearchItem.input('keyword', 'octop.keyword'),
        SearchItem.datePicker('timeRange', 'octop.timeRange')
            .withSpan(2)
            .withOneProps('type', 'datetimerange')
            .bindEvent('change', (value: Date[] | null) => {
                if (!value || value.length < 2) {
                    query.value.startTime = undefined;
                    query.value.endTime = undefined;
                    return;
                }
                query.value.startTime = new Date(value[0]).getTime();
                query.value.endTime = new Date(value[1]).getTime();
            }),
    ];
    await loadSummary();
});
</script>
