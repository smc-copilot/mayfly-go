<template>
    <el-drawer v-model="visible" :append-to-body="true" :title="drawerTitle" size="640px" destroy-on-close>
        <div v-loading="loading">
            <el-alert v-if="source" class="mb-3" :closable="false" :type="alertType(source.state)" :title="source.message || source.state" />
            <template v-if="pkg">
                <el-descriptions :column="1" border label-width="140">
                    <el-descriptions-item :label="$t('octop.name')">{{ pkg.name }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.packageId')">{{ pkg.packageId }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.description')">{{ pkg.description || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.createdBy')">{{ pkg.createdBy }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.skillCount')">
                        <stat-value-text :stat="pkg.skillCount" />
                    </el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.visibility')">
                        {{ pkg.shared ? $t('octop.shared') : $t('octop.notShared') }}
                        <span v-if="pkg.visibility" class="text-gray-500 ml-1">· {{ pkg.visibility }}</span>
                    </el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.copyPolicy')">{{ pkg.copyPolicy || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.downloadPolicy')">{{ pkg.downloadPolicy || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.updatedAt')">{{ formatShanghai(pkg.updatedAt) }}</el-descriptions-item>
                </el-descriptions>
            </template>

            <template v-if="skill">
                <h3 class="mt-4 mb-2 font-medium">{{ $t('octop.skillDetail') }}</h3>
                <el-descriptions :column="1" border label-width="140">
                    <el-descriptions-item :label="$t('octop.skill')">{{ skill.name || skill.slug }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.slug')">{{ skill.slug }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.description')">{{ skill.description || '-' }}</el-descriptions-item>
                    <el-descriptions-item :label="$t('octop.sourceSystem')">{{ skill.source || '-' }}</el-descriptions-item>
                </el-descriptions>

                <h3 class="mt-4 mb-2 font-medium">{{ $t('octop.files') }}</h3>
                <el-empty v-if="!skill.files?.length" :description="$t('octop.noFiles')" :image-size="64" />
                <el-table v-else :data="skill.files" size="small" border>
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

            <el-empty v-else-if="!loading && !pkg" :description="$t('octop.businessUnavailable')" />
        </div>
    </el-drawer>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { hasPerm } from '@/components/auth/auth';
import { octopBusinessApi } from '../api';
import { formatShanghai } from '../time';
import type { BizFileContent, BizSkill, BizSkillPackage, SourceMeta } from '../types';
import StatValueText from '../components/StatValueText.vue';

const props = defineProps<{
    instanceId?: number;
    userId?: string;
    packageId?: string;
    skillSlug?: string;
}>();

const visible = defineModel<boolean>('visible', { default: false });
const { t } = useI18n();

const loading = ref(false);
const pkg = ref<BizSkillPackage | null>(null);
const skill = ref<BizSkill | null>(null);
const source = ref<SourceMeta | null>(null);
const fileContent = ref<BizFileContent | null>(null);
const previewingId = ref<string | undefined>();

const drawerTitle = computed(() => (props.skillSlug ? t('octop.skillDetail') : t('octop.packageDetail')));

const alertType = (state?: string) => {
    if (state === 'failed' || state === 'forbidden') return 'error';
    if (state === 'partial' || state === 'not_verified') return 'warning';
    if (state === 'ready') return 'success';
    return 'info';
};

const load = async () => {
    pkg.value = null;
    skill.value = null;
    source.value = null;
    fileContent.value = null;
    if (!props.instanceId || !props.userId || !props.packageId) return;
    loading.value = true;
    try {
        const pkgRes = await octopBusinessApi.skillPackage.request({
            id: props.instanceId,
            userId: props.userId,
            packageId: props.packageId,
        });
        pkg.value = pkgRes?.item ?? null;
        source.value = pkgRes?.source ?? null;

        if (props.skillSlug) {
            const skillRes = await octopBusinessApi.skill.request({
                id: props.instanceId,
                userId: props.userId,
                packageId: props.packageId,
                slug: props.skillSlug,
            });
            skill.value = skillRes?.item ?? null;
            if (skillRes?.source) source.value = skillRes.source;
        }
    } finally {
        loading.value = false;
    }
};

const previewFile = async (fileId: string) => {
    if (!props.instanceId || !props.userId || !props.packageId || !props.skillSlug) return;
    previewingId.value = fileId;
    try {
        fileContent.value = await octopBusinessApi.skillFile.request({
            id: props.instanceId,
            userId: props.userId,
            packageId: props.packageId,
            slug: props.skillSlug,
            fileId,
        });
    } finally {
        previewingId.value = undefined;
    }
};

watch(
    () => [visible.value, props.instanceId, props.userId, props.packageId, props.skillSlug] as const,
    ([open]) => {
        if (open) void load();
    }
);
</script>
