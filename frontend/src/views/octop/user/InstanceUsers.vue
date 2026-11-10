<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-3">
            <h2 class="text-lg font-medium">{{ $t('octop.instanceUsers') }}</h2>
            <div class="flex gap-2 items-center">
                <instance-selector :model-value="instanceId" v-model:valid="instanceValid" @change="onInstanceChange" />
                <el-button link type="primary" icon="refresh" @click="reload">{{ $t('common.refresh') }}</el-button>
            </div>
        </div>

        <el-alert v-if="instanceId && instanceValid === false" type="error" :closable="false" class="mb-3" :title="$t('octop.instanceInvalid')" />
        <el-empty v-else-if="!instanceId" :description="$t('octop.selectInstanceFirst')" />
        <template v-else-if="instanceValid">
            <read-status-bar :sources="statusSources" />
            <el-alert v-if="usersBlocked" type="warning" :closable="false" class="mb-3" :title="blockedTitle" />
            <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.userCountHint')" />

            <data-table
                v-if="!usersBlocked"
                :key="contextGen"
                ref="tableRef"
                :page-api="octopBusinessApi.users"
                :search-items="searchItems"
                v-model:query-form="query"
                :columns="columns"
                :before-query-fn="hooks.beforeQueryFn"
                :data-handler-fn="hooks.dataHandlerFn"
            >
                <template #user="{ data }">
                    <div class="font-medium">{{ data.displayName || data.username || data.userId }}</div>
                    <div v-if="data.displayName && data.username" class="text-xs text-gray-500">{{ data.username }}</div>
                </template>
                <template #enabled="{ data }">
                    <enum-tag :enums="OctopUserStatusEnum" :value="data.enabled ?? 0" />
                </template>
                <template #roleSummary="{ data }">
                    {{ data.roleSummary || '-' }}
                </template>
                <template #expertCount="{ data }">
                    <el-link v-if="statReady(data.expertCount)" type="primary" @click="goDetail(data, 'experts')">
                        <stat-value-text :stat="data.expertCount" />
                    </el-link>
                    <stat-value-text v-else :stat="data.expertCount" />
                </template>
                <template #threadCount="{ data }">
                    <el-link v-if="statReady(data.threadCount)" type="primary" @click="goDetail(data, 'sessions')">
                        <stat-value-text :stat="data.threadCount" />
                    </el-link>
                    <stat-value-text v-else :stat="data.threadCount" />
                </template>
                <template #lastActive="{ data }">
                    {{ data.lastActive ? formatShanghai(data.lastActive) : $t('octop.noThreadActivity') }}
                </template>
                <template #createdAt="{ data }">
                    {{ formatShanghai(data.createdAt) }}
                </template>
            </data-table>
        </template>
    </div>
</template>

<script lang="ts" setup>
import { computed, ref, useTemplateRef, watch } from 'vue';
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { TableColumn } from '@/components/data-table';
import DataTable from '@/components/data-table/DataTable.vue';
import { SearchItem } from '@/components/data-table/SearchForm';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { octopBusinessApi } from '../api';
import { OctopUserStatusEnum } from '../enums';
import { formatShanghai } from '../time';
import type { BizPage, BizUser, Capabilities, SourceMeta, StatValue } from '../types';
import InstanceSelector from '../components/InstanceSelector.vue';
import ReadStatusBar from '../components/ReadStatusBar.vue';
import StatValueText from '../components/StatValueText.vue';
import { useOctopPageContext } from '../composables/useOctopPageContext';

defineOptions({ name: 'OctopInstanceUsers' });

const { t } = useI18n();
const router = useRouter();
const ctx = useOctopPageContext();
const { instanceId, contextGen } = ctx;

const tableRef = useTemplateRef<InstanceType<typeof DataTable>>('tableRef');
const instanceValid = ref<boolean | undefined>();
const capabilities = ref<Capabilities | null>(null);
const tableSource = ref<SourceMeta | null>(null);

