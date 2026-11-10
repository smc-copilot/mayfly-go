<template>
    <el-drawer
        v-model="visibleProxy"
        :title="$t('octop.sessionTrace')"
        size="90%"
        destroy-on-close
        append-to-body
        @opened="reload"
    >
        <div v-if="!instanceId || !userId || !threadId" class="p-4">
            <el-empty :description="$t('octop.selectThreadFirst')" />
        </div>
        <div v-else class="h-full flex flex-col gap-3 px-1">
            <div class="flex flex-wrap items-center gap-2 text-sm">
                <span>{{ $t('octop.threadId') }}: {{ threadId }}</span>
                <el-tag size="small" :type="assocTagType">{{ assocLabel }}</el-tag>
                <span v-if="meta?.projectId" class="text-gray-500">{{ $t('octop.langfuseProjectId') }}: {{ meta.projectId }}</span>
                <el-link v-if="meta?.externalUrl" type="primary" :href="meta.externalUrl" target="_blank" rel="noopener noreferrer">
                    {{ $t('octop.openInLangfuse') }}
                </el-link>
                <el-button link type="primary" icon="refresh" @click="reload">{{ $t('common.refresh') }}</el-button>
            </div>
            <read-status-bar :meta="tracePage?.source" />
            <el-alert v-if="meta?.message" :title="meta.message" type="info" :closable="false" />
            <el-alert
                v-if="tracePage?.association === 'association_unverified'"
                type="warning"
                :closable="false"
                :title="$t('octop.associationUnverified')"
            />
            <el-alert
                v-if="tracePage?.association === 'not_found_in_range'"
                type="info"
                :closable="false"
                :title="$t('octop.traceNotFoundInRange')"
            />

            <div class="flex-1 min-h-0 grid grid-cols-1 lg:grid-cols-3 gap-3">
                <div class="border rounded p-2 overflow-auto">
                    <div class="font-medium mb-2">{{ $t('octop.traceList') }}</div>
                    <el-empty v-if="!traces.length && !loadingTraces" :description="$t('octop.noTraces')" />
                    <el-menu v-else :default-active="activeTraceId" @select="onSelectTrace">
                        <el-menu-item v-for="tr in traces" :key="tr.traceId" :index="tr.traceId">
                            <div class="flex flex-col leading-5">
                                <span>{{ tr.name || tr.traceId }}</span>
                                <span class="text-xs text-gray-500">
                                    {{ formatShanghai(tr.startTime) }}
                                    <template v-if="tr.latencyMs != null"> · {{ tr.latencyMs }}ms</template>
                                    <template v-if="tr.hasError"> · {{ $t('octop.hasError') }}</template>
                                </span>
                            </div>
                        </el-menu-item>
                    </el-menu>
                    <el-button v-if="tracePage?.hasMore" class="mt-2 w-full" :loading="loadingTraces" @click="loadMoreTraces">
                        {{ $t('octop.loadMore') }}
                    </el-button>
                </div>

                <div class="border rounded p-2 overflow-auto">
                    <div class="font-medium mb-2">{{ $t('octop.observationTree') }}</div>
                    <el-empty v-if="!activeTraceId" :description="$t('octop.selectTraceFirst')" />
                    <el-empty v-else-if="!observations.length && !loadingObs" :description="$t('octop.noObservations')" />
                    <el-tree
                        v-else
                        :data="obsTree"
                        node-key="observationId"
                        :props="{ label: 'label', children: 'children' }"
                        highlight-current
                        default-expand-all
                        @node-click="onSelectObs"
                    />
                </div>

                <div class="border rounded p-2 overflow-auto">
                    <div class="font-medium mb-2">{{ $t('octop.observationDetail') }}</div>
                    <el-empty v-if="!activeObs" :description="$t('octop.selectObservationFirst')" />
                    <template v-else>
                        <el-descriptions :column="1" size="small" border>
                            <el-descriptions-item :label="$t('octop.observationId')">{{ activeObs.observationId }}</el-descriptions-item>
                            <el-descriptions-item :label="$t('octop.obsType')">{{ activeObs.type }}</el-descriptions-item>
                            <el-descriptions-item :label="$t('octop.name')">{{ activeObs.name || '-' }}</el-descriptions-item>
                            <el-descriptions-item :label="$t('octop.model')">{{ activeObs.model || '-' }}</el-descriptions-item>
                            <el-descriptions-item :label="$t('octop.latency')">
                                {{ activeObs.latencyMs == null ? $t('octop.unknownMetric') : `${activeObs.latencyMs}ms` }}
                            </el-descriptions-item>
                            <el-descriptions-item :label="$t('octop.tokens')">
                                {{ tokenText(activeObs) }}
                            </el-descriptions-item>
                            <el-descriptions-item v-if="activeObs.orphan" :label="$t('octop.orphanNode')">
                                {{ $t('octop.parentMissing') }}
                            </el-descriptions-item>
                        </el-descriptions>
                        <div class="mt-3" v-auth="'octop:content:read'">
                            <el-button type="primary" plain :loading="loadingContent" @click="loadContent">
                                {{ $t('octop.viewContent') }}
                            </el-button>
                            <el-alert v-if="content?.truncated" class="mt-2" type="warning" :closable="false" :title="$t('octop.contentTruncated')" />
                            <div v-if="content" class="mt-2 space-y-2">
                                <div>
                                    <div class="text-xs text-gray-500 mb-1">{{ $t('octop.input') }}</div>
                                    <pre class="text-xs whitespace-pre-wrap break-all bg-gray-50 dark:bg-gray-900 p-2 rounded max-h-48 overflow-auto">{{ content.input || '-' }}</pre>
                                </div>
                                <div>
                                    <div class="text-xs text-gray-500 mb-1">{{ $t('octop.output') }}</div>
                                    <pre class="text-xs whitespace-pre-wrap break-all bg-gray-50 dark:bg-gray-900 p-2 rounded max-h-48 overflow-auto">{{ content.output || '-' }}</pre>
                                </div>
                            </div>
                        </div>
                        <el-alert v-if="!hasPerm('octop:content:read')" class="mt-3" type="info" :closable="false" :title="$t('octop.contentDenied')" />
                    </template>
                </div>
            </div>
        </div>
    </el-drawer>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { hasPerm } from '@/components/auth/auth';
