import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { defineComponent, h, inject, onMounted, provide, type PropType } from 'vue';
import SessionTraceDrawer from './SessionTraceDrawer.vue';
import type { BizObservation, BizObservationContent, BizTrace, ThreadTraceMeta, TracePage } from '../types';

const api = vi.hoisted(() => ({
    threadTraceStatus: { request: vi.fn() },
    traces: { request: vi.fn() },
    observations: { request: vi.fn() },
    observationContent: { request: vi.fn() },
}));
const permissions = vi.hoisted(() => ({ base: true, content: true }));
vi.mock('../api', () => ({ octopBusinessApi: api }));
vi.mock('@/components/auth/auth', () => ({
    hasPerm: (permission: string) => (permission === 'octop:trace:base' ? permissions.base : permissions.content),
}));
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, te: () => true }) }));

type TreeNode = { observationId: string; label: string; raw: BizObservation; children?: TreeNode[] };
const Slot = defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) });
const Drawer = defineComponent({
    props: { modelValue: Boolean },
    emits: ['opened', 'update:modelValue'],
    setup(props, { slots, emit }) {
        onMounted(() => {
            if (props.modelValue) emit('opened');
        });
        return () => (props.modelValue ? h('div', slots.default?.()) : null);
    },
});
const Button = defineComponent({
    props: { loading: Boolean, disabled: Boolean },
    setup: (props, { slots }) => () => h('button', { 'data-loading': String(props.loading), disabled: props.disabled }, slots.default?.()),
});
const Menu = defineComponent({
    emits: ['select'],
    setup(_, { emit, slots }) {
        provide('selectTrace', (id: string) => emit('select', id));
        return () => h('div', slots.default?.());
    },
});
const MenuItem = defineComponent({
    props: { index: { type: String, required: true } },
    setup(props, { slots }) {
        const select = inject<(id: string) => void>('selectTrace')!;
        return () => h('button', { 'data-trace': props.index, onClick: () => select(props.index) }, slots.default?.());
    },
});
const Tree = defineComponent({
    props: { data: { type: Array as PropType<TreeNode[]>, default: () => [] } },
    emits: ['node-click'],
    setup(props, { emit }) {
        const renderNodes = (nodes: TreeNode[], depth = 0, seen = new Set<string>()): ReturnType<typeof h>[] =>
            nodes.flatMap((node) => {
                if (seen.has(node.observationId)) return [];
                seen.add(node.observationId);
                return [
                    h('button', {
                        'data-observation': node.observationId,
                        'data-depth': depth,
                        onClick: () => emit('node-click', node),
                    }, node.label),
                    ...renderNodes(node.children || [], depth + 1, seen),
                ];
            });
        return () => h('div', renderNodes(props.data));
    },
});
const Empty = defineComponent({ props: ['description'], setup: (props) => () => h('p', { 'data-empty': '' }, props.description) });
const Alert = defineComponent({ props: ['title'], setup: (props) => () => h('p', props.title) });

const page = <T>(list: T[], extra: Partial<TracePage<T>> = {}): TracePage<T> => ({
    list, total: list.length, pageNum: 1, pageSize: 20, hasMore: false,
    association: list.length ? 'ready' : 'not_found_in_range',
    source: { source: 'langfuse', state: 'ready', complete: true },
    ...extra,
});
const observation = (observationId: string, parentId?: string): BizObservation => ({ observationId, parentId, traceId: 'trace-a', type: 'SPAN' });
const deferred = <T>() => {
    let resolve!: (value: T) => void;
    let reject!: (error: Error) => void;
    const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
    return { promise, resolve, reject };
};
let wrappers: VueWrapper[] = [];
const openDrawer = async () => {
    const wrapper = mount(SessionTraceDrawer, {
        props: { visible: true, instanceId: 1, userId: 'user-a', threadId: 'thread-a' },
        global: {
            mocks: { $t: (key: string) => key },
            directives: { auth: {} },
            stubs: {
                ElDrawer: Drawer, ElButton: Button, ElMenu: Menu, ElMenuItem: MenuItem, ElTree: Tree,
                ElEmpty: Empty, ElAlert: Alert, ElTag: Slot, ElLink: Slot, ElDescriptions: Slot,
                ElDescriptionsItem: Slot,
            },
        },
    });
    wrappers.push(wrapper);
    await flushPromises();
    return wrapper;
};
const clickButton = async (wrapper: VueWrapper, text: string) => {
    const button = wrapper.findAll('button').find((candidate) => candidate.text() === text);
    expect(button, `button ${text}`).toBeDefined();
    await button!.trigger('click');
    await flushPromises();
};

