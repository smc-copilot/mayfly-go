import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { effectScope, type EffectScope } from 'vue';
import { useSessionTrace } from './useSessionTrace';
import type { BizObservation, BizTrace, TracePage } from '../types';

const api = vi.hoisted(() => ({
    threadTraceStatus: { request: vi.fn() },
    traces: { request: vi.fn() },
    observations: { request: vi.fn() },
    observationContent: { request: vi.fn() },
}));
vi.mock('../api', () => ({ octopBusinessApi: api }));

const scopes: EffectScope[] = [];
const page = <T>(list: T[], more = false): TracePage<T> => ({
    list, total: -1, pageNum: 1, pageSize: 20,
    hasMore: more, nextCursor: more ? 'next-cursor' : undefined,
    source: { source: 'langfuse', state: 'ready', complete: !more },
    association: 'ready', coverageStart: '2026-11-01T00:00:00Z', coverageEnd: '2026-11-10T00:00:00Z',
});
const root = (overrides: Partial<BizTrace> = {}): BizTrace => ({
    traceId: 'trace-1', name: 'turn', sessionId: 'thread-1', userId: 'user-1',
    startTime: '2026-11-10T08:00:01Z', endTime: '2026-11-10T08:00:02Z', latencyMs: 1000,
    level: 'DEFAULT', hasError: false, tags: [], inputTokens: null, outputTokens: null,
    ...overrides,
});
const openTrace = async (pages: BizTrace[][]) => {
    for (let index = 0; index < pages.length; index++) {
        api.traces.request.mockResolvedValueOnce(page(pages[index], index < pages.length - 1));
    }
    const scope = effectScope();
    scopes.push(scope);
    const state = scope.run(() => useSessionTrace(
        () => ({ visible: true, instanceId: 1, userId: 'user-1', threadId: 'thread-1' }),
        () => true,
        () => true
    ))!;
    await flushPromises();
    return state;
};

beforeEach(() => {
    vi.resetAllMocks();
    api.threadTraceStatus.request.mockResolvedValue({ association: 'not_verified' });
    api.observations.request.mockResolvedValue(page<BizObservation>([]));
});
afterEach(() => {
    for (const scope of scopes) scope.stop();
    scopes.length = 0;
});

describe('trace roots spanning cursor pages', () => {
    it('keeps the full time range and recomputes latency when a later root ends later', async () => {
        const state = await openTrace([
            [root({ endTime: '2026-11-10T08:00:04Z', latencyMs: 3000 })],
            [root({ startTime: '2026-11-10T08:00:02Z', endTime: '2026-11-10T08:00:07Z', latencyMs: 5000 })],
        ]);
        await state.loadMoreTraces();
        expect(state.traces.value).toHaveLength(1);
        expect(state.traces.value[0]).toMatchObject({
            startTime: '2026-11-10T08:00:01Z', endTime: '2026-11-10T08:00:07Z', latencyMs: 6000,
        });
    });

    it('keeps the full time range when a later page has an earlier shorter root', async () => {
        const state = await openTrace([
            [root({ endTime: '2026-11-10T08:00:04Z', latencyMs: 3000 })],
            [root({ startTime: '2026-11-10T08:00:00Z', endTime: '2026-11-10T08:00:03Z', latencyMs: 3000 })],
        ]);
        await state.loadMoreTraces();
        expect(state.traces.value[0]).toMatchObject({
            startTime: '2026-11-10T08:00:00Z', endTime: '2026-11-10T08:00:04Z', latencyMs: 4000,
        });
    });

    it.each([
        ['ERROR', 'DEFAULT', 'ERROR'],
        ['WARNING', 'DEFAULT', 'WARNING'],
        ['ERROR', 'WARNING', 'ERROR'],
    ])('keeps a prior %s level when the next root reports %s', async (previous, incoming, expected) => {
        const state = await openTrace([
            [root({ level: previous, hasError: true })],
            [root({ level: incoming, hasError: false })],
        ]);
        await state.loadMoreTraces();
        expect(state.traces.value[0].hasError).toBe(true);
        expect(state.traces.value[0].level).toBe(expected);
    });

    it('unions tags across roots without duplicates', async () => {
        const state = await openTrace([
            [root({ tags: ['first', 'shared'] })],
            [root({ tags: ['second', 'shared'] })],
        ]);
        await state.loadMoreTraces();
        expect(state.traces.value[0].tags).toEqual(['first', 'shared', 'second']);
    });

    it('does not add equal token totals and reports conflicting totals as unknown', async () => {
        const state = await openTrace([
            [root({ inputTokens: 12, outputTokens: 7 })],
            [root({ inputTokens: 12, outputTokens: 8 })],
        ]);
        await state.loadMoreTraces();
        expect(state.traces.value[0].inputTokens).toBe(12);
        expect(state.traces.value[0].outputTokens).toBeNull();
    });

    it('keeps missing or previously conflicting token totals unknown on subsequent pages', async () => {
        const state = await openTrace([
            [root({ inputTokens: 10, outputTokens: null })],
            [root({ inputTokens: 20, outputTokens: 8 })],
            [root({ inputTokens: 20, outputTokens: 8 })],
        ]);
        await state.loadMoreTraces();
        await state.loadMoreTraces();
        expect(state.traces.value[0].inputTokens).toBeNull();
        expect(state.traces.value[0].outputTokens).toBeNull();
    });

    it('continues replacing observation snapshots by ID instead of aggregating their metrics', async () => {
        api.observations.request
            .mockResolvedValueOnce(page<BizObservation>([{
                observationId: 'observation-1', traceId: 'trace-1', type: 'SPAN', name: 'old', inputTokens: 10,
            }], true))
            .mockResolvedValueOnce(page<BizObservation>([{
                observationId: 'observation-1', traceId: 'trace-1', type: 'SPAN', name: 'updated', inputTokens: 3,
            }]));
        const state = await openTrace([[root()]]);
        await state.loadMoreObservations();
        expect(state.observations.value).toHaveLength(1);
        expect(state.observations.value[0]).toMatchObject({ name: 'updated', inputTokens: 3 });
    });
});