import { octopBusinessApi } from '../api';
import { formatShanghai } from '../time';
import type { BizObservation, BizObservationContent, BizTrace, ThreadTraceMeta, TracePage } from '../types';
import ReadStatusBar from '../components/ReadStatusBar.vue';

const props = defineProps<{
    visible: boolean;
    instanceId?: number;
    userId?: string;
    threadId?: string;
}>();
const emit = defineEmits<{ 'update:visible': [boolean] }>();
const { t } = useI18n();

const visibleProxy = computed({
    get: () => props.visible,
    set: (v) => emit('update:visible', v),
});

const meta = ref<ThreadTraceMeta | null>(null);
const tracePage = ref<TracePage<BizTrace> | null>(null);
const traces = ref<BizTrace[]>([]);
const obsPage = ref<TracePage<BizObservation> | null>(null);
const observations = ref<BizObservation[]>([]);
const activeTraceId = ref('');
const activeObs = ref<BizObservation | null>(null);
const content = ref<BizObservationContent | null>(null);
const loadingTraces = ref(false);
const loadingObs = ref(false);
const loadingContent = ref(false);
const tracePageNum = ref(1);

const assocLabel = computed(() => {
    const a = meta.value?.association || tracePage.value?.association || '';
    const map: Record<string, string> = {
        ready: t('octop.stateReady'),
        not_configured: t('octop.stateNotConfigured'),
        not_verified: t('octop.stateNotVerified'),
        association_unverified: t('octop.associationUnverified'),
        not_found_in_range: t('octop.traceNotFoundInRange'),
        forbidden: t('octop.stateForbidden'),
        failed: t('octop.stateFailed'),
        unsupported: t('octop.stateUnsupported'),
    };
    return map[a] || a || '-';
});
const assocTagType = computed(() => {
    const a = meta.value?.association || tracePage.value?.association;
    if (a === 'ready') return 'success';
    if (a === 'association_unverified' || a === 'failed' || a === 'forbidden') return 'danger';
    return 'warning';
});

