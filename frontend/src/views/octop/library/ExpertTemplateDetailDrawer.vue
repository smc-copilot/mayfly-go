<template>
    <el-drawer v-model="visible" :append-to-body="true" :title="$t('octop.templateDetail')" size="640px" destroy-on-close>
        <div v-loading="loading">
            <el-alert v-if="source" class="mb-3" :closable="false" :type="alertType(source.state)" :title="source.message || source.state" />
            <template v-if="template">
                <el-descriptions :column="1" border label-width="140">
                    <el-descriptions-item :label="$t('octop.name')">{{ template.name }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.templateId')">{{ template.templateId }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.slug')">{{ template.slug || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.description')">{{ template.description || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.createdBy')">{{ template.createdBy }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.sourceExpert')">
                        <span v-if="template.sourceAvailable === false">{{ $t('octop.sourceExpertUnavailable') }}</span>
                        <span v-else>{{ template.sourceExpertId || '-' }}</span>
                    </el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.createdAt')">{{ formatShanghai(template.createdAt) }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.updatedAt')">{{ formatShanghai(template.updatedAt) }}</el-descriptions-item>
                </el-descriptions>

                <h3 class="mt-4 mb-2 font-medium">{{ $t('octop.files') }}</h3>
                <el-empty v-if="!template.files?.length" :description="$t('octop.noFiles')" :image-size="64" />
                <el-table v-else :data="template.files" size="small" border>
                    <el-table-column prop="name" :label="$t('octop.name')" />
                    <el-table-column prop="path" :label="$t('octop.filePath')" />
                    <el-table-column prop="size" :label="$t('octop.fileSize')" width="100" />
                    <el-table-column :label="$t('common.operation')" width="120">
                        <template #default="{ row }">
                            <el-button
                                v-if="hasPerm('octop:content:read')"
                                link
                                type="primary"
                                :loading="previewingId === row.fileId"
                                @click="previewFile(row.fileId)"
                            >
                                {{ $t('octop.previewFile') }}
                            </el-button>
                            <span v-else class="text-gray-400">{{ $t('octop.contentDenied') }}</span>
                        </template>
                    </el-table-column>
                </el-table>

                <div v-if="fileContent" class="mt-4">
                    <div class="mb-2 flex items-center justify-between">
                        <span class="font-medium">{{ fileContent.name || $t('octop.previewFile') }}</span>
                        <el-tag v-if="fileContent.truncated" type="warning" effect="plain" :disable-transitions="true">{{ $t('octop.contentTruncated') }}</el-tag>
                    </div>
                    <pre class="text-xs whitespace-pre-wrap break-all rounded border border-gray-200 dark:border-gray-700 p-3 max-h-96 overflow-auto">{{ fileContent.content }}</pre>
                </div>
            </template>
            <el-empty v-else-if="!loading" :description="$t('octop.businessUnavailable')" />
        </div>
    </el-drawer>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue';
import { hasPerm } from '@/components/auth/auth';
import { octopBusinessApi } from '../api';
import { formatShanghai } from '../time';
import type { BizExpertTemplate, BizFileContent, SourceMeta } from '../types';

const props = defineProps<{
    instanceId?: number;
    userId?: string;
    templateId?: string;
}>();

const visible = defineModel<boolean>('visible', { default: false });

const loading = ref(false);
const template = ref<BizExpertTemplate | null>(null);
const source = ref<SourceMeta | null>(null);
const fileContent = ref<BizFileContent | null>(null);
const previewingId = ref<string | undefined>();

const alertType = (state?: string) => {
    if (state === 'failed' || state === 'forbidden') return 'error';
    if (state === 'partial' || state === 'not_verified') return 'warning';
    if (state === 'ready') return 'success';
    return 'info';
};

const load = async () => {
    template.value = null;
    source.value = null;
    fileContent.value = null;
    if (!props.instanceId || !props.userId || !props.templateId) return;
    loading.value = true;
    try {
        const res = await octopBusinessApi.expertTemplate.request({
            id: props.instanceId,
            userId: props.userId,
            templateId: props.templateId,
        });
        template.value = res?.item ?? null;
        source.value = res?.source ?? null;
    } finally {
        loading.value = false;
    }
};

const previewFile = async (fileId: string) => {
    if (!props.instanceId || !props.userId || !props.templateId) return;
    previewingId.value = fileId;
    try {
        fileContent.value = await octopBusinessApi.expertTemplateFile.request({
            id: props.instanceId,
            userId: props.userId,
            templateId: props.templateId,
            fileId,
        });
    } finally {
        previewingId.value = undefined;
    }
};

watch(
    () => [visible.value, props.instanceId, props.userId, props.templateId] as const,
    ([open]) => {
        if (open) void load();
    }
);
</script>
