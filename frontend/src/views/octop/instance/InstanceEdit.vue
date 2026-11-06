<template>
    <el-dialog v-model="visibleProxy" :title="$t(data ? 'octop.editInstance' : 'octop.registerInstance')" width="680px" destroy-on-close>
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.savePending')" />
        <el-form :model="instanceForm" label-width="140px">
            <el-form-item :label="$t('octop.name')" required>
                <el-input v-model="instanceForm.name" />
            </el-form-item>
            <el-form-item :label="$t('octop.center')">
                <el-input v-model="instanceForm.center" />
            </el-form-item>
            <el-form-item :label="$t('octop.machine')" required>
                <el-select v-model="instanceForm.machineId" filterable class="w-full">
                    <el-option v-for="m in machines" :key="m.id" :label="`${m.name} (${m.ip})`" :value="m.id" />
                </el-select>
            </el-form-item>
            <el-form-item :label="$t('octop.apiUrl')">
                <el-input v-model="instanceForm.apiUrl" placeholder="http://10.x.x.x:port" />
            </el-form-item>
            <el-form-item :label="$t('octop.callbackUrl')">
                <el-input v-model="instanceForm.callbackUrl" placeholder="https://octop.example.com/api/auth/oidc/callback" />
                <div class="text-xs text-gray-500 mt-1">{{ $t('octop.callbackUrlHint') }}</div>
            </el-form-item>
            <el-form-item :label="$t('octop.apiToken')">
                <el-input v-model="tokenInput" type="password" show-password autocomplete="new-password" :placeholder="$t('octop.apiTokenKeepHint')" />
                <div class="text-xs text-gray-500 mt-1">
                    {{ $t('octop.apiTokenHint') }}
                    <span v-if="instanceForm.apiTokenConfigured === 1"> · {{ $t('octop.secretConfigured') }}</span>
                    <span v-else> · {{ $t('octop.secretEmpty') }}</span>
                </div>
            </el-form-item>
            <el-form-item :label="$t('octop.startMode')">
                <el-select v-model="instanceForm.startMode" class="w-full">
                    <el-option :label="$t('octop.systemd')" value="systemd" />
                    <el-option :label="$t('octop.cli')" value="cli" />
                </el-select>
            </el-form-item>
            <el-form-item v-if="instanceForm.startMode === 'systemd'" :label="$t('octop.systemdUnit')">
                <el-input v-model="instanceForm.systemdUnit" />
            </el-form-item>
            <el-form-item v-else :label="$t('octop.processMatch')">
                <el-input v-model="instanceForm.processMatch" />
            </el-form-item>
            <el-form-item :label="$t('octop.db')">
                <el-select v-model="instanceForm.dbId" filterable clearable class="w-full">
                    <el-option v-for="d in dbs" :key="d.id" :label="`${d.name || d.code} (#${d.id})`" :value="d.id" />
                </el-select>
            </el-form-item>
            <el-form-item :label="$t('octop.dbName')">
                <el-input v-model="instanceForm.dbName" />
            </el-form-item>
            <el-form-item :label="$t('octop.collectEnabled')">
                <el-switch v-model="collectOn" />
            </el-form-item>
            <el-form-item :label="$t('octop.maintenance')">
                <el-switch v-model="maintOn" />
            </el-form-item>
            <el-divider>{{ $t('octop.thresholds') }}</el-divider>
            <el-form-item :label="$t('octop.sampleInterval')">
                <el-input-number v-model="instanceForm.sampleIntervalSec" :min="30" :max="3600" />
            </el-form-item>
            <el-form-item :label="$t('octop.cpuWarn')">
                <el-input-number v-model="instanceForm.cpuWarn" :min="1" :max="100" />
            </el-form-item>
            <el-form-item :label="$t('octop.cpuCrit')">
                <el-input-number v-model="instanceForm.cpuCrit" :min="1" :max="100" />
            </el-form-item>
            <el-form-item :label="$t('octop.memWarn')">
                <el-input-number v-model="instanceForm.memWarn" :min="1" :max="100" />
            </el-form-item>
            <el-form-item :label="$t('octop.memCrit')">
                <el-input-number v-model="instanceForm.memCrit" :min="1" :max="100" />
            </el-form-item>
            <el-form-item :label="$t('octop.diskWarn')">
                <el-input-number v-model="instanceForm.diskWarn" :min="1" :max="100" />
            </el-form-item>
            <el-form-item :label="$t('octop.diskCrit')">
                <el-input-number v-model="instanceForm.diskCrit" :min="1" :max="100" />
            </el-form-item>
            <el-form-item :label="$t('octop.httpFailThreshold')">
                <el-input-number v-model="instanceForm.httpFailThreshold" :min="1" :max="20" />
            </el-form-item>
            <el-form-item :label="$t('octop.remark')">
                <el-input v-model="instanceForm.remark" type="textarea" :rows="2" />
            </el-form-item>
        </el-form>
        <div v-if="testResult" class="text-sm mb-3">
            <div>{{ $t('octop.ssh') }}: {{ testResult.ssh.status }} {{ testResult.ssh.reason }}</div>
            <div>{{ $t('octop.httpHealth') }}: {{ testResult.http.status }} {{ testResult.http.reason }}</div>
            <div>{{ $t('octop.pg') }}: {{ testResult.pg.status }} {{ testResult.pg.reason }}</div>
            <div>{{ testResult.saved === 'pending_fix' ? $t('octop.pendingFix') : $t('octop.normal') }}</div>
        </div>
        <template #footer>
            <el-button @click="visibleProxy = false">{{ $t('common.cancel') }}</el-button>
            <el-button @click="onTest">{{ $t('octop.testConn') }}</el-button>
            <el-button type="primary" @click="onSave">{{ $t('common.save') }}</el-button>
        </template>
    </el-dialog>
