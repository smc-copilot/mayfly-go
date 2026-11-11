<template>
    <el-drawer
        v-model="visibleProxy"
        :title="$t('octop.sessionTrace')"
        size="90%"
        destroy-on-close
        append-to-body
    >
        <div v-if="!instanceId || !userId || !threadId" class="p-4">
            <el-empty :description="$t('octop.selectThreadFirst')" />
        </div>
        <el-alert v-else-if="traceDenied" type="warning" :closable="false" :title="$t('octop.traceReadDenied')" />
        <div v-else class="h-full flex flex-col gap-3 px-1">
            <div class="flex flex-wrap items-center gap-2 text-sm">
                <span>{{ $t('octop.threadId') }}: {{ threadId }}</span>
                <span v-if="meta?.projectId" class="text-gray-500">{{ $t('octop.langfuseProjectId') }}: {{ meta.projectId }}</span>
                <span v-if="meta?.projectName" class="text-gray-500">{{ $t('octop.langfuseProjectName') }}: {{ meta.projectName }}</span>
                <span v-if="meta?.version" class="text-gray-500">{{ $t('octop.langfuseVersion') }}: {{ meta.version }}</span>
                <el-link v-if="externalUrl" type="primary" :href="externalUrl" target="_blank" rel="noopener noreferrer">
                    {{ $t('octop.openInLangfuse') }}
                </el-link>
                <el-button link type="primary" @click="onRefresh">{{ $t('common.refresh') }}</el-button>
            </div>
            <div data-testid="trace-connection-state">
                <div class="text-sm font-medium mb-1">{{ $t('octop.traceConnectionState') }}</div>
                <span v-if="loadingMeta" class="text-sm text-gray-500">{{ $t('octop.traceLoading') }}</span>
                <el-alert v-else-if="metaFailed" type="warning" :closable="false" :title="$t('octop.traceConnectionReadFailed')" />
                <read-status-bar v-else :meta="meta?.connection" />
            </div>
            <div data-testid="session-read-state" class="flex flex-wrap items-center gap-2 text-sm">
                <span class="font-medium">{{ $t('octop.sessionReadState') }}</span>
                <el-tag size="small" :type="assocTagType">{{ loadingTraces ? $t('octop.traceLoading') : assocLabel }}</el-tag>
            </div>
            <read-status-bar :meta="traceSource" />
            <el-alert v-if="traceFailed" type="error" :closable="false" :title="$t('octop.traceReadFailed')" />
            <el-alert v-if="meta?.message" :title="meta.message" type="info" :closable="false" />
            <el-alert
                v-if="association === 'association_unverified'"
                type="warning"
                :closable="false"
                :title="$t('octop.associationUnverified')"
            />
            <el-alert
                v-if="association === 'not_found_in_range'"
                type="info"
                :closable="false"
                :title="$t('octop.traceNotFoundInRange')"
            />

            <div class="flex-1 min-h-0 grid grid-cols-1 lg:grid-cols-3 gap-3">
                <div data-testid="trace-list" :aria-busy="loadingTraces" class="border rounded p-2 overflow-auto">
                    <div class="font-medium mb-2">{{ $t('octop.traceList') }}</div>
                    <template v-if="tracePage">
                        <div class="text-xs text-gray-500 mb-2">
                            {{ tracePage.total < 0 ? $t('octop.traceTotalUnknown', { count: traces.length }) : $t('octop.traceCount', { count: traces.length, total: tracePage.total }) }}
                        </div>
                        <div v-if="tracePage.coverageStart || tracePage.coverageEnd" class="text-xs text-gray-500 mb-2">
                            {{ $t('octop.traceQueryWindow') }}: {{ formatShanghai(tracePage.coverageStart) }} — {{ formatShanghai(tracePage.coverageEnd) }}
                        </div>
                    </template>
                    <div v-if="loadingTraces" class="text-sm text-gray-500">{{ $t('octop.traceLoading') }}</div>
                    <el-empty v-else-if="!traces.length && traceReadSucceeded" :description="$t('octop.noTraces')" />
                    <el-menu v-if="traces.length" :default-active="activeTraceId" @select="onSelectTrace">
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
                    <el-button v-if="tracePage?.hasMore" class="mt-2 w-full" :loading="loadingTraces" @click="onLoadMoreTraces">
                        {{ $t('octop.loadMore') }}
                    </el-button>
                </div>

                <div data-testid="observation-list" :aria-busy="loadingObs" class="border rounded p-2 overflow-auto">
                    <div class="font-medium mb-2">{{ $t('octop.observationTree') }}</div>
                    <el-empty v-if="!activeTraceId" :description="$t('octop.selectTraceFirst')" />
                    <div v-else-if="loadingObs" class="text-sm text-gray-500">{{ $t('octop.traceLoading') }}</div>
                    <el-empty v-else-if="!observations.length && obsReadSucceeded" :description="$t('octop.noObservations')" />
                    <read-status-bar v-if="obsSource" :meta="obsSource" />
                    <el-alert v-if="obsFailed" type="error" :closable="false" :title="$t('octop.observationReadFailed')" />
                    <template v-if="obsPage">
                        <div class="text-xs text-gray-500 mb-2">
                            {{ obsPage.total < 0 ? $t('octop.traceTotalUnknown', { count: observations.length }) : $t('octop.traceCount', { count: observations.length, total: obsPage.total }) }}
                        </div>
                    </template>
                    <el-tree
                        v-if="observations.length"
                        :data="obsTree"
                        node-key="observationId"
                        :props="{ label: 'label', children: 'children' }"
                        highlight-current
                        default-expand-all
                        @node-click="onSelectObs"
                    />
                    <el-button v-if="obsPage?.hasMore" class="mt-2 w-full" :loading="loadingObs" @click="onLoadMoreObservations">
                        {{ $t('octop.loadMoreObservations') }}
                    </el-button>
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
                            <el-descriptions-item :label="$t('octop.traceTokens')">
                                {{ tokenText(activeObs) }}
                            </el-descriptions-item>
                            <el-descriptions-item v-if="activeObs.orphan" :label="$t('octop.orphanNode')">
                                {{ $t('octop.parentMissing') }}
                            </el-descriptions-item>
                        </el-descriptions>
                        <div class="mt-3" v-auth="'octop:content:read'">
                            <el-button type="primary" plain :loading="loadingContent" @click="onLoadContent">
                                {{ $t('octop.viewContent') }}
                            </el-button>
                            <el-alert v-if="contentFailed" class="mt-2" type="error" :closable="false" :title="$t('octop.observationContentReadFailed')" />
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
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { hasPerm } from '@/components/auth/auth';
import { formatShanghai } from '../time';
import type { BizObservation } from '../types';
import { useSessionTrace, type ObservationNode } from '../composables/useSessionTrace';
import ReadStatusBar from '../components/ReadStatusBar.vue';

