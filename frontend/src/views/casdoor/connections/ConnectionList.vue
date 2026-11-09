<template>
    <div class="p-4">
        <div class="flex flex-wrap justify-between gap-3 mb-4"><h2>{{ $t('casdoor.serviceConnections') }}</h2><el-button v-if="canConfigure" v-auth="'casdoor:connection:manage'" type="primary" @click="create()">{{ $t('casdoor.newService') }}</el-button></div>
        <el-alert :title="$t('casdoor.readOnlyDirectory')" type="info" :closable="false" class="mb-4" />
        <el-table v-loading="loading" :data="services" row-key="id">
            <el-table-column prop="name" :label="$t('common.name')" />
            <el-table-column prop="serverUrl" :label="$t('casdoor.server')" min-width="220" show-overflow-tooltip />
            <el-table-column prop="clientId" :label="$t('casdoor.managementIdentity')" show-overflow-tooltip />
            <el-table-column prop="organizationCount" :label="$t('casdoor.organizationCount')" />
            <el-table-column :label="$t('casdoor.status')"><template #default="{ row }">{{ $t(row.status === 'verified' ? 'casdoor.identityVerified' : 'casdoor.verificationFailed') }}</template></el-table-column>
            <el-table-column :label="$t('casdoor.identityVerifiedAt')"><template #default="{ row }">{{ formatTime(row.verifiedAt) }}</template></el-table-column>
            <el-table-column :label="$t('common.operation')" min-width="240"><template #default="{ row }">
                <el-button v-if="canConfigure" v-auth="'casdoor:connection:manage'" link type="primary" @click="edit(row)">{{ $t('common.edit') }}</el-button>
                <el-button v-auth="'casdoor:connection:manage'" link type="primary" :loading="discovering === row.id" @click="discover(row)">{{ $t('casdoor.discover') }}</el-button>
                <el-button link @click="viewOrganizations(row)">{{ $t('casdoor.viewOrganizations') }}</el-button>
            </template></el-table-column>
        </el-table>
        <div v-if="legacy.length" class="mt-6">
            <h3>{{ $t('casdoor.legacyConfigurations') }}</h3><el-alert :title="$t('casdoor.legacyHelp')" type="warning" :closable="false" class="my-3" />
            <el-table :data="legacy" row-key="id"><el-table-column prop="name" :label="$t('common.name')" /><el-table-column prop="serverUrl" :label="$t('casdoor.server')" /><el-table-column prop="organization" :label="$t('casdoor.organization')" /><el-table-column v-if="canConfigure" :label="$t('common.operation')"><template #default="{ row }"><el-button v-auth="'casdoor:connection:manage'" link type="primary" @click="create(row)">{{ $t('casdoor.migrateLegacy') }}</el-button></template></el-table-column></el-table>
        </div>
        <el-dialog v-model="visible" :title="$t(form.id ? 'casdoor.editService' : 'casdoor.newService')" width="660px" @closed="clearSecret">
            <el-alert :title="$t('casdoor.managementHelp')" type="info" :closable="false" class="mb-3" />
            <el-form label-position="top" :disabled="working">
                <el-form-item :label="$t('common.name')" required><el-input v-model="form.name" maxlength="64" /></el-form-item>
                <el-form-item :label="$t('casdoor.server')" required><el-input v-model="form.serverUrl" :disabled="!!form.id" maxlength="255" /></el-form-item>
                <el-form-item :label="$t('casdoor.managementClientId')" required><el-input v-model="form.clientId" maxlength="128" /></el-form-item>
                <el-form-item :label="$t('casdoor.managementClientSecret')" :required="!form.id"><el-input v-model="form.clientSecret" type="password" show-password :placeholder="form.id ? $t('casdoor.savedSecret') : ''" /></el-form-item>
                <el-form-item :label="$t('casdoor.timeout')"><el-input-number v-model="form.timeoutSec" :min="1" :max="120" /></el-form-item>
                <el-form-item :label="$t('casdoor.interval')"><el-input-number v-model="form.syncIntervalSec" :min="60" /></el-form-item>
                <el-form-item :label="$t('casdoor.tls')"><el-switch v-model="tls" /></el-form-item>
                <el-form-item :label="$t('casdoor.migrationSource')"><span>{{ migrationName || $t('casdoor.noMigration') }}</span></el-form-item>
            </el-form>
            <div v-if="testedOrganizations"><strong>{{ $t('casdoor.identityVerified') }}</strong><ul><li v-for="org in testedOrganizations" :key="org.name">{{ org.displayName }} ({{ org.name }})</li></ul></div>
            <template #footer><el-button @click="visible = false">{{ $t('common.cancel') }}</el-button><el-button :loading="working" @click="test">{{ $t('casdoor.test') }}</el-button><el-button :loading="working" type="primary" @click="save">{{ $t('common.save') }}</el-button></template>
        </el-dialog>
        <el-dialog v-model="organizationsVisible" :title="$t('casdoor.viewOrganizations')" width="620px"><el-table :data="organizations"><el-table-column prop="name" :label="$t('common.name')" /><el-table-column prop="displayName" :label="$t('casdoor.displayName')" /></el-table></el-dialog>
    </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { casdoorApi, type Connection } from '../api';
