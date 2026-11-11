import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { defineComponent, h } from 'vue';
import ReadConnectionForm from './ReadConnectionForm.vue';

const api = vi.hoisted(() => ({ get: { request: vi.fn() }, test: { request: vi.fn() }, save: { request: vi.fn() } }));
vi.mock('../api', () => ({ octopReadConnectionApi: api }));
vi.mock('@/hooks/useI18n', () => ({ Msg: { success: vi.fn() } }));
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }));

const Slot = defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) });
const FormItem = defineComponent({ props: ['label'], setup: (props, { slots }) => () => h('label', [props.label, slots.default?.()]) });
const Input = defineComponent({
    props: ['modelValue'],
    emits: ['update:modelValue'],
    setup: (props, { emit }) => () => h('input', {
        value: props.modelValue,
        onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value),
    }),
});
const Tabs = defineComponent({
    emits: ['update:modelValue'],
    setup: (_, { emit }) => () => h('button', { 'data-langfuse': '', onClick: () => emit('update:modelValue', 'langfuse') }, 'Langfuse'),
});
const Button = defineComponent({
    props: { loading: Boolean },
    setup: (props, { slots }) => () => h('button', { 'data-loading': String(props.loading) }, slots.default?.()),
});
let wrapper: VueWrapper;
const openForm = async () => {
    wrapper = mount(ReadConnectionForm, {
        props: { instanceId: 1 },
        global: {
            mocks: { $t: (key: string) => key },
            directives: { auth: {}, loading: {} },
            stubs: {
                ElDivider: Slot, ElTabs: Tabs, ElTabPane: Slot, ElForm: Slot, ElFormItem: FormItem, ElInput: Input,
                ElButton: Button, ElTag: Slot, ElAlert: Slot, ElSwitch: Slot,
            },
        },
    });
    await flushPromises();
    await wrapper.get('[data-langfuse]').trigger('click');
    await flushPromises();
    return wrapper;
};
const click = async (text: string) => {
    const button = wrapper.findAll('button').find((candidate) => candidate.text() === text)!;
    await button.trigger('click');
    await flushPromises();
};
beforeEach(() => {
    vi.resetAllMocks();
    api.get.request.mockResolvedValue([{ type: 'langfuse', baseUrl: 'https://langfuse.example', projectId: 'configured-project', publicKey: 'pk-test' }]);
});
afterEach(() => wrapper?.unmount());

describe('Langfuse read connection verification', () => {
    it('displays version, verified project and four check results without changing or saving the configured project', async () => {
        api.test.request.mockResolvedValue({
            ok: true, state: 'ready', version: '4.70.1', apiMode: 'v4', projectId: 'verified-project', projectName: 'Verified project',
            checks: [
                { name: 'health', state: 'ready' }, { name: 'auth', state: 'ready' },
                { name: 'project', state: 'ready' }, { name: 'read', state: 'ready' },
            ],
        });
        await openForm();
        await click('octop.testRead');
        expect(wrapper.text()).toContain('4.70.1');
        expect(wrapper.text()).toContain('Verified project');
        expect(wrapper.text()).toContain('verified-project');
        for (const name of ['health', 'auth', 'project', 'read']) {
            expect(wrapper.get(`[data-read-check="${name}"]`).text()).toContain('octop.stateReady');
        }
        expect((wrapper.findAll('input')[1].element as HTMLInputElement).value).toBe('configured-project');
        expect(api.save.request).not.toHaveBeenCalled();
    });

    it('keeps a project mismatch failed while displaying the actual project so the user can correct the configuration', async () => {
        api.test.request.mockResolvedValue({
            ok: false, state: 'failed', projectId: 'actual-project', projectName: 'Actual project',
            checks: [{ name: 'health', state: 'ready' }, { name: 'auth', state: 'ready' },
                { name: 'project', state: 'failed', message: 'Project mismatch' }, { name: 'read', state: 'not_verified' }],
        });
        await openForm();
        await click('octop.testRead');
        expect(wrapper.get('[data-read-check="project"]').text()).toContain('octop.stateFailed');
        expect(wrapper.get('[data-read-check="project"]').text()).toContain('Project mismatch');
        expect(wrapper.text()).toContain('Actual project');
        expect((wrapper.findAll('input')[1].element as HTMLInputElement).value).toBe('configured-project');
        expect(api.save.request).not.toHaveBeenCalled();
    });

    it('discards an old instance test result and its finally while a new instance is testing', async () => {
        let resolveOld!: (value: { state: string; version: string }) => void;
        let resolveNew!: (value: { state: string; version: string }) => void;
        api.test.request
            .mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve; }))
            .mockReturnValueOnce(new Promise((resolve) => { resolveNew = resolve; }));
        await openForm();
        await click('octop.testRead');
        await wrapper.setProps({ instanceId: 2 });
        await flushPromises();
        await click('octop.testRead');
        resolveOld({ state: 'ready', version: 'old-version' });
        await flushPromises();
        expect(wrapper.text()).not.toContain('old-version');
        expect(wrapper.findAll('button').find((button) => button.text() === 'octop.testRead')!.attributes('data-loading')).toBe('true');
        resolveNew({ state: 'ready', version: 'new-version' });
        await flushPromises();
        expect(wrapper.text()).toContain('new-version');
    });
});
