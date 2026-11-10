<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-3">
            <h2 class="text-lg font-medium">{{ $t('octop.instanceLibrary') }}</h2>
            <div class="flex gap-2 items-center">
                <instance-selector :model-value="instanceId" v-model:valid="instanceValid" @change="onInstanceChange" />
                <user-selector :instance-id="instanceId" :model-value="userId" @change="onUserChange" />
                <el-button link type="primary" icon="refresh" @click="reload">{{ $t('common.refresh') }}</el-button>
            </div>
        </div>

        <el-alert v-if="instanceId && instanceValid === false" type="error" :closable="false" class="mb-3" :title="$t('octop.instanceInvalid')" />
        <el-empty v-else-if="!instanceId" :description="$t('octop.selectInstanceFirst')" />
        <el-empty v-else-if="!userId" :description="$t('octop.selectUserFirst')" />
        <template v-else-if="instanceValid && userId">
            <div class="mb-3 text-sm text-gray-600 dark:text-gray-400">
                {{ $t('octop.currentUser') }}:
                <span class="font-medium">{{ currentUserLabel }}</span>
            </div>
            <read-status-bar :sources="statusSources" />

            <el-tabs v-model="activeTab">
                <el-tab-pane :label="templateTabLabel" name="templates">
                    <data-table
                        :key="`tpl-${contextGen}`"
                        ref="templateTableRef"
                        :page-api="octopBusinessApi.expertTemplates"
                        :search-items="searchItems"
                        v-model:query-form="templateQuery"
                        :columns="templateColumns"
                        :before-query-fn="templateHooks.beforeQueryFn"
                        :data-handler-fn="templateHooks.dataHandlerFn"
                    >
                        <template #name="{ data }">
                            <div class="flex items-center gap-2">
                                <el-avatar v-if="data.avatar" :src="data.avatar" :size="28" />
                                <div>
                                    <div class="font-medium">{{ data.name }}</div>
                                    <div class="text-xs text-gray-500">{{ data.templateId }}{{ data.slug ? ` · ${data.slug}` : '' }}</div>
                                </div>
                            </div>
                        </template>
                        <template #sourceExpert="{ data }">
                            <span v-if="data.sourceAvailable === false" class="text-gray-400">{{ $t('octop.sourceExpertUnavailable') }}</span>
                            <span v-else>{{ data.sourceExpertId || '-' }}</span>
                        </template>
                        <template #createdAt="{ data }">{{ formatShanghai(data.createdAt) }}</template>
                        <template #updatedAt="{ data }">{{ formatShanghai(data.updatedAt) }}</template>
                    </data-table>
                </el-tab-pane>

                <el-tab-pane :label="packageTabLabel" name="packages">
                    <div class="flex flex-col md:flex-row gap-4">
                        <div class="md:w-2/5 min-w-0">
                            <data-table
                                :key="`pkg-${contextGen}`"
                                ref="packageTableRef"
                                :page-api="octopBusinessApi.skillPackages"
                                :search-items="searchItems"
                                v-model:query-form="packageQuery"
                                :columns="packageColumns"
                                :before-query-fn="packageHooks.beforeQueryFn"
                                :data-handler-fn="packageHooks.dataHandlerFn"
                            >
                                <template #name="{ data }">
                                    <div class="font-medium">{{ data.name }}</div>
                                    <div class="text-xs text-gray-500">{{ data.packageId }}</div>
                                </template>
                                <template #skillCount="{ data }">
                                    <stat-value-text :stat="data.skillCount" />
                                </template>
                                <template #shared="{ data }">
                                    {{ data.shared ? $t('octop.shared') : $t('octop.notShared') }}
                                </template>
                                <template #updatedAt="{ data }">{{ formatShanghai(data.updatedAt) }}</template>
                            </data-table>
                        </div>
                        <div class="md:w-3/5 min-w-0">
                            <el-empty v-if="!selectedPackageId" :description="$t('octop.selectPackageFirst')" />
                            <template v-else>
                                <div class="mb-2 font-medium">{{ $t('octop.skillsInPackage') }}</div>
                                <data-table
                                    :key="`skills-${contextGen}-${selectedPackageId}`"
                                    ref="skillTableRef"
                                    :page-api="octopBusinessApi.skills"
                                    :search-items="skillSearchItems"
                                    v-model:query-form="skillQuery"
                                    :columns="skillColumns"
                                    :before-query-fn="skillHooks.beforeQueryFn"
                                    :data-handler-fn="skillHooks.dataHandlerFn"
                                >
                                    <template #name="{ data }">
                                        <div class="font-medium">{{ data.name || data.slug }}</div>
                                        <div class="text-xs text-gray-500">{{ data.slug }}</div>
                                    </template>
                                </data-table>
                            </template>
                        </div>
                    </div>
                </el-tab-pane>
            </el-tabs>
        </template>

        <expert-template-detail-drawer
            v-model:visible="templateDrawerVisible"
            :instance-id="instanceId"
            :user-id="userId"
            :template-id="activeTemplateId"
        />
        <skill-package-detail-drawer
            v-model:visible="packageDrawerVisible"
            :instance-id="instanceId"
            :user-id="userId"
            :package-id="activePackageId"
            :skill-slug="activeSkillSlug"
        />
    </div>
