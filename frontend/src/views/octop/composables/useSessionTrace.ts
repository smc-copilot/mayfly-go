import { computed, onScopeDispose, ref, watch } from 'vue';
import { octopBusinessApi } from '../api';
import type { BizObservation, BizObservationContent, BizTrace, SourceMeta, ThreadTraceMeta, TracePage } from '../types';

interface TraceContext {
    visible: boolean;
    instanceId?: number;
    userId?: string;
    threadId?: string;
}

export interface ObservationNode {
    observationId: string;
    children: ObservationNode[];
    raw: BizObservation;
    cycle: boolean;
}

/** Rebuild from all loaded pages: an upstream orphan flag can become obsolete after pagination. */
export function buildObservationTree(observations: BizObservation[]): ObservationNode[] {
    const nodes = new Map<string, ObservationNode>();
    for (const observation of observations) {
        nodes.set(observation.observationId, {
            observationId: observation.observationId,
            children: [],
            raw: { ...observation, orphan: false },
            cycle: false,
        });
    }
    const roots: ObservationNode[] = [];
    for (const node of nodes.values()) {
        const parentId = node.raw.parentId;
        if (!parentId) {
            roots.push(node);
            continue;
        }
        const parent = nodes.get(parentId);
        if (!parent) {
            node.raw.orphan = true;
            roots.push(node);
            continue;
        }
        // Follow the source parent chain before creating an edge, so no cycle enters el-tree.
        const seen = new Set([node.observationId]);
        let ancestor: ObservationNode | undefined = parent;
        while (ancestor && !seen.has(ancestor.observationId)) {
            seen.add(ancestor.observationId);
            ancestor = ancestor.raw.parentId ? nodes.get(ancestor.raw.parentId) : undefined;
        }
        if (ancestor?.observationId === node.observationId) {
            node.cycle = true;
            roots.push(node);
        } else {
            parent.children.push(node);
        }
    }
    return roots;
}

function mergeById<T>(previous: T[], incoming: T[], id: (item: T) => string, merge?: (previous: T, incoming: T) => T): T[] {
    const items = new Map(previous.map((item) => [id(item), item]));
    for (const item of incoming) {
        const key = id(item);
        const existing = items.get(key);
        items.set(key, existing !== undefined && merge ? merge(existing, item) : item);
    }
    return [...items.values()];
}

/** Cursor pages can contain different logical roots of the same trace. */
function mergeTraceRoots(previous: BizTrace, incoming: BizTrace): BizTrace {
    const timestamps = (values: Array<string | undefined>) => values
        .filter((value): value is string => !!value && Number.isFinite(Date.parse(value)))
        .sort((left, right) => Date.parse(left) - Date.parse(right));
    const startTime = timestamps([previous.startTime, incoming.startTime])[0];
    const endTime = timestamps([previous.endTime, incoming.endTime]).at(-1);
    const duration = startTime && endTime ? Date.parse(endTime) - Date.parse(startTime) : null;
    const levels: Record<string, number> = { ERROR: 3, WARNING: 2, DEFAULT: 1, DEBUG: 0 };
    const previousLevel = levels[previous.level?.toUpperCase() || ''] ?? -1;
    const incomingLevel = levels[incoming.level?.toUpperCase() || ''] ?? -1;
    const agreedTokens = (left: number | null | undefined, right: number | null | undefined) =>
        left != null && Number.isFinite(left) && left === right ? left : null;
    return {
        ...previous,
        ...incoming,
        name: incoming.name || previous.name,
        sessionId: incoming.sessionId || previous.sessionId,
        userId: incoming.userId || previous.userId,
        startTime,
        endTime,
        latencyMs: duration != null && duration >= 0 ? duration : null,
        level: previousLevel >= incomingLevel ? previous.level || incoming.level : incoming.level,
        hasError: !!(previous.hasError || incoming.hasError),
        tags: [...new Set([...(previous.tags || []), ...(incoming.tags || [])])],
        // Root metrics may overlap. Without a complete aggregate, adding them would invent a total.
        inputTokens: agreedTokens(previous.inputTokens, incoming.inputTokens),
        outputTokens: agreedTokens(previous.outputTokens, incoming.outputTokens),
    };
}

function emptyUnverifiedPage<T>(page: TracePage<T>): boolean {
    return page.list.length === 0 && page.association === 'not_verified' &&
        !!page.source && ['ready', 'partial'].includes(page.source.state);
}

/** Preserve the first request window and any incomplete coverage reported by earlier pages. */
function mergePage<T>(previous: TracePage<T> | null, incoming: TracePage<T>): TracePage<T> {
    if (!previous) return incoming;
    let source = incoming.source || previous.source;
    if (previous.source?.state === 'partial' && source && ['ready', 'partial'].includes(source.state)) {
        const messages = [...new Set([previous.source.message, source.message].filter(Boolean))];
        source = { ...source, state: 'partial', message: messages.join('; ') || undefined };
    }
    return {
        ...incoming,
        total: incoming.total < 0 || previous.total < 0 ? -1 : incoming.total,
        source,
        association: previous.association === 'ready' && emptyUnverifiedPage(incoming) ? 'ready' : incoming.association,
        coverageStart: previous.coverageStart || incoming.coverageStart,
        coverageEnd: previous.coverageEnd || incoming.coverageEnd,
        externalUrl: incoming.externalUrl || previous.externalUrl,
    };
}

