<template>
    <div v-auth="'octop:integration:save'" class="mt-4" v-loading="loading">
        <el-divider>{{ $t('octop.readConnection') }}</el-divider>
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.readConnectionHint')" />

        <el-tabs v-model="activeType">
            <el-tab-pane :label="$t('octop.sourceNameOctopApi')" name="octop_api" />
            <el-tab-pane :label="$t('octop.sourceNameLangfuse')" name="langfuse" />
        </el-tabs>

        <el-form :model="form" label-width="140px" class="mt-2">
            <el-form-item :label="$t('octop.readBaseUrl')" required>
                <el-input v-model="form.baseUrl" :placeholder="$t('octop.readBaseUrlPlaceholder')" />
            </el-form-item>
            <el-form-item v-if="activeType === 'langfuse'" :label="$t('octop.langfuseProjectId')">
                <el-input v-model="form.projectId" />
            </el-form-item>
            <el-form-item :label="activeType === 'langfuse' ? $t('octop.langfusePublicKey') : $t('octop.readPublicKey')">
                <el-input v-model="form.publicKey" autocomplete="off" />
            </el-form-item>
            <el-form-item :label="$t('octop.readSecret')">
                <el-input v-model="form.secret" type="password" show-password autocomplete="new-password" :placeholder="secretPlaceholder" />
                <div class="text-xs text-gray-500 mt-1">{{ $t('octop.secretKeepHint') }}</div>
            </el-form-item>
            <el-form-item v-if="view?.credentialConfigured" :label="$t('octop.clearSecret')">
                <el-switch v-model="form.clearSecret" />
            </el-form-item>
            <el-form-item :label="$t('octop.verifyState')">
                <enum-tag v-if="view?.verifyState" :enums="OctopSourceStateEnum" :value="view.verifyState" />
                <span v-else class="text-gray-500">{{ $t('octop.secretEmpty') }}</span>
                <span v-if="view?.verifiedAt" class="ml-2 text-xs text-gray-500">{{ formatShanghai(view.verifiedAt) }}</span>
                <div v-if="view?.verifyError" class="text-xs text-red-500 mt-1 break-all">{{ view.verifyError }}</div>
            </el-form-item>
            <el-form-item v-if="testResult" :label="$t('octop.testResult')">
                <div class="space-y-2 w-full">
                    <div>
                        <enum-tag :enums="OctopSourceStateEnum" :value="testResult.state" />
                        <span v-if="testResult.message" class="ml-2 text-sm">{{ testResult.message }}</span>
                    </div>
                    <div v-if="testResult.version" class="text-sm">{{ $t('octop.langfuseVersion') }}: {{ testResult.version }}</div>
                    <div v-if="testResult.apiMode" class="text-sm">{{ $t('octop.langfuseApiMode') }}: {{ testResult.apiMode }}</div>
                    <div v-if="testResult.projectId" class="text-sm">{{ $t('octop.verifiedProjectId') }}: {{ testResult.projectId }}</div>
                    <div v-if="testResult.projectName" class="text-sm">{{ $t('octop.langfuseProjectName') }}: {{ testResult.projectName }}</div>
                    <div v-for="check in testResult.checks" :key="check.name" :data-read-check="check.name" class="flex flex-wrap items-center gap-2 text-sm">
                        <span>{{ checkLabel(check.name) }}</span>
                        <enum-tag :enums="OctopSourceStateEnum" :value="check.state" />
                        <span v-if="check.message" class="text-gray-500 break-all">{{ check.message }}</span>
                    </div>
                </div>
            </el-form-item>
        </el-form>

        <div class="flex gap-2 justify-end">
            <el-button :loading="testing" @click="onTest">{{ $t('octop.testRead') }}</el-button>
            <el-button type="primary" :loading="saving" @click="onSave">{{ $t('octop.saveReadConnection') }}</el-button>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { computed, onScopeDispose, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { Msg } from '@/hooks/useI18n';
import { octopReadConnectionApi } from '../api';
import { OctopSourceStateEnum } from '../enums';
import { formatShanghai } from '../time';
import type { ReadConnectionSaveForm, ReadConnectionTestResult, ReadConnectionType, ReadConnectionView } from '../types';

const props = defineProps<{
    instanceId: number;
}>();

const { t } = useI18n();
const activeType = ref<ReadConnectionType>('octop_api');
const views = ref<ReadConnectionView[]>([]);
const loading = ref(false);
const saving = ref(false);
const testing = ref(false);
const testResult = ref<ReadConnectionTestResult | null>(null);
let contextSequence = 0;
let testSequence = 0;
let saveSequence = 0;
let disposed = false;

const form = reactive<ReadConnectionSaveForm>({
    type: 'octop_api',
    baseUrl: '',
    projectId: '',
    publicKey: '',
    secret: '',
    clearSecret: false,
    revision: 0,
});

const view = computed(() => views.value.find((v) => v.type === activeType.value));

const secretPlaceholder = computed(() =>
    view.value?.credentialConfigured ? t('octop.secretKeepHint') : t('octop.secretEmpty')
);

const checkLabel = (name: string) => {
    const labels: Record<string, string> = {
        health: 'octop.readCheckHealth',
        auth: 'octop.readCheckAuth',
        project: 'octop.readCheckProject',
        read: 'octop.readCheckAccess',
    };
    return t(labels[name] || 'octop.readCheckOther');
};

const applyView = (v?: ReadConnectionView) => {
    form.type = activeType.value;
    form.baseUrl = v?.baseUrl || '';
    form.projectId = v?.projectId || '';
    form.publicKey = v?.publicKey || '';
    form.secret = '';
    form.clearSecret = false;
    form.revision = v?.revision ?? 0;
    testResult.value = null;
};

const load = async () => {
    if (!props.instanceId) return;
    const context = contextSequence;
    const id = props.instanceId;
    const current = () => !disposed && context === contextSequence && id === props.instanceId;
    loading.value = true;
    try {
        const res = await octopReadConnectionApi.get.request({ id });
        if (!current()) return;
        views.value = Array.isArray(res) ? res : res ? [res] : [];
        applyView(view.value);
    } catch {
        if (current()) views.value = [];
    } finally {
        if (current()) loading.value = false;
    }
};

const payload = (): ReadConnectionSaveForm & { id: number } => ({
    id: props.instanceId,
    type: activeType.value,
    baseUrl: form.baseUrl,
    projectId: activeType.value === 'langfuse' ? form.projectId : undefined,
    publicKey: form.publicKey,
    secret: form.clearSecret ? '' : form.secret,
    clearSecret: form.clearSecret,
    revision: form.revision,
});

const onTest = async () => {
    const context = contextSequence;
    const sequence = ++testSequence;
    const current = () => !disposed && context === contextSequence && sequence === testSequence;
    testing.value = true;
    testResult.value = null;
    try {
        const result = await octopReadConnectionApi.test.request(payload());
        if (current()) testResult.value = result;
    } catch {
        if (current()) testResult.value = { ok: false, state: 'failed', message: t('octop.readConnectionTestFailed') };
    } finally {
        if (current()) testing.value = false;
    }
};

const onSave = async () => {
    const context = contextSequence;
    const sequence = ++saveSequence;
    const current = () => !disposed && context === contextSequence && sequence === saveSequence;
    saving.value = true;
    try {
        const saved = await octopReadConnectionApi.save.request(payload());
        if (!current()) return;
        const idx = views.value.findIndex((v) => v.type === saved.type);
        if (idx >= 0) views.value[idx] = saved;
        else views.value.push(saved);
        applyView(saved);
        Msg.success('common.success');
    } finally {
        if (current()) saving.value = false;
    }
};

const invalidateActions = () => {
    testSequence++;
    saveSequence++;
    testing.value = false;
    saving.value = false;
};
watch(activeType, () => {
    invalidateActions();
    applyView(view.value);
}, { flush: 'sync' });

watch(
    () => props.instanceId,
    () => {
        contextSequence++;
        invalidateActions();
        views.value = [];
        applyView();
        loading.value = false;
        void load();
    },
    { immediate: true, flush: 'sync' }
);
onScopeDispose(() => {
    disposed = true;
    contextSequence++;
    invalidateActions();
    views.value = [];
    applyView();
});
</script>
