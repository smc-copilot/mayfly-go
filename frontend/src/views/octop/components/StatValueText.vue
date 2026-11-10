<template>
    <span>
        <el-tooltip v-if="stat && stat.value === null" placement="top" :content="tip">
            <el-tag :type="tagType" effect="plain" :disable-transitions="true">{{ $t('octop.statUnavailable') }}</el-tag>
        </el-tooltip>
        <el-tooltip v-else-if="stat && tip" placement="top" :content="tip">
            <span>{{ stat.value }}</span>
        </el-tooltip>
        <span v-else-if="stat">{{ stat.value }}</span>
        <span v-else>-</span>
    </span>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { OctopSourceStateEnum } from '../enums';
import { formatShanghai } from '../time';
import type { StatValue } from '../types';

/**
 * 统计值展示：value 为 null 时显示「不可用」+ 状态提示，绝不当 0 展示。
 */
const props = defineProps<{ stat?: StatValue | null }>();
const { t } = useI18n();

const stateLabel = computed(() => {
    const s = props.stat?.state;
    if (!s) return '';
    const hit = Object.values(OctopSourceStateEnum).find((e) => e.value === s);
    return hit ? t(hit.label) : s;
});

const tip = computed(() => {
    const s = props.stat;
    if (!s) return '';
    const parts: string[] = [];
    if (stateLabel.value && s.state !== 'ready') parts.push(stateLabel.value);
    if (s.definition) parts.push(s.definition);
    if (s.asOf) parts.push(`${t('octop.readAt')}: ${formatShanghai(s.asOf)}`);
    return parts.join(' · ');
});

const tagType = computed(() => {
    const s = props.stat?.state;
    if (s === 'failed' || s === 'forbidden') return 'danger';
    if (s === 'partial' || s === 'not_verified') return 'warning';
    return 'info';
});
</script>
