import { computed, getCurrentScope, onScopeDispose, ref, watch } from 'vue';
import { useRoute, useRouter, type LocationQuery } from 'vue-router';

/** 路由最小契约：便于测试注入，运行时即 vue-router 的 route / router */
export interface OctopRouteLike {
    query: LocationQuery;
}
export interface OctopRouterLike {
    push: (to: { query: Record<string, string> }) => unknown;
    replace: (to: { query: Record<string, string> }) => unknown;
}

export interface OctopPageContextOptions {
    /** 允许的 tab 值；不传表示页面没有 tab，不读写 URL 的 tab 参数 */
    tabs?: readonly string[];
    defaultTab?: string;
    /** 写 URL 的方式，默认 replace（切换上下文不堆积历史） */
    navigate?: 'push' | 'replace';
    route?: OctopRouteLike;
    router?: OctopRouterLike;
}

export interface OctopContextValue {
    instanceId?: number;
    userId?: string;
    tab?: string;
}

const firstQuery = (v: LocationQuery[string] | undefined): string | undefined => {
    const raw = Array.isArray(v) ? v[0] : v;
    if (raw == null) return undefined;
    const s = String(raw).trim();
    return s === '' ? undefined : s;
};

/** 仅接受正整数，其余视为未指定（不回退到其他实例） */
export const parseInstanceId = (v: LocationQuery[string] | undefined): number | undefined => {
    const s = firstQuery(v);
    if (!s || !/^\d+$/.test(s)) return undefined;
    const n = Number(s);
    return Number.isSafeInteger(n) && n > 0 ? n : undefined;
};

export const parseUserId = (v: LocationQuery[string] | undefined): string | undefined => firstQuery(v);

export const parseTab = (v: LocationQuery[string] | undefined, tabs: readonly string[] | undefined, defaultTab: string | undefined): string | undefined => {
    if (!tabs || tabs.length === 0) return undefined;
    const s = firstQuery(v);
    if (s && tabs.includes(s)) return s;
    return defaultTab && tabs.includes(defaultTab) ? defaultTab : tabs[0];
};

/**
 * Octop 业务页上下文：URL query（instanceId / userId / tab）⇄ 状态。
 *
 * - 实例或用户变化 → contextGen 自增；切换实例同时清空用户。
 * - 异步请求发起前 captureGen()，返回后用 isCurrent(gen) 判断是否仍是当前上下文，过期响应直接丢弃（AC-13）。
 * - 组件卸载同样使上下文失效，卸载后到达的响应不会写回。
 * - tab 变化不改变上下文代数（同一实例/用户下的两个视图相互独立）。
 */