type ObsNode = { observationId: string; label: string; children?: ObsNode[]; raw: BizObservation };

const obsTree = computed<ObsNode[]>(() => {
    const byId = new Map<string, ObsNode>();
    const roots: ObsNode[] = [];
    for (const o of observations.value) {
        byId.set(o.observationId, {
            observationId: o.observationId,
            label: `${o.type}${o.name ? ' · ' + o.name : ''}${o.orphan ? ' (' + t('octop.orphanNode') + ')' : ''}`,
            children: [],
            raw: o,
        });
    }
    for (const o of observations.value) {
        const node = byId.get(o.observationId)!;
        if (o.parentId && byId.has(o.parentId) && !o.orphan) {
            byId.get(o.parentId)!.children!.push(node);
        } else {
            roots.push(node);
        }
    }
    return roots;
});

const tokenText = (o: BizObservation) => {
    if (o.inputTokens == null && o.outputTokens == null) return t('octop.unknownMetric');
    return `${o.inputTokens ?? '?'} / ${o.outputTokens ?? '?'}`;
};

const reload = async () => {
    meta.value = null;
    tracePage.value = null;
    traces.value = [];
    observations.value = [];
    activeTraceId.value = '';
    activeObs.value = null;
    content.value = null;
    tracePageNum.value = 1;
    if (!props.instanceId || !props.userId || !props.threadId) return;
    try {
        meta.value = await octopBusinessApi.threadTraceStatus.request({
            id: props.instanceId,
            userId: props.userId,
            threadId: props.threadId,
        });
    } catch {
        meta.value = null;
    }
    await loadTraces(false);
};

const loadTraces = async (more: boolean) => {
    if (!props.instanceId || !props.userId || !props.threadId) return;
    loadingTraces.value = true;
    try {
        const pageNum = more ? tracePageNum.value + 1 : 1;
        const res = await octopBusinessApi.traces.request({
            id: props.instanceId,
            userId: props.userId,
            threadId: props.threadId,
            pageNum,
            pageSize: 20,
        });
        tracePage.value = res ?? null;
        const list = res?.list || [];
        traces.value = more ? [...traces.value, ...list] : list;
        tracePageNum.value = pageNum;
        if (!activeTraceId.value && traces.value.length) {
            await onSelectTrace(traces.value[0].traceId);
        }
    } finally {
        loadingTraces.value = false;
    }
};

const loadMoreTraces = () => loadTraces(true);

const onSelectTrace = async (traceId: string) => {
    activeTraceId.value = traceId;
    activeObs.value = null;
    content.value = null;
    observations.value = [];
    if (!props.instanceId || !props.userId || !props.threadId) return;
    loadingObs.value = true;
    try {
        const res = await octopBusinessApi.observations.request({
            id: props.instanceId,
            userId: props.userId,
            threadId: props.threadId,
            traceId,
            pageNum: 1,
            pageSize: 100,
        });
        obsPage.value = res ?? null;
        observations.value = res?.list || [];
    } finally {
        loadingObs.value = false;
    }
};

const onSelectObs = (node: ObsNode) => {
    activeObs.value = node.raw;
    content.value = null;
};

const loadContent = async () => {
    if (!activeObs.value || !props.instanceId || !props.userId || !props.threadId || !activeTraceId.value) return;
    loadingContent.value = true;
    try {
        content.value = await octopBusinessApi.observationContent.request({
            id: props.instanceId,
            userId: props.userId,
            threadId: props.threadId,
            traceId: activeTraceId.value,
            observationId: activeObs.value.observationId,
        });
    } finally {
        loadingContent.value = false;
    }
};

watch(
    () => [props.visible, props.instanceId, props.userId, props.threadId],
    () => {
        if (props.visible) void reload();
    }
);
</script>