const props = defineProps<{
    instanceId?: number;
    userId?: string;
    threadId?: string;
}>();
const visibleProxy = defineModel<boolean>('visible', { default: false });
const { t } = useI18n();
const {
    meta, tracePage, traces, obsPage, observations, activeTraceId, activeObs, content,
    loadingMeta, loadingTraces, loadingObs, loadingContent, metaFailed, traceFailed, obsFailed, contentFailed,
    tree, association, traceSource, obsSource, traceDenied, traceReadSucceeded, obsReadSucceeded, externalUrl,
    reload: onRefresh, onSelectTrace, onSelectObs, loadContent: onLoadContent,
    loadMoreTraces: onLoadMoreTraces, loadMoreObservations: onLoadMoreObservations,
} = useSessionTrace(
    () => ({ visible: visibleProxy.value, instanceId: props.instanceId, userId: props.userId, threadId: props.threadId }),
    () => hasPerm('octop:trace:base'),
    () => hasPerm('octop:content:read')
);

const assocLabel = computed(() => {
    const labels: Record<string, string> = {
        ready: 'octop.stateReady',
        not_configured: 'octop.stateNotConfigured',
        not_verified: 'octop.stateNotVerified',
        association_unverified: 'octop.associationUnverified',
        not_found_in_range: 'octop.traceNotFoundInRange',
        forbidden: 'octop.stateForbidden',
        failed: 'octop.stateFailed',
        unsupported: 'octop.stateUnsupported',
        partial: 'octop.statePartial',
    };
    return t(labels[association.value] || 'octop.stateNotVerified');
});
const assocTagType = computed(() => {
    if (association.value === 'ready') return 'success';
    if (['association_unverified', 'failed', 'forbidden'].includes(association.value)) return 'danger';
    return 'warning';
});
const obsTree = computed(() => {
    const labelNodes = (nodes: ObservationNode[]): Array<ObservationNode & { label: string }> => nodes.map((node) => ({
        ...node,
        label: `${node.raw.type}${node.raw.name ? ' · ' + node.raw.name : ''}${node.raw.orphan ? ' (' + t('octop.orphanNode') + ')' : ''}${node.cycle ? ' (' + t('octop.observationCycle') + ')' : ''}`,
        children: labelNodes(node.children),
    }));
    return labelNodes(tree.value);
});
const tokenText = (observation: BizObservation) => {
    if (observation.inputTokens == null && observation.outputTokens == null) return t('octop.unknownMetric');
    return `${observation.inputTokens ?? '?'} / ${observation.outputTokens ?? '?'}`;
};
</script>
