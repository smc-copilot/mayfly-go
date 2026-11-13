import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { defineComponent, h, onMounted, ref } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import InstanceLibrary from './InstanceLibrary.vue';

const api = vi.hoisted(() => ({
    expertTemplates: { request: vi.fn() },
    skillPackages: { request: vi.fn() },
    skills: { request: vi.fn() },
}));
vi.mock('../api', () => ({ octopBusinessApi: api }));
vi.mock('@/i18n', () => ({ i18n: { global: { t: (key: string) => key, locale: { value: 'zh-cn' } } } }));
vi.mock('vue-i18n', async (importOriginal) => ({
    ...(await importOriginal<typeof import('vue-i18n')>()),
    useI18n: () => ({ t: (key: string) => key, te: () => true }),
}));

// Replace the table UI, keeping the page's real request/result hooks and actions.
const DataTable = defineComponent({
    props: ['pageApi', 'beforeQueryFn', 'dataHandlerFn', 'columns'],
    setup(props) {
        const rows = ref<Record<string, unknown>[]>([]);
        onMounted(async () => {
            const result = await props.pageApi.request(props.beforeQueryFn({ pageNum: 1, pageSize: 10 }));
            rows.value = props.dataHandlerFn(result).list;
        });
        return () =>
            h(
                'div',
                rows.value.flatMap((row) =>
                    props.columns.flatMap((column: { actions?: { key: string; label: string; onClick: (row: unknown) => void }[] }) =>
                        (column.actions || [])
                            .filter((action) => action.key === 'select')
                            .map((action) => h('button', { onClick: () => action.onClick(row) }, action.label))
                    )
                )
            );
    },
});
const Slot = defineComponent({
    setup:
        (_, { slots }) =>
        () =>
            h('div', slots.default?.()),
});
const InstanceSelector = defineComponent({
    emits: ['update:valid'],
    setup(_, { emit }) {
        onMounted(() => emit('update:valid', true));
        return () => null;
    },
});
let wrapper: VueWrapper | undefined;
afterEach(() => wrapper?.unmount());

describe('instance package skills source status', () => {
    it('shows a skill-list source failure instead of silently presenting an empty package', async () => {
        const ready = { source: 'octop_postgres', state: 'ready' };
        api.expertTemplates.request.mockResolvedValue({ list: [], total: 0, source: ready });
        api.skillPackages.request.mockResolvedValue({ list: [{ packageId: 'TN2R8B', name: '合同生成' }], total: 1, source: ready });
        api.skills.request.mockResolvedValue({
            list: [],
            total: 0,
            source: { source: 'octop_api', state: 'forbidden', message: 'Skill API credentials rejected' },
        });
        const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: Slot }] });
        await router.push('/?instanceId=2&userId=1');
        wrapper = mount(InstanceLibrary, {
            global: {
                plugins: [router],
                mocks: { $t: (key: string) => key },
                stubs: {
                    DataTable,
                    InstanceSelector,
                    UserSelector: true,
                    ExpertTemplateDetailDrawer: true,
                    SkillPackageDetailDrawer: true,
                    StatValueText: true,
                    EnumTag: true,
                    ElTabs: Slot,
                    ElTabPane: Slot,
                    ElButton: Slot,
                    ElTag: Slot,
                    ElAlert: true,
                    ElEmpty: true,
                    ElAvatar: true,
                },
            },
        });
        await flushPromises();
        await wrapper.get('button').trigger('click');
        await flushPromises();
        expect(wrapper.text()).toContain('Skill API credentials rejected');
    });
});
