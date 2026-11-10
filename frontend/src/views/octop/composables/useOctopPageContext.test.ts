import { effectScope, reactive } from 'vue';
import { describe, expect, it, vi } from 'vitest';
import type { LocationQuery } from 'vue-router';
import { parseInstanceId, parseTab, parseUserId, useOctopPageContext, type OctopRouteLike, type OctopRouterLike } from './useOctopPageContext';

const TABS = ['experts', 'sessions'] as const;

/** 内存路由：replace/push 直接改写 route.query（等价于一次同步导航） */
function makeRoute(query: LocationQuery = {}) {
    const route = reactive<OctopRouteLike>({ query });
    const calls: Array<{ kind: 'push' | 'replace'; query: Record<string, string> }> = [];
    const router: OctopRouterLike = {
        push: (to) => {
            calls.push({ kind: 'push', query: to.query });
            route.query = { ...to.query };
        },
        replace: (to) => {
            calls.push({ kind: 'replace', query: to.query });
            route.query = { ...to.query };
        },
    };
    return { route, router, calls };
}

const setup = (query: LocationQuery = {}, withTabs = true) => {
    const env = makeRoute(query);
    const scope = effectScope();
    const ctx = scope.run(() => useOctopPageContext({ route: env.route, router: env.router, tabs: withTabs ? TABS : undefined, defaultTab: 'experts' }))!;
    return { ...env, ctx, scope };
};

describe('query parsers', () => {
    it('instanceId only accepts positive integers', () => {
        expect(parseInstanceId('12')).toBe(12);
        expect(parseInstanceId(['7', '8'])).toBe(7);
        expect(parseInstanceId('0')).toBeUndefined();
        expect(parseInstanceId('-3')).toBeUndefined();
        expect(parseInstanceId('1.5')).toBeUndefined();
        expect(parseInstanceId('abc')).toBeUndefined();
        expect(parseInstanceId(undefined)).toBeUndefined();
    });
    it('userId keeps string identity and trims blanks', () => {
        expect(parseUserId(' 00042 ')).toBe('00042');
        expect(parseUserId('')).toBeUndefined();
        expect(parseUserId(null)).toBeUndefined();
    });
    it('tab falls back to default when invalid', () => {
        expect(parseTab('sessions', TABS, 'experts')).toBe('sessions');
        expect(parseTab('bogus', TABS, 'experts')).toBe('experts');
        expect(parseTab('sessions', undefined, undefined)).toBeUndefined();
    });
});

describe('useOctopPageContext', () => {
    it('parses instanceId / userId / tab from the URL', () => {
        const { ctx } = setup({ instanceId: '3', userId: 'u-1', tab: 'sessions' });
        expect(ctx.instanceId.value).toBe(3);
        expect(ctx.userId.value).toBe('u-1');
        expect(ctx.tab.value).toBe('sessions');
        expect(ctx.hasUser.value).toBe(true);
    });

    it('does not report a user when instance is missing', () => {
        const { ctx } = setup({ userId: 'u-1' });
        expect(ctx.hasUser.value).toBe(false);
    });

    it('defaults tab and ignores tab entirely on pages without tabs', () => {
        const { ctx } = setup({ instanceId: '1' });
        expect(ctx.tab.value).toBe('experts');
        const plain = setup({ instanceId: '1', tab: 'sessions' }, false);
        expect(plain.ctx.tab.value).toBeUndefined();
        plain.ctx.setUser('u-2');
        expect(plain.calls[0].query).toEqual({ instanceId: '1', userId: 'u-2' });
    });

    it('switching instance clears user, bumps gen once and syncs the URL', () => {
        const { ctx, calls, route } = setup({ instanceId: '1', userId: 'u-1', tab: 'sessions' });
        const g0 = ctx.contextGen.value;
        expect(ctx.setInstance(2)).toBe(true);
        expect(ctx.instanceId.value).toBe(2);
        expect(ctx.userId.value).toBeUndefined();
        expect(ctx.contextGen.value).toBe(g0 + 1);
        expect(calls).toHaveLength(1);
        expect(route.query).toEqual({ instanceId: '2', tab: 'sessions' });
    });

    it('switching user bumps gen; selecting the same user is a no-op', () => {
        const { ctx, calls } = setup({ instanceId: '1', userId: 'u-1' });
        const g0 = ctx.contextGen.value;
        expect(ctx.setUser('u-1')).toBe(false);
        expect(ctx.contextGen.value).toBe(g0);
        expect(calls).toHaveLength(0);
        expect(ctx.setUser('u-2')).toBe(true);
        expect(ctx.contextGen.value).toBe(g0 + 1);
    });

    it('tab change keeps the context generation', () => {
        const { ctx, route } = setup({ instanceId: '1', userId: 'u-1' });
        const g0 = ctx.contextGen.value;
        ctx.setTab('sessions');
        expect(ctx.tab.value).toBe('sessions');
        expect(ctx.contextGen.value).toBe(g0);
        expect(route.query).toEqual({ instanceId: '1', userId: 'u-1', tab: 'sessions' });
    });

    it('setContext changes instance and user together with a single bump', () => {
        const { ctx } = setup({ instanceId: '1' });
        const g0 = ctx.contextGen.value;
        ctx.setContext({ instanceId: 5, userId: 'u-9', tab: 'sessions' });
        expect(ctx.contextGen.value).toBe(g0 + 1);
        expect(ctx.instanceId.value).toBe(5);
        expect(ctx.userId.value).toBe('u-9');
        expect(ctx.tab.value).toBe('sessions');
    });

    it('preserves unrelated query keys when writing', () => {
        const { ctx, route } = setup({ instanceId: '1', foo: 'bar' });
        ctx.setUser('u-1');
        expect(route.query).toEqual({ foo: 'bar', instanceId: '1', userId: 'u-1', tab: 'experts' });
    });

    it('follows external URL changes (back/forward) without writing back', () => {
        const { ctx, route, calls } = setup({ instanceId: '1', userId: 'u-1' });
        const g0 = ctx.contextGen.value;
        route.query = { instanceId: '2', userId: 'u-7', tab: 'sessions' };
        expect(ctx.instanceId.value).toBe(2);
        expect(ctx.userId.value).toBe('u-7');
        expect(ctx.tab.value).toBe('sessions');
        expect(ctx.contextGen.value).toBe(g0 + 1);
        expect(calls).toHaveLength(0);
    });

    it('uses push when configured', () => {
        const env = makeRoute({ instanceId: '1' });
        const ctx = effectScope().run(() => useOctopPageContext({ route: env.route, router: env.router, navigate: 'push' }))!;
        ctx.setUser('u-1');
        expect(env.calls[0].kind).toBe('push');
    });
});

