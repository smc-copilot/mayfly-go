<template>
    <div class="mb-3 rounded border border-gray-200 dark:border-gray-700 px-3 py-2 text-sm">
        <div v-if="items.length === 0" class="text-gray-500">{{ $t('octop.readStatusNone') }}</div>
        <div v-else class="flex flex-wrap items-center gap-x-4 gap-y-1">
            <div v-for="(s, idx) in items" :key="`${s.source}-${idx}`" class="flex items-center gap-2">
                <span class="font-medium">{{ sourceLabel(s.source) }}</span>
                <enum-tag :enums="OctopSourceStateEnum" :value="s.state" />
                <el-tag v-if="s.complete === false" type="warning" effect="plain" :disable-transitions="true">{{ $t('octop.readIncomplete') }}</el-tag>
                <span class="text-gray-500">{{ $t('octop.readAt') }}: {{ formatShanghai(s.readAt) }}</span>
                <span v-if="s.message" class="text-gray-500 truncate max-w-80" :title="s.message">{{ s.message }}</span>
            </div>
            <el-button v-if="showRefresh" link type="primary" icon="refresh" class="ml-auto" @click="$emit('refresh')">{{ $t('common.refresh') }}</el-button>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { OctopSourceStateEnum } from '../enums';
import { formatShanghai } from '../time';
import type { SourceMeta } from '../types';

const props = withDefaults(
    defineProps<{
        /** 多个来源（PostgreSQL / Octop API / Langfuse 各自状态与读取时间） */
        sources?: SourceMeta[];
        /** 单个来源的快捷写法 */
        meta?: SourceMeta | null;
        showRefresh?: boolean;
    }>(),
    { sources: () => [], meta: null, showRefresh: false }
);

defineEmits<{ refresh: [] }>();

const { t, te } = useI18n();

const SOURCE_LABEL_KEYS: Record<string, string> = {
    postgres: 'octop.sourceNamePostgres',
    octop_api: 'octop.sourceNameOctopApi',
    langfuse: 'octop.sourceNameLangfuse',
};

const items = computed<SourceMeta[]>(() => {
    if (props.sources.length > 0) return props.sources;
    return props.meta ? [props.meta] : [];
});

const sourceLabel = (source: string) => {
    const key = SOURCE_LABEL_KEYS[source];
    return key && te(key) ? t(key) : source;
};
</script>