const readSucceeded = <T>(page: TracePage<T> | null, failed: boolean) =>
    !!page && !failed && (!page.source || ['ready', 'partial'].includes(page.source.state)) &&
    (!page.association || ['ready', 'partial', 'not_found_in_range'].includes(page.association));

export function useSessionTrace(
    getContext: () => TraceContext,
    canReadTrace: () => boolean,
    canReadContent: () => boolean
) {
    const meta = ref<ThreadTraceMeta | null>(null);
    const tracePage = ref<TracePage<BizTrace> | null>(null);
    const traces = ref<BizTrace[]>([]);
    const obsPage = ref<TracePage<BizObservation> | null>(null);
    const observations = ref<BizObservation[]>([]);
    const activeTraceId = ref('');
    const activeObservationId = ref('');
    const content = ref<BizObservationContent | null>(null);
    const loadingMeta = ref(false);
    const loadingTraces = ref(false);
    const loadingObs = ref(false);
    const loadingContent = ref(false);
    const metaFailed = ref(false);
    const traceFailed = ref(false);
    const obsFailed = ref(false);
    const contentFailed = ref(false);
    const traceFailureSource = ref<SourceMeta | null>(null);
    const obsFailureSource = ref<SourceMeta | null>(null);
    let contextSequence = 0;
    let traceSequence = 0;
    let observationSequence = 0;
    let contentSequence = 0;
    let tracePageNum = 1;
    let obsPageNum = 1;
    let disposed = false;

    const tree = computed(() => buildObservationTree(observations.value));
    const activeObs = computed(() => {
        const find = (nodes: ObservationNode[]): BizObservation | null => {
            for (const node of nodes) {
                if (node.observationId === activeObservationId.value) return node.raw;
                const child = find(node.children);
                if (child) return child;
            }
            return null;
        };
        return find(tree.value);
    });
    const association = computed(() => {
        if (traceFailed.value) return traceFailureSource.value?.state || 'failed';
        const sourceState = tracePage.value?.source?.state;
        if (sourceState && !['ready', 'partial'].includes(sourceState)) return sourceState;
        return tracePage.value?.association || meta.value?.association || 'not_verified';
    });
    const traceSource = computed<SourceMeta | undefined>(() =>
        traceFailed.value ? traceFailureSource.value || { source: 'langfuse', state: 'failed', complete: false } : tracePage.value?.source
    );
    const obsSource = computed<SourceMeta | undefined>(() =>
        obsFailed.value ? obsFailureSource.value || { source: 'langfuse', state: 'failed', complete: false } : obsPage.value?.source
    );
    const traceReadSucceeded = computed(() => readSucceeded(tracePage.value, traceFailed.value));
    const obsReadSucceeded = computed(() => readSucceeded(obsPage.value, obsFailed.value));
    const externalUrl = computed(() => tracePage.value?.externalUrl || meta.value?.externalUrl);
    const traceDenied = computed(() => !canReadTrace());

    const params = () => {
        const context = getContext();
        if (disposed || !context.visible || !context.instanceId || !context.userId || !context.threadId || !canReadTrace()) return null;
        return { id: context.instanceId, userId: context.userId, threadId: context.threadId };
    };
    const isCurrent = (sequence: number) => sequence === contextSequence && !!params();

    const clearContent = () => {
        contentSequence++;
        content.value = null;
        loadingContent.value = false;
        contentFailed.value = false;
    };
    const clearObservations = () => {
        observationSequence++;
        obsPageNum = 1;
        observations.value = [];
        obsPage.value = null;
        activeObservationId.value = '';
        loadingObs.value = false;
        obsFailed.value = false;
        obsFailureSource.value = null;
        clearContent();
    };
    const clear = () => {
        contextSequence++;
        traceSequence++;
        meta.value = null;
        tracePage.value = null;
        traces.value = [];
        activeTraceId.value = '';
        tracePageNum = 1;
        loadingMeta.value = false;
        loadingTraces.value = false;
        metaFailed.value = false;
        traceFailed.value = false;
        traceFailureSource.value = null;
        clearObservations();
    };

    const onSelectObs = (node: ObservationNode) => {
        clearContent();
        activeObservationId.value = node.observationId;
    };
    const loadObservations = async (more = false) => {
        const path = params();
        const traceId = activeTraceId.value;
        if (!path || !traceId || (more && (loadingObs.value || !obsPage.value?.hasMore))) return;
        const context = contextSequence;
        const sequence = ++observationSequence;
        const current = () => isCurrent(context) && sequence === observationSequence && traceId === activeTraceId.value;
        const previous = more ? obsPage.value : null;
        const window = previous || tracePage.value;
        const pageNum = more ? obsPageNum + 1 : 1;
        loadingObs.value = true;
        obsFailed.value = false;
        obsFailureSource.value = null;
        try {
            const response = await octopBusinessApi.observations.request({
                ...path, traceId, pageNum, pageSize: 100,
                ...(window ? { startTime: window.coverageStart, endTime: window.coverageEnd } : {}),
                ...(previous ? { cursor: previous.nextCursor } : {}),
            });
            if (!current()) return;
            if (!response || !Array.isArray(response.list)) {
                obsFailed.value = true;
                return;
            }
            if (more && !readSucceeded(response, false) && !emptyUnverifiedPage(response)) {
                obsFailed.value = true;
                obsFailureSource.value = response.source && !['ready', 'partial'].includes(response.source.state) ? response.source : null;
                return;
            }
            obsPage.value = mergePage(previous, response);
            observations.value = mergeById(more ? observations.value : [], response.list, (item) => item.observationId);
            obsPageNum = pageNum;
        } catch {
            if (current()) obsFailed.value = true;
        } finally {
            if (current()) loadingObs.value = false;
        }
    };
    const onSelectTrace = async (traceId: string) => {
        if (!params()) return;
        clearObservations();
        activeTraceId.value = traceId;
        await loadObservations();
    };
    const loadTraces = async (more = false) => {
        const path = params();
        if (!path || (more && (loadingTraces.value || !tracePage.value?.hasMore))) return;
        const context = contextSequence;
        const sequence = ++traceSequence;
        const current = () => isCurrent(context) && sequence === traceSequence;
        const previous = more ? tracePage.value : null;
        const pageNum = more ? tracePageNum + 1 : 1;
        loadingTraces.value = true;
        traceFailed.value = false;
        traceFailureSource.value = null;
        try {
            const response = await octopBusinessApi.traces.request({
                ...path, pageNum, pageSize: 20,
                ...(previous ? { cursor: previous.nextCursor, startTime: previous.coverageStart, endTime: previous.coverageEnd } : {}),
            });
            if (!current()) return;
            if (!response || !Array.isArray(response.list)) {
                traceFailed.value = true;
                return;
            }
            if (more && !readSucceeded(response, false) && !emptyUnverifiedPage(response)) {
                traceFailed.value = true;
                traceFailureSource.value = response.source && !['ready', 'partial'].includes(response.source.state) ? response.source : null;
                return;
            }
            tracePage.value = mergePage(previous, response);
            traces.value = mergeById(more ? traces.value : [], response.list, (item) => item.traceId, mergeTraceRoots);
            tracePageNum = pageNum;
            if (!activeTraceId.value && traces.value.length && traceReadSucceeded.value) {
                void onSelectTrace(traces.value[0].traceId);
            }
        } catch {
            if (current()) traceFailed.value = true;
        } finally {
            if (current()) loadingTraces.value = false;
        }
    };
    const reload = async () => {
        clear();
        const path = params();
        if (!path) return;
        const context = contextSequence;
        loadingMeta.value = true;
        loadingTraces.value = true;
        try {
            const response = await octopBusinessApi.threadTraceStatus.request(path);
            if (!isCurrent(context)) return;
            meta.value = response || null;
            metaFailed.value = !response;
        } catch {
            if (isCurrent(context)) metaFailed.value = true;
        } finally {
            if (isCurrent(context)) loadingMeta.value = false;
        }
        if (isCurrent(context)) await loadTraces();
    };
    const loadContent = async () => {
        const path = params();
        const observationId = activeObservationId.value;
        const traceId = activeTraceId.value;
        if (!path || !traceId || !observationId || !canReadContent()) return;
        const context = contextSequence;
        const sequence = ++contentSequence;
        const current = () => isCurrent(context) && sequence === contentSequence &&
            traceId === activeTraceId.value && observationId === activeObservationId.value && canReadContent();
        loadingContent.value = true;
        contentFailed.value = false;
        content.value = null;
        try {
            const response = await octopBusinessApi.observationContent.request({
                ...path, traceId, observationId,
                startTime: obsPage.value?.coverageStart || tracePage.value?.coverageStart,
                endTime: obsPage.value?.coverageEnd || tracePage.value?.coverageEnd,
            });
            if (!current()) return;
            content.value = response || null;
            contentFailed.value = !response;
        } catch {
            if (current()) contentFailed.value = true;
        } finally {
            if (current()) loadingContent.value = false;
        }
    };

    watch(
        () => {
            const context = getContext();
            return [context.visible, context.instanceId, context.userId, context.threadId, canReadTrace()] as const;
        },
        () => { void reload(); },
        { immediate: true, flush: 'sync' }
    );
    watch(canReadContent, (allowed) => { if (!allowed) clearContent(); }, { flush: 'sync' });
    onScopeDispose(() => {
        disposed = true;
        clear();
    });

    return {
        meta, tracePage, traces, obsPage, observations, activeTraceId, activeObs, content,
        loadingMeta, loadingTraces, loadingObs, loadingContent, metaFailed, traceFailed, obsFailed, contentFailed,
        tree, association, traceSource, obsSource, traceDenied, traceReadSucceeded, obsReadSucceeded, externalUrl,
        reload, onSelectTrace, onSelectObs, loadContent,
        loadMoreTraces: () => loadTraces(true),
        loadMoreObservations: () => loadObservations(true),
    };
}