beforeEach(() => {
    vi.resetAllMocks();
    permissions.base = true;
    permissions.content = true;
    api.threadTraceStatus.request.mockResolvedValue({ association: 'not_verified', connection: { source: 'langfuse', state: 'ready' } });
    api.traces.request.mockResolvedValue(page<BizTrace>([]));
    api.observations.request.mockResolvedValue(page<BizObservation>([]));
    api.observationContent.request.mockResolvedValue({ observationId: 'obs-a', input: 'current-input' });
});
afterEach(() => {
    for (const wrapper of wrappers) wrapper.unmount();
    wrappers = [];
});

describe('SessionTraceDrawer read status', () => {
    it('shows current session failure ahead of old ready metadata and keeps connection capability separate', async () => {
        api.threadTraceStatus.request.mockResolvedValue({ association: 'ready', connection: { source: 'langfuse', state: 'ready' } });
        api.traces.request.mockResolvedValue(page([], { association: 'failed', source: { source: 'langfuse', state: 'failed' } }));
        const wrapper = await openDrawer();
        expect(wrapper.get('[data-testid="session-read-state"]').text()).toContain('octop.stateFailed');
        expect(wrapper.get('[data-testid="trace-connection-state"]').text()).toContain('octop.stateReady');
        expect(wrapper.text()).not.toContain('octop.noTraces');
    });

    it('does not turn a missing response into a successful empty trace result', async () => {
        api.traces.request.mockResolvedValue(undefined);
        const wrapper = await openDrawer();
        expect(wrapper.text()).toContain('octop.traceReadFailed');
        expect(wrapper.text()).not.toContain('octop.noTraces');
    });

    it('shows empty results only after a successful current session read', async () => {
        const wrapper = await openDrawer();
        expect(wrapper.get('[data-testid="session-read-state"]').text()).toContain('octop.traceNotFoundInRange');
        expect(wrapper.text()).toContain('octop.noTraces');
    });

    it('does not issue metadata or content reads without the respective permission', async () => {
        permissions.base = false;
        const wrapper = await openDrawer();
        expect(wrapper.text()).toContain('octop.traceReadDenied');
        expect(api.threadTraceStatus.request).not.toHaveBeenCalled();
        expect(api.traces.request).not.toHaveBeenCalled();
    });
});