describe('stale response discard (AC-13)', () => {
    it('isCurrent is false for a generation captured before a switch', () => {
        const { ctx } = setup({ instanceId: '1', userId: 'u-1' });
        const gen = ctx.captureGen();
        expect(ctx.isCurrent(gen)).toBe(true);
        ctx.setUser('u-2');
        expect(ctx.isCurrent(gen)).toBe(false);
        expect(ctx.isCurrent(ctx.captureGen())).toBe(true);
    });

    it('a slow response for A resolving after B is applied never overwrites B', async () => {
        const { ctx } = setup({ instanceId: '1', userId: 'A' });
        let resolveA!: (v: string) => void;
        const slowA = new Promise<string>((r) => (resolveA = r));
        const shown: string[] = [];

        const pA = ctx.loadInContext(
            () => slowA,
            (v) => shown.push(v)
        );
        ctx.setUser('B');
        const appliedB = await ctx.loadInContext(
            () => Promise.resolve('data-B'),
            (v) => shown.push(v)
        );
        resolveA('data-A');
        const appliedA = await pA;

        expect(appliedB).toBe(true);
        expect(appliedA).toBe(false);
        expect(shown).toEqual(['data-B']);
    });

    it('bindTable injects a snapshot of path params and drops stale data callbacks', () => {
        const { ctx } = setup({ instanceId: '1', userId: 'A' });
        const onData = vi.fn();
        const hooks = ctx.bindTable({ id: ctx.instanceId.value, userId: ctx.userId.value }, onData);
        expect(hooks.beforeQueryFn({ pageNum: 2, keyword: 'x' })).toEqual({ pageNum: 2, keyword: 'x', id: 1, userId: 'A' });

        const res = { list: [], total: 0 };
        expect(hooks.dataHandlerFn(res)).toBe(res);
        expect(onData).toHaveBeenCalledTimes(1);

        ctx.setUser('B');
        // 旧表格的晚到响应：仍原样返回（不破坏 DataTable 流程），但不再回调页面
        expect(hooks.dataHandlerFn(res)).toBe(res);
        expect(onData).toHaveBeenCalledTimes(1);
        // 旧表格的后续翻页仍然打到旧上下文快照，不会串到 B
        expect(hooks.beforeQueryFn({ pageNum: 3 })).toMatchObject({ userId: 'A' });
    });

    it('responses arriving after the scope is disposed are discarded', async () => {
        const { ctx, scope } = setup({ instanceId: '1', userId: 'A' });
        let resolve!: (v: string) => void;
        const pending = new Promise<string>((r) => (resolve = r));
        const onResult = vi.fn();
        const p = ctx.loadInContext(() => pending, onResult);
        scope.stop();
        resolve('late');
        expect(await p).toBe(false);
        expect(onResult).not.toHaveBeenCalled();
    });
});