import { serviceApi, type Service, type ServiceForm, type Organization, type DiscoveredOrganization } from '../service-api';
import { formatTime } from '../time';
const services = ref<Service[]>([]); const legacy = ref<Connection[]>([]); const canConfigure = ref(false);
const loading = ref(false); const working = ref(false); const visible = ref(false); const discovering = ref(0);
const organizationsVisible = ref(false); const organizations = ref<Organization[]>([]); const testedOrganizations = ref<DiscoveredOrganization[]>(); const migrationName = ref('');
const blank = (): ServiceForm => ({ name: '', serverUrl: '', clientId: '', clientSecret: '', timeoutSec: 15, syncIntervalSec: 3600, tlsVerify: 1 });
const form = ref<ServiceForm>(blank());
const tls = computed({ get: () => form.value.tlsVerify !== -1, set: (value: boolean) => { form.value.tlsVerify = value ? 1 : -1; } });
watch(form, () => { testedOrganizations.value = undefined; }, { deep: true });
function clearSecret() { form.value.clientSecret = ''; testedOrganizations.value = undefined; }
function create(source?: Connection) { form.value = blank(); migrationName.value = source?.name || ''; if (source) { form.value.serverUrl = source.serverUrl; form.value.name = `Casdoor-${new URL(source.serverUrl).hostname}`; form.value.legacyConnectionId = source.id; } visible.value = true; }
function edit(service: Service) { form.value = { id: service.id, name: service.name, serverUrl: service.serverUrl, clientId: service.clientId, clientSecret: '', timeoutSec: service.timeoutSec, syncIntervalSec: service.syncIntervalSec, tlsVerify: service.tlsVerify }; migrationName.value = ''; visible.value = true; }
async function load() { loading.value = true; try { const [list, previous, capabilities] = await Promise.all([serviceApi.list.request(), casdoorApi.connections.request(), serviceApi.capabilities.request()]); services.value = list; legacy.value = previous; canConfigure.value = capabilities.canConfigure; } finally { loading.value = false; } }
async function test() { working.value = true; try { testedOrganizations.value = await serviceApi.test.request({ ...form.value }); } finally { working.value = false; } }
async function save() { working.value = true; try { await serviceApi.save.request({ ...form.value }); visible.value = false; clearSecret(); await load(); } finally { working.value = false; } }
async function discover(service: Service) { discovering.value = service.id; try { organizations.value = await serviceApi.discover.request({ serviceId: service.id }); organizationsVisible.value = true; await load(); } finally { discovering.value = 0; } }
async function viewOrganizations(service: Service) { organizations.value = await serviceApi.organizations.request({ serviceId: service.id }); organizationsVisible.value = true; }
onMounted(load);
</script>