export function useOctopPageContext(options: OctopPageContextOptions = {}) {
    const route = options.route ?? useRoute();
    const router = options.router ?? useRouter();
    const { tabs, defaultTab } = options;
    const mode = options.navigate ?? 'replace';

    const parse = (): OctopContextValue => ({
        instanceId: parseInstanceId(route.query.instanceId),
        userId: parseUserId(route.query.userId),
        tab: parseTab(route.query.tab, tabs, defaultTab),
    });

    const initial = parse();
    const instanceId = ref<number | undefined>(initial.instanceId);
    const userId = ref<string | undefined>(initial.userId);
    const tab = ref<string | undefined>(initial.tab);
    const contextGen = ref(0);

    const hasInstance = computed(() => instanceId.value !== undefined);
    const hasUser = computed(() => instanceId.value !== undefined && !!userId.value);

    const captureGen = () => contextGen.value;
    const isCurrent = (gen: number) => gen === contextGen.value;
    const invalidate = () => {
        contextGen.value += 1;
    };

    const buildQuery = (): Record<string, string> => {
        const base: Record<string, string> = {};
        for (const [k, v] of Object.entries(route.query)) {
            if (k === 'instanceId' || k === 'userId' || k === 'tab') continue;
            const s = firstQuery(v);
            if (s !== undefined) base[k] = s;
        }
        if (instanceId.value !== undefined) base.instanceId = String(instanceId.value);
        if (userId.value) base.userId = userId.value;
        if (tabs && tab.value) base.tab = tab.value;
        return base;
    };

    const writeQuery = () => {
        const to = { query: buildQuery() };
        return mode === 'push' ? router.push(to) : router.replace(to);
    };

    /** 应用新上下文；返回是否发生变化。write=false 用于「URL 已经是新值」的场景 */
    const apply = (next: OctopContextValue, write: boolean): boolean => {
        const ctxChanged = next.instanceId !== instanceId.value || next.userId !== userId.value;
        const tabChanged = tabs !== undefined && next.tab !== tab.value;
        if (!ctxChanged && !tabChanged) return false;
        if (ctxChanged) {
            instanceId.value = next.instanceId;
            userId.value = next.userId;
            invalidate();
        }
        if (tabChanged) tab.value = next.tab;
        if (write) void writeQuery();
        return true;
    };

    /** 切换实例：清空用户，上下文失效 */
    const setInstance = (id: number | undefined) => apply({ instanceId: id, userId: undefined, tab: tab.value }, true);
    /** 切换用户：实例不变，上下文失效 */
    const setUser = (id: string | undefined) => apply({ instanceId: instanceId.value, userId: id || undefined, tab: tab.value }, true);
    const setTab = (t: string) => apply({ instanceId: instanceId.value, userId: userId.value, tab: parseTab(t, tabs, defaultTab) }, true);
    /** 一次性设置（如从列表进入详情），只触发一次上下文变化 */
    const setContext = (next: OctopContextValue) =>
        apply({ instanceId: next.instanceId, userId: next.userId || undefined, tab: next.tab === undefined ? tab.value : parseTab(next.tab, tabs, defaultTab) }, true);

    /**
     * 在当前上下文内执行异步加载：返回时若上下文已变化或页面已卸载，丢弃结果（不调用 apply）。
     * @returns 是否应用了结果
     */
    const loadInContext = async <T>(loader: () => Promise<T>, onResult: (value: T) => void): Promise<boolean> => {
        const gen = captureGen();
        const value = await loader();
        if (!isCurrent(gen)) return false;
        onResult(value);
        return true;
    };

    /**
     * 为 DataTable 生成绑定当前上下文的钩子（在 computed 中调用，上下文变化时重新生成并配合 :key 重挂载表格）：
     * - beforeQueryFn 总是使用「生成时」的路径参数快照，保证表格内的翻页/查询不会串到新上下文；
     * - dataHandlerFn 在上下文已变化后不再回调 onData（表格数据本身随旧表格卸载一并丢弃）。
     * 注意 DataTable 的 reset 会把 queryForm 里的非分页字段置空，所以路径参数不放进 queryForm，而是在这里注入。
     */
    const bindTable = <R = unknown>(params: Record<string, unknown>, onData?: (res: R) => void) => {
        const gen = captureGen();
        const snapshot = { ...params };
        return {
            beforeQueryFn: (p: Record<string, unknown>) => ({ ...p, ...snapshot }),
            dataHandlerFn: (res: R) => {
                if (isCurrent(gen)) onData?.(res);
                return res;
            },
        };
    };

    // 浏览器前进/后退或外部改写 URL：解析后同步（不回写）
    watch(
        () => route.query,
        () => {
            apply(parse(), false);
        },
        { flush: 'sync' }
    );

    if (getCurrentScope()) {
        onScopeDispose(invalidate);
    }

    return {
        instanceId,
        userId,
        tab,
        contextGen,
        hasInstance,
        hasUser,
        captureGen,
        isCurrent,
        invalidate,
        loadInContext,
        bindTable,
        setInstance,
        setUser,
        setTab,
        setContext,
    };
}

export type OctopPageContext = ReturnType<typeof useOctopPageContext>;
