<template>
    <div class="p-4">
        <div class="flex flex-wrap items-center justify-between gap-3 mb-4">
            <h2>{{ $t('casdoor.connections') }}</h2>
            <el-button v-auth="'casdoor:connection:manage'" type="primary" @click="openCreate">{{ $t('casdoor.create') }}</el-button>
        </div>
        <el-alert :title="$t('casdoor.manageWarning')" type="info" :closable="false" class="mb-4" />
        <el-table v-loading="loading" :data="connections" row-key="id">
            <el-table-column prop="name" :label="$t('common.name')" />
            <el-table-column prop="serverUrl" :label="$t('casdoor.server')" show-overflow-tooltip />
            <el-table-column prop="organization" :label="$t('casdoor.organization')" />
            <el-table-column :label="$t('casdoor.credential')">
                <template #default="{ row }">{{ row.credentialConfigured === 1 ? $t('casdoor.configured') : $t('casdoor.missing') }}</template>
            </el-table-column>
            <el-table-column prop="appSyncStatus" :label="$t('casdoor.appStatus')" />
            <el-table-column prop="userSyncStatus" :label="$t('casdoor.userStatus')" />
            <el-table-column :label="$t('common.operation')" min-width="230">
                <template #default="{ row }">
                    <el-button v-auth="'casdoor:connection:manage'" link type="primary" @click="openEdit(row)">{{ $t('common.edit') }}</el-button>
                    <el-button v-auth="'casdoor:sync'" link type="primary" @click="sync(row)">{{ $t('casdoor.sync') }}</el-button>
                </template>
            </el-table-column>
        </el-table>
        <el-dialog v-model="visible" :title="$t(form.id ? 'casdoor.edit' : 'casdoor.create')" width="620px" @closed="clearSecret">
            <el-form label-position="top">
                <el-form-item :label="$t('common.name')" required><el-input v-model="form.name" maxlength="64" /></el-form-item>
                <el-form-item :label="$t('casdoor.server')" required><el-input v-model="form.serverUrl" :disabled="!!form.id" maxlength="255" /></el-form-item>
                <el-form-item :label="$t('casdoor.organization')" required><el-input v-model="form.organization" :disabled="!!form.id" maxlength="64" /></el-form-item>
                <el-form-item :label="$t('casdoor.clientId')" required><el-input v-model="form.clientId" maxlength="128" /></el-form-item>
                <el-form-item :label="$t('casdoor.clientSecret')" :required="!form.id"><el-input v-model="form.clientSecret" type="password" show-password :placeholder="form.id ? $t('casdoor.savedSecret') : ''" /></el-form-item>
                <el-form-item :label="$t('casdoor.owner')"><el-input v-model="form.appOwner" maxlength="64" /><div>{{ $t('casdoor.scopeWarning') }}</div></el-form-item>
                <el-form-item :label="$t('casdoor.timeout')"><el-input-number v-model="form.timeoutSec" :min="1" :max="120" /></el-form-item>
                <el-form-item :label="$t('casdoor.interval')"><el-input-number v-model="form.syncIntervalSec" :min="60" /></el-form-item>
                <el-form-item :label="$t('casdoor.tls')"><el-switch v-model="tls" /></el-form-item>
            </el-form>
            <div v-if="probe" class="mb-3">
                <strong>{{ $t('casdoor.testResult') }}</strong>
                <div v-for="item in probeRows" :key="item.name">{{ $t(item.name) }}: {{ item.value.status }} {{ item.value.reasonCode }}</div>
            </div>
            <template #footer>
                <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
                <el-button :loading="working" @click="test">{{ $t('casdoor.test') }}</el-button>
                <el-button :loading="working" type="primary" @click="save">{{ $t('common.save') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';
import { casdoorApi, type Connection, type ConnectionForm, type ProbeResult } from '../api';

const { t } = useI18n();
const connections = ref<Connection[]>([]);
const loading = ref(false);
const working = ref(false);
const visible = ref(false);
const probe = ref<ProbeResult>();
const blank = (): ConnectionForm => ({ name: '', serverUrl: '', organization: '', appOwner: '', clientId: '', clientSecret: '', timeoutSec: 15, syncIntervalSec: 3600, tlsVerify: 1 });
const form = ref<ConnectionForm>(blank());
const tls = computed({ get: () => form.value.tlsVerify !== -1, set: (value: boolean) => { form.value.tlsVerify = value ? 1 : -1; } });
const probeRows = computed(() => probe.value ? [
    { name: 'casdoor.service', value: probe.value.service }, { name: 'casdoor.apps', value: probe.value.apps }, { name: 'casdoor.users', value: probe.value.users },
] : []);

async function load() { loading.value = true; try { connections.value = await casdoorApi.connections.request() || []; } finally { loading.value = false; } }
function clearSecret() { form.value.clientSecret = ''; probe.value = undefined; }
function openCreate() { form.value = blank(); probe.value = undefined; visible.value = true; }
function openEdit(row: Connection) { form.value = { ...row, clientSecret: '' }; probe.value = undefined; visible.value = true; }
async function test() { working.value = true; try { probe.value = await casdoorApi.test.request(form.value); } finally { working.value = false; } }
async function save() { working.value = true; try { await casdoorApi.save.request(form.value); visible.value = false; await load(); } finally { working.value = false; } }
async function sync(row: Connection) { await casdoorApi.sync.request({ id: row.id }); ElMessage.info(t('casdoor.syncResult')); await load(); }
onMounted(load);
</script>