</template>

<script lang="ts" setup>
import { computed, ref, useTemplateRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { TableColumn } from '@/components/data-table';
import DataTable from '@/components/data-table/DataTable.vue';
import { SearchItem } from '@/components/data-table/SearchForm';
import { octopBusinessApi } from '../api';
import { formatShanghai } from '../time';
import type { BizExpertTemplate, BizPage, BizSkill, BizSkillPackage, BizUserOption, SourceMeta } from '../types';
import InstanceSelector from '../components/InstanceSelector.vue';
import UserSelector from '../components/UserSelector.vue';
import ReadStatusBar from '../components/ReadStatusBar.vue';
import StatValueText from '../components/StatValueText.vue';
import { useOctopPageContext } from '../composables/useOctopPageContext';
import ExpertTemplateDetailDrawer from './ExpertTemplateDetailDrawer.vue';
import SkillPackageDetailDrawer from './SkillPackageDetailDrawer.vue';

defineOptions({ name: 'OctopInstanceLibrary' });

const { t } = useI18n();
const ctx = useOctopPageContext();
const { instanceId, userId, contextGen } = ctx;

const instanceValid = ref<boolean | undefined>();
const activeTab = ref<'templates' | 'packages'>('templates');
const selectedUser = ref<BizUserOption | undefined>();
const templateTotal = ref<number | null>(null);
const packageTotal = ref<number | null>(null);
const templateSource = ref<SourceMeta | null>(null);
const packageSource = ref<SourceMeta | null>(null);
const selectedPackageId = ref<string | undefined>();

const templateDrawerVisible = ref(false);
const packageDrawerVisible = ref(false);
const activeTemplateId = ref<string | undefined>();
const activePackageId = ref<string | undefined>();
const activeSkillSlug = ref<string | undefined>();

const templateTableRef = useTemplateRef<InstanceType<typeof DataTable>>('templateTableRef');
const packageTableRef = useTemplateRef<InstanceType<typeof DataTable>>('packageTableRef');
const skillTableRef = useTemplateRef<InstanceType<typeof DataTable>>('skillTableRef');

const templateQuery = ref({ keyword: '', pageNum: 1, pageSize: 10 });
const packageQuery = ref({ keyword: '', pageNum: 1, pageSize: 10 });
const skillQuery = ref({ keyword: '', pageNum: 1, pageSize: 10 });

const searchItems = [SearchItem.input('keyword', 'octop.keyword')];
const skillSearchItems = [SearchItem.input('keyword', 'octop.keyword')];

const currentUserLabel = computed(() => {
    const u = selectedUser.value;
    if (u && (u.displayName || u.username)) return [u.displayName, u.username, u.userId].filter(Boolean).join(' · ');
    return userId.value || '';
});

const templateTabLabel = computed(() => {
    const n = templateTotal.value;
    return n == null ? t('octop.expertTemplates') : `${t('octop.expertTemplates')} (${n})`;
});
const packageTabLabel = computed(() => {
    const n = packageTotal.value;
    return n == null ? t('octop.skillPackages') : `${t('octop.skillPackages')} (${n})`;
});

const statusSources = computed<SourceMeta[]>(() => {
    const items: SourceMeta[] = [];
    if (templateSource.value) items.push(templateSource.value);
    if (packageSource.value) items.push(packageSource.value);
    return items;
});

const templateColumns = [
    TableColumn.new('name', 'octop.name').isSlot(),
    TableColumn.new('description', 'octop.description'),
    TableColumn.new('createdBy', 'octop.createdBy'),
    TableColumn.new('sourceExpert', 'octop.sourceExpert').isSlot(),
    TableColumn.new('createdAt', 'octop.createdAt').isSlot(),
    TableColumn.new('updatedAt', 'octop.updatedAt').isSlot(),
    TableColumn.new('action', 'common.operation')
        .setActions<BizExpertTemplate>([{ key: 'detail', label: 'common.detail', type: 'primary', onClick: (row) => openTemplate(row) }])
        .fixedRight(),
];

const packageColumns = [
    TableColumn.new('name', 'octop.name').isSlot(),
    TableColumn.new('description', 'octop.description'),
    TableColumn.new('skillCount', 'octop.skillCount').isSlot(),
    TableColumn.new('shared', 'octop.visibility').isSlot(),
    TableColumn.new('updatedAt', 'octop.updatedAt').isSlot(),
    TableColumn.new('action', 'common.operation')
        .setActions<BizSkillPackage>([
            {
                key: 'select',
                label: 'octop.viewSkills',
                type: 'primary',
                onClick: (row) => {
                    selectedPackageId.value = row.packageId;
                },
            },
            { key: 'detail', label: 'common.detail', type: 'primary', onClick: (row) => openPackage(row) },
        ])
        .fixedRight(),
];

const skillColumns = [
    TableColumn.new('name', 'octop.skill').isSlot(),
    TableColumn.new('description', 'octop.description'),
    TableColumn.new('source', 'octop.sourceSystem'),
    TableColumn.new('action', 'common.operation')
        .setActions<BizSkill>([{ key: 'detail', label: 'common.detail', type: 'primary', onClick: (row) => openSkill(row) }])
        .fixedRight(),
];

const templateHooks = computed(() =>
    ctx.bindTable<BizPage<BizExpertTemplate>>({ id: instanceId.value, userId: userId.value }, (res) => {
        templateTotal.value = res?.source && !['ready', 'partial'].includes(res.source.state) ? null : (res?.total ?? null);
        templateSource.value = res?.source ?? null;
    })
);
const packageHooks = computed(() =>
    ctx.bindTable<BizPage<BizSkillPackage>>({ id: instanceId.value, userId: userId.value }, (res) => {
        packageTotal.value = res?.source && !['ready', 'partial'].includes(res.source.state) ? null : (res?.total ?? null);
        packageSource.value = res?.source ?? null;
        if (selectedPackageId.value && !(res?.list || []).some((p) => p.packageId === selectedPackageId.value)) {
            selectedPackageId.value = undefined;
        }
    })
);
const skillHooks = computed(() =>
    ctx.bindTable<BizPage<BizSkill>>({ id: instanceId.value, userId: userId.value, packageId: selectedPackageId.value })
);

const onInstanceChange = (id: number | undefined) => {
    selectedPackageId.value = undefined;
    selectedUser.value = undefined;
    templateTotal.value = null;
    packageTotal.value = null;
    ctx.setInstance(id);
};
const onUserChange = (user: BizUserOption | undefined) => {
    selectedPackageId.value = undefined;
    selectedUser.value = user;
    ctx.setUser(user?.userId);
};

const openTemplate = (row: BizExpertTemplate) => {
    activeTemplateId.value = row.templateId;
    templateDrawerVisible.value = true;
};
const openPackage = (row: BizSkillPackage) => {
    activePackageId.value = row.packageId;
    activeSkillSlug.value = undefined;
    packageDrawerVisible.value = true;
};
const openSkill = (row: BizSkill) => {
    activePackageId.value = selectedPackageId.value;
    activeSkillSlug.value = row.slug;
    packageDrawerVisible.value = true;
};

const reload = () => {
    if (!userId.value) return;
    templateTableRef.value?.search();
    packageTableRef.value?.search();
    if (selectedPackageId.value) skillTableRef.value?.search();
};

watch(
    () => contextGen.value,
    () => {
        selectedPackageId.value = undefined;
        templateTotal.value = null;
        packageTotal.value = null;
        templateSource.value = null;
        packageSource.value = null;
    }
);
</script>