describe('SessionTraceDrawer pagination', () => {
    it('passes the trace cursor and the original window and deduplicates trace IDs', async () => {
        api.traces.request
            .mockResolvedValueOnce(page([{ traceId: 'trace-a' }], {
                total: -1, hasMore: true, nextCursor: 'trace-cursor',
                coverageStart: '2026-11-01T00:00:00Z', coverageEnd: '2026-11-10T00:00:00Z',
                source: { source: 'langfuse', state: 'ready', complete: false },
            }))
            .mockResolvedValueOnce(page([{ traceId: 'trace-a' }, { traceId: 'trace-b' }], { total: -1 }));
        const wrapper = await openDrawer();
        await clickButton(wrapper, 'octop.loadMore');
        expect(api.traces.request.mock.calls[1][0]).toMatchObject({
            id: 1, userId: 'user-a', threadId: 'thread-a', pageNum: 2,
            cursor: 'trace-cursor', startTime: '2026-11-01T00:00:00Z', endTime: '2026-11-10T00:00:00Z',
        });
        expect(wrapper.findAll('[data-trace]')).toHaveLength(2);
        expect(wrapper.text()).not.toContain('octop.readIncomplete');
        expect(wrapper.text()).toContain('octop.traceTotalUnknown');
    });

    it('passes the trace window on the first observation read and the observation window on content reads', async () => {
        api.traces.request.mockResolvedValue(page([{ traceId: 'trace-a' }], { coverageStart: 'trace-start', coverageEnd: 'trace-end' }));
        api.observations.request.mockResolvedValue(page([observation('obs-a')], { coverageStart: 'obs-start', coverageEnd: 'obs-end' }));
        const wrapper = await openDrawer();
        expect(api.observations.request.mock.calls[0][0]).toMatchObject({ startTime: 'trace-start', endTime: 'trace-end' });
        await wrapper.get('[data-observation="obs-a"]').trigger('click');
        await clickButton(wrapper, 'octop.viewContent');
        expect(api.observationContent.request.mock.calls[0][0]).toMatchObject({ startTime: 'obs-start', endTime: 'obs-end' });
    });

    it('retains a partial read warning from an earlier page after pagination finishes', async () => {
        api.traces.request
            .mockResolvedValueOnce(page([{ traceId: 'trace-a' }], {
                hasMore: true, nextCursor: 'cursor', source: { source: 'langfuse', state: 'partial', complete: false, message: 'coverage-gap' },
            }))
            .mockResolvedValueOnce(page([{ traceId: 'trace-b' }]));
        const wrapper = await openDrawer();
        await clickButton(wrapper, 'octop.loadMore');
        expect(wrapper.text()).toContain('octop.statePartial');
        expect(wrapper.text()).toContain('coverage-gap');
        expect(wrapper.text()).not.toContain('octop.readIncomplete');
    });

    it('keeps loaded traces and retries a failed continuation with the same cursor', async () => {
        api.traces.request
            .mockResolvedValueOnce(page([{ traceId: 'trace-a' }], { hasMore: true, nextCursor: 'retry-cursor' }))
            .mockResolvedValueOnce(page([], { association: 'failed', source: { source: 'langfuse', state: 'failed' } }))
            .mockResolvedValueOnce(page([{ traceId: 'trace-b' }]));
        const wrapper = await openDrawer();
        await clickButton(wrapper, 'octop.loadMore');
        expect(wrapper.findAll('[data-trace]').map((node) => node.attributes('data-trace'))).toEqual(['trace-a']);
        expect(wrapper.get('[data-testid="session-read-state"]').text()).toContain('octop.stateFailed');
        await clickButton(wrapper, 'octop.loadMore');
        expect(api.traces.request.mock.calls[2][0]).toMatchObject({ cursor: 'retry-cursor', pageNum: 2 });
        expect(wrapper.findAll('[data-trace]').map((node) => node.attributes('data-trace'))).toEqual(['trace-a', 'trace-b']);
    });

    it('keeps loaded observations and retries a failed continuation with the same cursor', async () => {
        api.traces.request.mockResolvedValue(page([{ traceId: 'trace-a' }]));
        api.observations.request
            .mockResolvedValueOnce(page([observation('obs-a')], { hasMore: true, nextCursor: 'retry-obs' }))
            .mockResolvedValueOnce(page([], { association: 'failed', source: { source: 'langfuse', state: 'failed' } }))
            .mockResolvedValueOnce(page([observation('obs-b')]));
        const wrapper = await openDrawer();
        await clickButton(wrapper, 'octop.loadMoreObservations');
        expect(wrapper.findAll('[data-observation]').map((node) => node.attributes('data-observation'))).toEqual(['obs-a']);
        expect(wrapper.text()).toContain('octop.observationReadFailed');
        await clickButton(wrapper, 'octop.loadMoreObservations');
        expect(api.observations.request.mock.calls[2][0]).toMatchObject({ cursor: 'retry-obs', pageNum: 2 });
        expect(wrapper.findAll('[data-observation]')).toHaveLength(2);
    });

    it('continues through empty v4 cursor pages until a current-session match is found', async () => {
        api.traces.request
            .mockResolvedValueOnce(page([], { association: 'not_verified', hasMore: true, nextCursor: 'empty-1' }))
            .mockResolvedValueOnce(page([], { association: 'not_verified', hasMore: true, nextCursor: 'empty-2' }))
            .mockResolvedValueOnce(page([{ traceId: 'matched-trace' }]));
        const wrapper = await openDrawer();
        expect(wrapper.get('[data-testid="session-read-state"]').text()).toContain('octop.stateNotVerified');
        expect(wrapper.text()).not.toContain('octop.noTraces');
        await clickButton(wrapper, 'octop.loadMore');
        expect(wrapper.text()).not.toContain('octop.traceReadFailed');
        await clickButton(wrapper, 'octop.loadMore');
        expect(api.traces.request.mock.calls[2][0]).toMatchObject({ cursor: 'empty-2', pageNum: 3 });
        expect(wrapper.find('[data-trace="matched-trace"]').exists()).toBe(true);
        expect(wrapper.get('[data-testid="session-read-state"]').text()).toContain('octop.stateReady');
    });

    it('keeps a verified session association across a later successful empty v4 cursor page', async () => {
        api.traces.request
            .mockResolvedValueOnce(page([{ traceId: 'trace-a' }], { hasMore: true, nextCursor: 'after-match' }))
            .mockResolvedValueOnce(page([], { association: 'not_verified', hasMore: true, nextCursor: 'empty-after-match' }));
        const wrapper = await openDrawer();
        await clickButton(wrapper, 'octop.loadMore');
        expect(wrapper.get('[data-testid="session-read-state"]').text()).toContain('octop.stateReady');
        expect(wrapper.findAll('[data-trace]')).toHaveLength(1);
        expect(wrapper.text()).not.toContain('octop.traceReadFailed');
    });

    it('loads observations by cursor, deduplicates IDs and reattaches parents loaded on later pages', async () => {
        api.traces.request.mockResolvedValue(page([{ traceId: 'trace-a' }]));
        api.observations.request
            .mockResolvedValueOnce(page([{ ...observation('child', 'parent'), orphan: true }], {
                total: -1, hasMore: true, nextCursor: 'obs-cursor', coverageStart: 'start', coverageEnd: 'end',
            }))
            .mockResolvedValueOnce(page([observation('child', 'parent'), observation('parent')]));
        const wrapper = await openDrawer();
        expect(wrapper.get('[data-observation="child"]').attributes('data-depth')).toBe('0');
        await clickButton(wrapper, 'octop.loadMoreObservations');
        expect(api.observations.request.mock.calls[1][0]).toMatchObject({
            traceId: 'trace-a', pageNum: 2, cursor: 'obs-cursor', startTime: 'start', endTime: 'end',
        });
        expect(wrapper.findAll('[data-observation]')).toHaveLength(2);
        expect(wrapper.get('[data-observation="child"]').attributes('data-depth')).toBe('1');
        expect(wrapper.get('[data-observation="child"]').text()).not.toContain('octop.orphanNode');
    });

    it('keeps every observation reachable when a node parents itself or parents form a cycle', async () => {
        api.traces.request.mockResolvedValue(page([{ traceId: 'trace-a' }]));
        api.observations.request.mockResolvedValue(page([
            observation('self', 'self'), observation('cycle-a', 'cycle-b'), observation('cycle-b', 'cycle-a'), observation('leaf', 'cycle-b'),
        ]));
        const wrapper = await openDrawer();
        expect(wrapper.findAll('[data-observation]').map((node) => node.attributes('data-observation')).sort())
            .toEqual(['cycle-a', 'cycle-b', 'leaf', 'self']);
        expect(wrapper.text()).toContain('octop.observationCycle');
    });
});

