import { describe, it, expect, vi } from 'vitest';
import { defineComponent } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia } from 'pinia';
import ServiceOrganizationSelect from '@/views/casdoor/components/ServiceOrganizationSelect.vue';
import { useCasdoorContext } from '@/views/casdoor/context';
const mocked = vi.hoisted(() => ({ list: vi.fn(), organizations: vi.fn(), sync: vi.fn() }));
vi.mock('@/views/casdoor/service-api', () => ({ serviceApi: { list: { request: mocked.list }, organizations: { request: mocked.organizations }, sync: { request: mocked.sync } } }));
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }));
const Select = defineComponent({ name: 'ElSelect', props: ['modelValue'], emits: ['change'], template: '<div><slot /></div>' });

describe('Casdoor service context', () => {
    it('does not restore an old page context after its service response completes following unmount', async () => {
        let resolveList!: (value: unknown) => void;
        mocked.list.mockImplementation(() => new Promise((resolve) => { resolveList = resolve; }));
        mocked.organizations.mockClear();
        const pinia = createPinia();
        const wrapper = mount(ServiceOrganizationSelect, { global: { plugins: [pinia], mocks: { $t: (key: string) => key }, directives: { auth: {} }, stubs: { ElSelect: Select, ElOption: true, ElButton: true, ElAlert: true, ElDialog: true, ElTable: true, ElTableColumn: true } } });
        await flushPromises();
        wrapper.unmount();
        const selection = useCasdoorContext(pinia);
        selection.serviceId = 2; selection.organizationId = 20;
        resolveList([{ id: 1, name: 'old' }]);
        await flushPromises();
        expect(selection.serviceId).toBe(2);
        expect(selection.organizationId).toBe(20);
        expect(mocked.organizations).not.toHaveBeenCalled();
    });
    it('ignores a previous service organization response after switching services', async () => {
        let resolveFirst!: (value: unknown) => void;
        mocked.list.mockResolvedValue([{ id: 1, name: 'first' }, { id: 2, name: 'second' }]);
        mocked.organizations.mockImplementation(({ serviceId }) => serviceId === 1 ? new Promise((resolve) => { resolveFirst = resolve; }) : Promise.resolve([{ id: 20, serviceId: 2, name: 'second-org' }]));
        const wrapper = mount(ServiceOrganizationSelect, { global: { plugins: [createPinia()], mocks: { $t: (key: string) => key }, directives: { auth: {} }, stubs: { ElSelect: Select, ElOption: true, ElButton: true, ElAlert: true, ElDialog: true, ElTable: true, ElTableColumn: true } } });
        await flushPromises();
        wrapper.findAllComponents(Select)[0].vm.$emit('change', 2);
        await flushPromises();
        resolveFirst([{ id: 10, serviceId: 1, name: 'first-org' }]);
        await flushPromises();
        const events = wrapper.emitted('change') || [];
        expect(events.at(-1)?.[0]).toEqual({ serviceId: 2, organizationId: 20 });
        expect(wrapper.findAllComponents(Select)[1].props('modelValue')).toBe(20);
        wrapper.unmount();
    });
});
