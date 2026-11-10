<template>
    <el-drawer v-model="visible" :append-to-body="true" :title="$t('octop.expertConfig')" size="560px" destroy-on-close>
        <div v-loading="loading">
            <el-alert v-if="source" class="mb-3" :closable="false" :type="alertType(source.state)" :title="source.message || source.state" />
            <template v-if="expert">
                <el-descriptions :column="1" border label-width="140">
                    <el-descriptions-item :label="$t('octop.expert')">{{ expert.name }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.expertId')">{{ expert.expertId }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.description')">{{ expert.description || expert.config?.description || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.model')">{{ expert.model || expert.config?.modelRef || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.provider')">
                        {{ expert.config?.providerName || '-' }}
                        <span v-if="expert.config?.providerId" class="text-xs text-gray-500 ml-1">{{ expert.config.providerId }}</span>
                    </el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.userStatus')">
                        <enum-tag :enums="OctopUserStatusEnum" :value="expert.enabled ?? expert.config?.enabled ?? 0" />
                    </el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.lastRecordState')">{{ expert.lastState || expert.config?.lastState || $t('octop.notProvided') }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.sourceTemplate')">{{ expert.sourceTemplate || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.createdAt')">{{ formatShanghai(expert.createdAt) }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.updatedAt')">{{ formatShanghai(expert.updatedAt) }}</el-descriptions-item>
                </el-descriptions>

                <h3 class="mt-4 mb-2 font-medium">{{ $t('octop.runParams') }}</h3>
                <el-table :data="paramRows" size="small" border>
                    <el-table-column prop="label" :label="$t('octop.paramName')" width="160" />
                    <el-table-column :label="$t('octop.paramValue')">
                        <template #default="{ row }">
                            <span v-if="row.unsupported" class="text-gray-400">{{ $t('octop.paramUnsupported') }}</span>
                            <span v-else-if="row.value == null || row.value === ''">{{ $t('octop.notProvided') }}</span>
                            <span v-else>{{ row.value }}</span>
                        </template>
                    </el-table-column>
                </el-table>

                <h3 class="mt-4 mb-2 font-medium">{{ $t('octop.mounts') }}</h3>
                <el-empty v-if="mountRows.length === 0" :description="$t('octop.noMounts')" :image-size="64" />
                <el-table v-else :data="mountRows" size="small" border>
                    <el-table-column prop="type" :label="$t('octop.mountType')" width="140" />
                    <el-table-column prop="name" :label="$t('octop.name')" />
                    <el-table-column prop="id" :label="$t('octop.sourceId')" />
                    <el-table-column :label="$t('octop.status')" width="100">
                        <template #default="{ row }">
                            <el-tag v-if="row.status === 'missing'" type="danger" effect="plain" :disable-transitions="true">{{ $t('octop.refMissing') }}</el-tag>
                            <el-tag v-else type="success" effect="plain" :disable-transitions="true">{{ $t('octop.refOk') }}</el-tag>
                        </template>
                    </el-table-column>
                </el-table>

                <div v-if="hasPerm('octop:content:read')" class="mt-4">
                    <el-button v-if="!promptLoaded" type="primary" plain :loading="promptLoading" @click="loadPrompt">{{ $t('octop.loadPrompt') }}</el-button>
                    <template v-else>
                        <h3 class="mb-2 font-medium">{{ $t('octop.promptPreview') }}</h3>
                        <el-alert v-if="promptUnsupported" type="info" :closable="false" :title="$t('octop.paramUnsupported')" />
                        <pre v-else class="text-xs whitespace-pre-wrap break-all rounded border border-gray-200 dark:border-gray-700 p-3 max-h-80 overflow-auto">{{ promptText || $t('octop.notProvided') }}</pre>
                    </template>
                </div>
            </template>
            <el-empty v-else-if="!loading" :description="$t('octop.businessUnavailable')" />
        </div>
    </el-drawer>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { hasPerm } from '@/components/auth/auth';
import { octopBusinessApi } from '../api';
import { OctopUserStatusEnum } from '../enums';
import { formatShanghai } from '../time';
import type { BizExpert, BizRefSummary, SourceMeta } from '../types';

const props = defineProps<{
    instanceId?: number;
    userId?: string;
    expertId?: string;
}>();

const visible = defineModel<boolean>('visible', { default: false });
const { t } = useI18n();

const loading = ref(false);
const expert = ref<BizExpert | null>(null);
const source = ref<SourceMeta | null>(null);
const promptLoaded = ref(false);
const promptLoading = ref(false);
const promptText = ref('');
const promptUnsupported = ref(false);

const alertType = (state?: string) => {
    if (state === 'failed' || state === 'forbidden') return 'error';
    if (state === 'partial' || state === 'not_verified') return 'warning';
    if (state === 'ready') return 'success';
    return 'info';
};

const paramRows = computed(() => {
    const cfg = expert.value?.config;
    const unsupported = new Set(cfg?.unsupported || []);
    const rows: Array<{ key: string; label: string; value: string | number | boolean | null | undefined; unsupported: boolean }> = [
        { key: 'temperature', label: t('octop.paramTemperature'), value: cfg?.temperature, unsupported: unsupported.has('temperature') },
        { key: 'topP', label: t('octop.paramTopP'), value: cfg?.topP, unsupported: unsupported.has('topP') },
        { key: 'maxTokens', label: t('octop.paramMaxTokens'), value: cfg?.maxTokens, unsupported: unsupported.has('maxTokens') },
        { key: 'maxIterations', label: t('octop.paramMaxIterations'), value: cfg?.maxIterations, unsupported: unsupported.has('maxIterations') },
        { key: 'inputLength', label: t('octop.paramInputLength'), value: cfg?.inputLength, unsupported: unsupported.has('inputLength') },
        { key: 'trajectoryOn', label: t('octop.paramTrajectory'), value: cfg?.trajectoryOn, unsupported: unsupported.has('trajectoryOn') },
    ];
    return rows;
});

const mountRows = computed(() => {
    const cfg = expert.value?.config;
    if (!cfg) return [];
    const map = (type: string, list?: BizRefSummary[]) => (list || []).map((r) => ({ type: t(type), id: r.id, name: r.name || r.id, status: r.status }));
    return [
        ...map('octop.mountSkillPackage', cfg.skillPackages),
        ...map('octop.mountKnowledge', cfg.knowledgeBases),
        ...map('octop.mountMcp', cfg.mcpConnections),
        ...map('octop.mountSubExpert', cfg.subExperts),
    ];
});

const load = async () => {
    expert.value = null;
    source.value = null;
    promptLoaded.value = false;
    promptText.value = '';
    promptUnsupported.value = false;
    if (!props.instanceId || !props.userId || !props.expertId) return;
    loading.value = true;
    try {
        const res = await octopBusinessApi.expert.request({
            id: props.instanceId,
            userId: props.userId,
            expertId: props.expertId,
        });
        expert.value = res?.item ?? null;
        source.value = res?.source ?? null;
    } finally {
        loading.value = false;
    }
};

const loadPrompt = async () => {
    if (!props.instanceId || !props.userId || !props.expertId) return;
    promptLoading.value = true;
    try {
        const res = await octopBusinessApi.expert.request({
            id: props.instanceId,
            userId: props.userId,
            expertId: props.expertId,
            includeContent: true,
        });
        if (res?.item) expert.value = res.item;
        source.value = res?.source ?? source.value;
        promptText.value = res?.item?.config?.promptPreview || '';
        promptUnsupported.value = !!res?.item?.config?.unsupported?.includes('prompt');
        promptLoaded.value = true;
    } finally {
        promptLoading.value = false;
    }
};

watch(
    () => [visible.value, props.instanceId, props.userId, props.expertId] as const,
    ([open]) => {
        if (open) void load();
    }
);
</script>