describe('SessionTraceDrawer request contexts', () => {
    it('discards old metadata and trace data after instance, user and session change', async () => {
        const oldMeta = deferred<ThreadTraceMeta>();
        api.threadTraceStatus.request.mockReturnValueOnce(oldMeta.promise);
        const wrapper = await openDrawer();
        api.threadTraceStatus.request.mockResolvedValue({ association: 'not_verified', projectId: 'new-project' });
        api.traces.request.mockResolvedValue(page([{ traceId: 'new-trace' }]));
        await wrapper.setProps({ instanceId: 2, userId: 'user-b', threadId: 'thread-b' });
        await flushPromises();
        oldMeta.resolve({ association: 'ready', projectId: 'old-project' });
        await flushPromises();
        expect(wrapper.text()).toContain('new-project');
        expect(wrapper.text()).not.toContain('old-project');
        expect(api.traces.request.mock.calls.every(([params]) => params.id === 2)).toBe(true);
    });

    it('does not let a stale trace request clear loading or replace newer refresh results', async () => {
        const old = deferred<TracePage<BizTrace>>();
        const current = deferred<TracePage<BizTrace>>();
        api.traces.request.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
        const wrapper = await openDrawer();
        await clickButton(wrapper, 'common.refresh');
        old.resolve(page([{ traceId: 'old-trace' }]));
        await flushPromises();
        expect(wrapper.find('[data-trace="old-trace"]').exists()).toBe(false);
        expect(wrapper.get('[data-testid="trace-list"]').attributes('aria-busy')).toBe('true');
        current.resolve(page([{ traceId: 'new-trace' }]));
        await flushPromises();
        expect(wrapper.find('[data-trace="new-trace"]').exists()).toBe(true);
        expect(wrapper.get('[data-testid="trace-list"]').attributes('aria-busy')).toBe('false');
    });

    it('discards late observations and their finally when switching traces quickly', async () => {
        const old = deferred<TracePage<BizObservation>>();
        const current = deferred<TracePage<BizObservation>>();
        api.traces.request.mockResolvedValue(page([{ traceId: 'trace-a' }, { traceId: 'trace-b' }]));
        api.observations.request.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
        const wrapper = await openDrawer();
        await wrapper.get('[data-trace="trace-b"]').trigger('click');
        old.resolve(page([observation('old-observation')]));
        await flushPromises();
        expect(wrapper.find('[data-observation="old-observation"]').exists()).toBe(false);
        expect(wrapper.get('[data-testid="observation-list"]').attributes('aria-busy')).toBe('true');
        current.resolve(page([{ ...observation('new-observation'), traceId: 'trace-b' }]));
        await flushPromises();
        expect(wrapper.find('[data-observation="new-observation"]').exists()).toBe(true);
    });

    it('clears content on observation change and discards late content responses', async () => {
        const old = deferred<BizObservationContent>();
        api.traces.request.mockResolvedValue(page([{ traceId: 'trace-a' }]));
        api.observations.request.mockResolvedValue(page([observation('obs-a'), observation('obs-b')]));
        api.observationContent.request.mockReturnValueOnce(old.promise);
        const wrapper = await openDrawer();
        await wrapper.get('[data-observation="obs-a"]').trigger('click');
        await clickButton(wrapper, 'octop.viewContent');
        await wrapper.get('[data-observation="obs-b"]').trigger('click');
        old.resolve({ observationId: 'obs-a', input: 'old-private-content' });
        await flushPromises();
        expect(wrapper.text()).not.toContain('old-private-content');
        permissions.content = false;
        await clickButton(wrapper, 'octop.viewContent');
        expect(api.observationContent.request).toHaveBeenCalledTimes(1);
    });

    it('clears the current content and discards results arriving after close', async () => {
        const pending = deferred<TracePage<BizTrace>>();
        api.traces.request.mockReturnValueOnce(pending.promise);
        const wrapper = await openDrawer();
        await wrapper.setProps({ visible: false });
        pending.resolve(page([{ traceId: 'closed-trace' }]));
        await flushPromises();
        api.threadTraceStatus.request.mockReturnValue(new Promise(() => {}));
        await wrapper.setProps({ visible: true });
        await flushPromises();
        expect(wrapper.text()).not.toContain('closed-trace');
        expect(api.observations.request).not.toHaveBeenCalled();
    });
});
