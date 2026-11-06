<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-medium">{{ $t('octop.instanceDetail') }}</h2>
            <div class="flex gap-2 items-center">
                <el-select v-model="instanceId" filterable :placeholder="$t('octop.selectInstance')" class="w-64" @change="load">
                    <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
                <el-button v-if="detail?.canRestart" v-auth="'octop:instance:restart'" type="danger" @click="onRestart">{{ $t('octop.restart') }}</el-button>
                <el-button link type="primary" icon="refresh" @click="load">{{ $t('common.refresh') }}</el-button>
            </div>
        </div>

        <el-alert v-if="detail && !detail.canRestart" type="info" :closable="false" class="mb-3" :title="detail.restartHint || $t('octop.restartDeniedCli')" />
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.noShell')" />

        <div v-if="overview" class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div class="rounded border border-gray-200 dark:border-gray-700 p-4">
                <div class="font-medium mb-2">{{ overview.name }}</div>
                <enum-tag :enums="OctopHealthEnum" :value="overview.healthStatus" />
                <div class="text-sm text-gray-500 mt-2">{{ overview.healthReason }} · {{ formatShanghai(overview.lastCollectAt) }}</div>
                <div class="mt-3 text-sm space-y-1">
                    <div>{{ $t('octop.machine') }}: {{ overview.machineName }} {{ overview.machineIp }}</div>
                    <div>{{ $t('octop.casdoorApp') }}: {{ overview.casdoorAppName || '-' }}</div>
                    <div>{{ $t('octop.callbackUrl') }}: {{ overview.callbackUrl || '-' }}</div>
                    <div>{{ $t('octop.apiToken') }}: {{ overview.apiTokenConfigured === 1 ? $t('octop.secretConfigured') : $t('octop.secretEmpty') }}</div>
                    <div>{{ $t('octop.startMode') }}: {{ overview.startMode }} {{ overview.systemdUnit }}</div>
                </div>
            </div>
            <div class="rounded border border-gray-200 dark:border-gray-700 p-4">
                <div class="font-medium mb-2">{{ $t('octop.health') }}</div>
                <div v-for="p in probes" :key="p.name" class="flex items-center justify-between py-1 text-sm">
                    <span>{{ probeLabel(p.name) }}</span>
                    <span>
                        <enum-tag :enums="OctopProbeEnum" :value="p.status" />
                        <span class="ml-2 text-gray-500">{{ p.reason }}</span>
                    </span>
                </div>
            </div>
        </div>

        <div class="mb-3 flex gap-2">
            <el-radio-group v-model="range" @change="loadMetrics">
                <el-radio-button value="24h">{{ $t('octop.trend24h') }}</el-radio-button>
                <el-radio-button value="7d">{{ $t('octop.trend7d') }}</el-radio-button>
            </el-radio-group>
            <span class="text-xs text-gray-500 self-center">{{ $t('octop.gapHint') }}</span>
        </div>
        <ECharts height="280" :option="chartOption" />

        <div v-if="detail?.checkpoints?.length" class="mt-4 text-sm">
            <div class="font-medium mb-2">{{ $t('octop.events') }}</div>
            <el-table :data="detail.checkpoints" size="small">
                <el-table-column prop="sourceTable" :label="$t('octop.sourceSystem')" />
                <el-table-column prop="coverage" :label="$t('octop.status')" />
                <el-table-column prop="lastRecordId" :label="$t('octop.sourceId')" />
                <el-table-column :label="$t('octop.lastCollect')">
                    <template #default="{ row }">{{ formatShanghai(row.lastSuccessAt) }}</template>
                </el-table-column>
                <el-table-column prop="lastError" :label="$t('octop.healthReason')" />
            </el-table>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import ECharts from '@/components/echarts/ECharts.vue';
import type { ECOption } from '@/components/echarts/config';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { useI18nConfirm } from '@/hooks/useI18n';
import { octopInstanceApi } from '../api';
import { OctopHealthEnum, OctopProbeEnum } from '../enums';
import type { InstanceDetail, InstanceOverview, MetricSample, ProbeItem } from '../types';
import { formatShanghai } from '../time';

const { t } = useI18n();
const route = useRoute();
const instanceId = ref<number | undefined>();
const instances = ref<InstanceOverview[]>([]);
const detail = ref<InstanceDetail | null>(null);
const range = ref<'24h' | '7d'>('24h');
const samples = ref<MetricSample[]>([]);

const overview = computed(() => detail.value?.overview);
const probes = computed<ProbeItem[]>(() => detail.value?.probes || []);

const probeLabel = (name: string) => {
    if (name === 'host') return t('octop.host');
    if (name === 'octop') return t('octop.octopProcess');
    if (name === 'http') return t('octop.httpHealth');
    if (name === 'pg') return t('octop.pg');
    return name;
};

const alignedSamples = computed(() => {
    const src = samples.value;
    if (src.length === 0) return src;
    const interval = (detail.value?.thresholds?.sampleIntervalSec || 120) * 1000;
    const gapLimit = interval * 1.5;
    const out: Array<MetricSample | { collectedAt: string; hostCpu: null; hostMem: null; hostDisk: null }> = [];
    for (let i = 0; i < src.length; i++) {
        if (i > 0) {
            const prev = new Date(src[i - 1].collectedAt).getTime();
            const cur = new Date(src[i].collectedAt).getTime();
            if (cur - prev > gapLimit) {
                out.push({ collectedAt: new Date(prev + interval).toISOString(), hostCpu: null, hostMem: null, hostDisk: null });
            }
        }
        out.push(src[i]);
    }
    return out;
});

const chartOption = computed<ECOption>(() => {
    const times = alignedSamples.value.map((s) => formatShanghai(s.collectedAt));
    const seriesOf = (key: 'hostCpu' | 'hostMem' | 'hostDisk', name: string) => ({
        name,
        type: 'line' as const,
        connectNulls: false,
        data: alignedSamples.value.map((s) => {
            const v = s[key];
            return typeof v === 'number' ? v : null;
        }),
    });
    return {
        tooltip: { trigger: 'axis' },
        legend: { data: [t('octop.host') + ' CPU', t('octop.host') + ' MEM', t('octop.host') + ' DISK'] },
        xAxis: { type: 'category', data: times },
        yAxis: { type: 'value', min: 0, max: 100 },
        series: [
            seriesOf('hostCpu', t('octop.host') + ' CPU'),
            seriesOf('hostMem', t('octop.host') + ' MEM'),
            seriesOf('hostDisk', t('octop.host') + ' DISK'),
        ],
    };
});

const load = async () => {
    instances.value = (await octopInstanceApi.overview.request()) || [];
    if (!instanceId.value && instances.value.length) {
        instanceId.value = Number(route.query.id) || instances.value[0].id;
    }
    if (!instanceId.value) return;
    detail.value = await octopInstanceApi.detail.request({ id: instanceId.value });
    await loadMetrics();
};

const loadMetrics = async () => {
    if (!instanceId.value) return;
    const end = Date.now();
    const start = range.value === '7d' ? end - 7 * 24 * 3600 * 1000 : end - 24 * 3600 * 1000;
    samples.value = (await octopInstanceApi.metrics.request({ id: instanceId.value, start, end })) || [];
};

const onRestart = async () => {
    if (!instanceId.value) return;
    if (!(await useI18nConfirm('octop.restartConfirm'))) return;
    await octopInstanceApi.restart.request({ id: instanceId.value });
    await load();
};

onMounted(load);
</script>
