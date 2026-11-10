<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
                <el-button link type="primary" icon="ArrowLeft" @click="goBack">{{ $t('octop.backToUsers') }}</el-button>
                <h2 class="text-lg font-medium">{{ $t('octop.userDetail') }}</h2>
            </div>
            <div class="flex gap-2 items-center">
                <instance-selector :model-value="instanceId" v-model:valid="instanceValid" @change="onInstanceChange" />
                <el-button v-if="hasUser && hasPerm('octop:library:base')" type="primary" plain @click="goLibrary">{{ $t('octop.viewUserLibrary') }}</el-button>
                <el-button link type="primary" icon="refresh" @click="reload">{{ $t('common.refresh') }}</el-button>
            </div>
        </div>

        <el-alert v-if="instanceId && instanceValid === false" type="error" :closable="false" class="mb-3" :title="$t('octop.instanceInvalid')" />
        <el-empty v-else-if="!instanceId" :description="$t('octop.selectInstanceFirst')" />
        <el-empty v-else-if="!userId" :description="$t('octop.selectUserFirst')" />
        <template v-else-if="instanceValid && userId">
            <read-status-bar :meta="detail?.source" :show-refresh="true" @refresh="reload" />

            <el-alert v-if="userMissing" type="warning" :closable="false" class="mb-3" :title="$t('octop.userNotFound')" />

            <div v-else-if="user" class="mb-4 rounded border border-gray-200 dark:border-gray-700 p-4">
                <div class="flex flex-wrap items-center gap-3 mb-3">
                    <span class="text-lg font-medium">{{ user.displayName || user.username || user.userId }}</span>
                    <enum-tag :enums="OctopUserStatusEnum" :value="user.enabled ?? 0" />
                    <span class="text-sm text-gray-500">{{ user.username }} · {{ user.userId }}</span>
                </div>
                <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                    <div>
                        <div class="text-gray-500">{{ $t('octop.expertCount') }}</div>
                        <stat-value-text :stat="detail?.expertCount" />
                    </div>
                    <div>
                        <div class="text-gray-500">{{ $t('octop.threadCount') }}</div>
                        <stat-value-text :stat="detail?.threadCount" />
                    </div>
                    <div v-if="hasPerm('octop:library:base')">
                        <div class="text-gray-500">{{ $t('octop.templateCount') }}</div>
                        <stat-value-text :stat="detail?.templateCount" />
                    </div>
                    <div v-if="hasPerm('octop:library:base')">
                        <div class="text-gray-500">{{ $t('octop.packageCount') }}</div>
                        <stat-value-text :stat="detail?.skillPackageCount" />
                    </div>
                </div>
            </div>

            <el-tabs v-if="!userMissing" :model-value="tab" @tab-change="onTabChange">
                <el-tab-pane v-if="hasPerm('octop:expert:read')" :label="$t('octop.expertsTab')" name="experts">
                    <data-table
                        :key="`experts-${contextGen}`"
                        ref="expertTableRef"
                        :page-api="octopBusinessApi.experts"
                        :search-items="expertSearchItems"
                        v-model:query-form="expertQuery"
                        :columns="expertColumns"
                        :before-query-fn="expertHooks.beforeQueryFn"
                        :data-handler-fn="expertHooks.dataHandlerFn"
                    >
                        <template #name="{ data }">
                            <div class="font-medium">{{ data.name || data.expertId }}</div>
                            <div class="text-xs text-gray-500">{{ data.expertId }}</div>
                        </template>
                        <template #enabled="{ data }">
                            <enum-tag :enums="OctopUserStatusEnum" :value="data.enabled ?? 0" />
                        </template>
                        <template #lastState="{ data }">
                            {{ data.lastState || $t('octop.notProvided') }}
                        </template>
                        <template #skillPkgCount="{ data }">
                            {{ data.skillPkgCount == null ? '-' : data.skillPkgCount }}
                        </template>
                        <template #createdAt="{ data }">{{ formatShanghai(data.createdAt) }}</template>
                        <template #updatedAt="{ data }">{{ formatShanghai(data.updatedAt) }}</template>
                    </data-table>
                </el-tab-pane>
                <el-tab-pane v-if="hasPerm('octop:session:base')" :label="$t('octop.sessionsTab')" name="sessions">
                    <data-table
                        :key="`sessions-${contextGen}`"
                        ref="threadTableRef"
                        :page-api="octopBusinessApi.threads"
                        :search-items="threadSearchItems"
                        v-model:query-form="threadQuery"
                        :columns="threadColumns"
                        :before-query-fn="threadHooks.beforeQueryFn"
                        :data-handler-fn="threadHooks.dataHandlerFn"
                    >
                        <template #title="{ data }">
                            {{ data.title || $t('octop.unnamedThread') }}
                        </template>
                        <template #agent="{ data }">
                            <div>{{ data.agentName || '-' }}</div>
                            <div class="text-xs text-gray-500">{{ data.agentId || '' }}</div>
                        </template>
                        <template #hasMessages="{ data }">
                            {{ data.hasMessages ? $t('octop.hasMessages') : $t('octop.noMessages') }}
                        </template>
                        <template #createdAt="{ data }">{{ formatShanghai(data.createdAt) }}</template>
                        <template #lastActive="{ data }">
                            {{ data.lastActive ? formatShanghai(data.lastActive) : $t('octop.noThreadActivity') }}
                        </template>
                    </data-table>
                </el-tab-pane>
            </el-tabs>
        </template>

        <expert-config-drawer v-model:visible="expertDrawerVisible" :instance-id="instanceId" :user-id="userId" :expert-id="activeExpertId" />
    </div>