const query = ref({ keyword: '', enabled: undefined as number | undefined, pageNum: 1, pageSize: 10 });

const searchItems = [
    SearchItem.input('keyword', 'octop.userKeyword'),
    SearchItem.select('enabled', 'octop.userStatus').withOptions(Object.values(OctopUserStatusEnum).map((e) => ({ value: e.value, label: e.label }))),
];

const columns = [
    TableColumn.new('user', 'octop.user').isSlot(),
    TableColumn.new('userId', 'octop.octopUserId'),
    TableColumn.new('enabled', 'octop.userStatus').isSlot(),
    TableColumn.new('roleSummary', 'octop.userRoles').isSlot(),
    TableColumn.new('expertCount', 'octop.expertCount').isSlot(),
    TableColumn.new('threadCount', 'octop.threadCount').isSlot(),
    TableColumn.new('lastActive', 'octop.lastThreadActive').isSlot(),
    TableColumn.new('createdAt', 'octop.createdAt').isSlot(),
    TableColumn.new('action', 'common.operation')
        .setActions<BizUser>([
            { key: 'detail', label: 'octop.userDetail', type: 'primary', perm: 'octop:user:base', onClick: (row) => goDetail(row) },
            { key: 'library', label: 'octop.instanceLibrary', type: 'primary', perm: 'octop:library:base', onClick: (row) => goLibrary(row) },
        ])
        .fixedRight(),
];

const hooks = computed(() =>
    ctx.bindTable<BizPage<BizUser>>({ id: instanceId.value }, (res) => {
        tableSource.value = res?.source ?? null;
    })
);

const dbState = computed(() => capabilities.value?.businessDb?.state);
const usersBlocked = computed(() => !!dbState.value && !['ready', 'partial'].includes(dbState.value));
const blockedTitle = computed(() => capabilities.value?.businessDb?.message || t('octop.businessUnavailable'));

const statusSources = computed<SourceMeta[]>(() => {
    if (tableSource.value) return [tableSource.value];
    const caps = capabilities.value;
    if (!caps) return [];
    const items: SourceMeta[] = [];
    if (caps.businessDb) {
        items.push({ source: 'postgres', state: caps.businessDb.state, message: caps.businessDb.message, readAt: caps.businessDb.verifiedAt });
    }
    if (caps.octopPreview) {
        items.push({ source: 'octop_api', state: caps.octopPreview.state, message: caps.octopPreview.message, readAt: caps.octopPreview.verifiedAt });
    }
    if (caps.langfuse) {
        items.push({ source: 'langfuse', state: caps.langfuse.state, message: caps.langfuse.message, readAt: caps.langfuse.verifiedAt });
    }
    return items;
});

const loadCapabilities = async () => {
    capabilities.value = null;
    tableSource.value = null;
    const id = instanceId.value;
    if (!id) return;
    try {
        await ctx.loadInContext(
            () => octopBusinessApi.capabilities.request({ id }),
            (res) => {
                capabilities.value = res ?? null;
            }
        );
    } catch {
        // 能力读取失败不阻止列表
    }
};

const statReady = (s?: StatValue | null) => !!s && s.value !== null;

const onInstanceChange = (id: number | undefined) => {
    ctx.setInstance(id);
};

const reload = async () => {
    await loadCapabilities();
    tableRef.value?.search();
};

const goDetail = (row: BizUser, tab: 'experts' | 'sessions' = 'experts') => {
    router.push({ name: 'OctopInstanceUserDetail', query: { instanceId: String(instanceId.value), userId: row.userId, tab } });
};
const goLibrary = (row: BizUser) => {
    router.push({ name: 'OctopInstanceLibrary', query: { instanceId: String(instanceId.value), userId: row.userId } });
};

watch(
    () => [instanceId.value, instanceValid.value],
    () => {
        if (instanceId.value && instanceValid.value) void loadCapabilities();
    },
    { immediate: true }
);
</script>
