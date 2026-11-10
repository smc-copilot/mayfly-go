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
                <enum-tag :enums="OctopSourceStateEnum" :value="testResult.state" />
                <span v-if="testResult.message" class="ml-2 text-sm">{{ testResult.message }}</span>
            </el-form-item>
        </el-form>

        <div class="flex gap-2 justify-end">
            <el-button :loading="testing" @click="onTest">{{ $t('octop.testRead') }}</el-button>
            <el-button type="primary" :loading="saving" @click="onSave">{{ $t('octop.saveReadConnection') }}</el-button>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { computed, reactive, ref, watch } from 'vue';
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
    loading.value = true;
    try {
        const res = await octopReadConnectionApi.get.request({ id: props.instanceId });
        views.value = Array.isArray(res) ? res : res ? [res] : [];
        applyView(view.value);
    } finally {
        loading.value = false;
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
    testing.value = true;
    try {
        testResult.value = await octopReadConnectionApi.test.request(payload());
    } finally {
        testing.value = false;
    }
};

const onSave = async () => {
    saving.value = true;
    try {
        const saved = await octopReadConnectionApi.save.request(payload());
        const idx = views.value.findIndex((v) => v.type === saved.type);
        if (idx >= 0) views.value[idx] = saved;
        else views.value.push(saved);
        applyView(saved);
        Msg.success('common.success');
    } finally {
        saving.value = false;
    }
};

watch(activeType, () => applyView(view.value));

watch(
    () => props.instanceId,
    () => {
        void load();
    },
    { immediate: true }
);
</script>