</template>

<script lang="ts" setup>
import { computed, ref, useTemplateRef, watch } from 'vue';
import { useRouter } from 'vue-router';
import { TableColumn } from '@/components/data-table';
import DataTable from '@/components/data-table/DataTable.vue';
import { SearchItem } from '@/components/data-table/SearchForm';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { hasPerm } from '@/components/auth/auth';
import { octopBusinessApi } from '../api';
import { OctopUserStatusEnum } from '../enums';
import { formatShanghai } from '../time';
import type { BizExpert, BizPage, BizThread, BizUser, BizUserDetail } from '../types';
import InstanceSelector from '../components/InstanceSelector.vue';
import ReadStatusBar from '../components/ReadStatusBar.vue';
import StatValueText from '../components/StatValueText.vue';
import { useOctopPageContext } from '../composables/useOctopPageContext';
import ExpertConfigDrawer from './ExpertConfigDrawer.vue';

defineOptions({ name: 'OctopInstanceUserDetail' });

const TABS = ['experts', 'sessions'] as const;
const router = useRouter();
const ctx = useOctopPageContext({ tabs: TABS, defaultTab: 'experts' });
const { instanceId, userId, tab, contextGen, hasUser } = ctx;

const instanceValid = ref<boolean | undefined>();
const detail = ref<BizUserDetail | null>(null);
const expertDrawerVisible = ref(false);
const activeExpertId = ref<string | undefined>();
const expertTableRef = useTemplateRef<InstanceType<typeof DataTable>>('expertTableRef');
const threadTableRef = useTemplateRef<InstanceType<typeof DataTable>>('threadTableRef');

const user = computed<BizUser | null | undefined>(() => detail.value?.user);
const userMissing = computed(() => !!detail.value && !detail.value.user && detail.value.source?.state === 'ready');

const expertQuery = ref({ keyword: '', model: '', enabled: undefined as number | undefined, pageNum: 1, pageSize: 10 });
const threadQuery = ref({ keyword: '', channel: '', agentId: '', pageNum: 1, pageSize: 10 });

const expertSearchItems = [
    SearchItem.input('keyword', 'octop.keyword'),
    SearchItem.input('model', 'octop.model'),
    SearchItem.select('enabled', 'octop.userStatus').withOptions(Object.values(OctopUserStatusEnum).map((e) => ({ value: e.value, label: e.label }))),
];
const threadSearchItems = [
    SearchItem.input('keyword', 'octop.keyword'),
    SearchItem.input('agentId', 'octop.expertId'),
    SearchItem.input('channel', 'octop.channel'),
];

const expertColumns = [
    TableColumn.new('name', 'octop.expert').isSlot(),
    TableColumn.new('description', 'octop.description'),
    TableColumn.new('model', 'octop.model'),
    TableColumn.new('enabled', 'octop.userStatus').isSlot(),
    TableColumn.new('lastState', 'octop.lastRecordState').isSlot(),
    TableColumn.new('sourceTemplate', 'octop.sourceTemplate'),
    TableColumn.new('skillPkgCount', 'octop.mountedSkillPackages').isSlot(),
    TableColumn.new('createdAt', 'octop.createdAt').isSlot(),
    TableColumn.new('updatedAt', 'octop.updatedAt').isSlot(),
    TableColumn.new('action', 'common.operation')
        .setActions<BizExpert>([{ key: 'config', label: 'octop.expertConfig', type: 'primary', onClick: (row) => openExpert(row) }])
        .fixedRight(),
];

const threadColumns = [
    TableColumn.new('title', 'octop.threadTitle').isSlot(),
    TableColumn.new('threadId', 'octop.threadId'),
    TableColumn.new('agent', 'octop.expert').isSlot(),
    TableColumn.new('channel', 'octop.channel'),
    TableColumn.new('createdAt', 'octop.createdAt').isSlot(),
    TableColumn.new('lastActive', 'octop.lastThreadActive').isSlot(),
    TableColumn.new('hasMessages', 'octop.messageState').isSlot(),
];

const expertHooks = computed(() => ctx.bindTable<BizPage<BizExpert>>({ id: instanceId.value, userId: userId.value }));
const threadHooks = computed(() => ctx.bindTable<BizPage<BizThread>>({ id: instanceId.value, userId: userId.value }));

const loadDetail = async () => {
    detail.value = null;
    if (!instanceId.value || !userId.value) return;
    try {
        await ctx.loadInContext(
            () => octopBusinessApi.user.request({ id: instanceId.value, userId: userId.value }),
            (res) => {
                detail.value = res ?? null;
            }
        );
    } catch {
        detail.value = null;
    }
};

const onInstanceChange = (id: number | undefined) => ctx.setInstance(id);
const onTabChange = (name: string | number) => ctx.setTab(String(name));
const openExpert = (row: BizExpert) => {
    activeExpertId.value = row.expertId;
    expertDrawerVisible.value = true;
};
const goBack = () => {
    router.push({ name: 'OctopInstanceUsers', query: { instanceId: instanceId.value ? String(instanceId.value) : undefined } });
};
const goLibrary = () => {
    router.push({ name: 'OctopInstanceLibrary', query: { instanceId: String(instanceId.value), userId: userId.value } });
};
const reload = async () => {
    await loadDetail();
    if (tab.value === 'experts') expertTableRef.value?.search();
    else threadTableRef.value?.search();
};

watch(
    () => [instanceId.value, userId.value, instanceValid.value, contextGen.value],
    () => {
        if (instanceId.value && userId.value && instanceValid.value !== false) void loadDetail();
    },
    { immediate: true }
);
</script>