</template>

<script lang="ts" setup>
import { computed, reactive, ref, watch } from 'vue';
import { Msg } from '@/hooks/useI18n';
import { machineApi } from '@/views/ops/machine/api';
import { dbApi } from '@/views/ops/db/api';
import { octopInstanceApi } from '../api';
import type { ConnTestResult, OctopInstance } from '../types';

type InstanceFormState = {
    id?: number;
    name: string;
    center: string;
    remark: string;
    machineId?: number;
    apiUrl: string;
    callbackUrl: string;
    apiTokenConfigured: number;
    startMode: string;
    systemdUnit: string;
    processMatch: string;
    dbId?: number;
    dbName: string;
    collectEnabled: number;
    maintenance: number;
    sampleIntervalSec: number;
    cpuWarn: number;
    cpuCrit: number;
    memWarn: number;
    memCrit: number;
    diskWarn: number;
    diskCrit: number;
    httpFailThreshold: number;
};

const emptyForm = (): InstanceFormState => ({
    name: '',
    center: '',
    remark: '',
    apiUrl: '',
    callbackUrl: '',
    apiTokenConfigured: -1,
    startMode: 'cli',
    systemdUnit: '',
    processMatch: '',
    dbName: '',
    collectEnabled: 1,
    maintenance: -1,
    sampleIntervalSec: 120,
    cpuWarn: 85,
    cpuCrit: 95,
    memWarn: 85,
    memCrit: 95,
    diskWarn: 85,
    diskCrit: 95,
    httpFailThreshold: 3,
});

const props = defineProps<{ visible: boolean; data?: OctopInstance | null }>();
const emit = defineEmits<{ 'update:visible': [boolean]; success: [] }>();

const visibleProxy = computed({
    get: () => props.visible,
    set: (v) => emit('update:visible', v),
});

const instanceForm = reactive<InstanceFormState>(emptyForm());
const collectOn = computed({
    get: () => instanceForm.collectEnabled === 1,
    set: (v) => {
        instanceForm.collectEnabled = v ? 1 : -1;
    },
});
const maintOn = computed({
    get: () => instanceForm.maintenance === 1,
    set: (v) => {
        instanceForm.maintenance = v ? 1 : -1;
    },
});
const machines = ref<{ id: number; name: string; ip: string }[]>([]);
const dbs = ref<{ id: number; name?: string; code?: string }[]>([]);
const testResult = ref<ConnTestResult | null>(null);
const tokenInput = ref('');

watch(
    () => props.visible,
    async (open) => {
        if (!open) return;
        testResult.value = null;
        tokenInput.value = '';
        Object.assign(instanceForm, emptyForm());
        if (props.data) Object.assign(instanceForm, props.data);
        const ms = await machineApi.list.request({ pageNum: 1, pageSize: 200 });
        machines.value = (ms?.list || []).map((m: any) => ({ id: m.id, name: m.name, ip: m.ip }));
        const ds = await dbApi.dbs.request({ pageNum: 1, pageSize: 200 });
        dbs.value = (ds?.list || []).map((d: any) => ({ id: d.id, name: d.name, code: d.code }));
    }
);

const onTest = async () => {
    testResult.value = await octopInstanceApi.testConn.request(instanceForm);
};
const onSave = async () => {
    await octopInstanceApi.save.request({ ...instanceForm, apiToken: tokenInput.value });
    Msg.success('common.success');
    emit('success');
};
</script>
