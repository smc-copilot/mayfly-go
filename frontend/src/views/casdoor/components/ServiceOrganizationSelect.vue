<template>
    <div class="mb-4">
        <div class="flex flex-wrap gap-3 items-center">
            <el-select :model-value="selection.serviceId || undefined" :placeholder="$t('casdoor.chooseService')" style="width: 240px" @change="selectService">
                <el-option v-for="service in services" :key="service.id" :label="service.name" :value="service.id" />
            </el-select>
            <el-select :model-value="selection.organizationId || undefined" :loading="loading" :disabled="!selection.serviceId" :placeholder="$t('casdoor.chooseOrganization')" style="width: 240px" @change="selectOrganization">
                <el-option v-for="org in organizations" :key="org.id" :label="org.displayName ? `${org.displayName} (${org.name})` : org.name" :value="org.id" />
            </el-select>
            <el-button v-auth="'casdoor:sync'" :disabled="!selection.organizationId" :loading="syncing" @click="sync">{{ $t('casdoor.sync') }}</el-button>
        </div>
        <el-alert v-if="!services.length && !loading" class="mt-3" :title="$t('casdoor.noService')" type="info" :closable="false" />
        <el-alert v-if="failure" class="mt-3" :title="$t('casdoor.contextFailed')" type="error" :closable="false" />
        <el-alert v-if="currentService?.status === 'failed'" class="mt-3" :title="$t('casdoor.serviceNeedsVerification')" type="warning" :closable="false" />
        <el-collapse v-if="selection.organizationId" class="mt-3">
            <el-collapse-item :title="$t('casdoor.persistedSyncStatus')" name="status">
                <el-table :data="persistedRows">
                    <el-table-column prop="category" :label="$t('casdoor.category')" />
                    <el-table-column prop="status" :label="$t('casdoor.status')" />
                    <el-table-column prop="attemptAt" :label="$t('casdoor.lastAttempt')" />
                    <el-table-column prop="successAt" :label="$t('casdoor.lastSuccessfulSync')" />
                    <el-table-column prop="count" :label="$t('casdoor.snapshotCount')" />
                    <el-table-column prop="reason" :label="$t('casdoor.reason')" />
                </el-table>
            </el-collapse-item>
        </el-collapse>
        <el-dialog v-model="resultVisible" :title="$t('casdoor.syncDetails')" width="560px">
            <el-table :data="resultRows"><el-table-column prop="category" :label="$t('casdoor.category')" /><el-table-column prop="status" :label="$t('casdoor.status')" /><el-table-column prop="count" :label="$t('casdoor.recordCount')" /><el-table-column prop="reason" :label="$t('casdoor.reason')" /></el-table>
        </el-dialog>
    </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { serviceApi, type Service, type Organization, type ServiceContext, type CategoryResult } from '../service-api';
import { useCasdoorContext } from '../context';
import { formatTime as formatCasdoorTime } from '../time';
const emit = defineEmits<{ change: [value: ServiceContext] }>();
const { t } = useI18n();
const selection = useCasdoorContext();
const services = ref<Service[]>([]);
const organizations = ref<Organization[]>([]);
const loading = ref(true);
const failure = ref(false);
const syncing = ref(false);
const resultVisible = ref(false);
const results = ref<Record<string, CategoryResult>>({});
const resultRows = computed(() => Object.entries(results.value).map(([key, value]) => ({ category: t(`casdoor.category_${key}`), status: t(`casdoor.sync_${value.status}`), count: value.status === 'success' ? value.count : '—', reason: value.reasonCode ? t('casdoor.categoryFailed') : '—' })));
const currentService = computed(() => services.value.find((item) => item.id === selection.serviceId));
const persistedRows = computed(() => Object.entries(organizations.value.find((item) => item.id === selection.organizationId)?.syncStates || {}).map(([key, value]) => ({ category: t(`casdoor.category_${key}`), status: t(`casdoor.sync_${value.status || 'never'}`), attemptAt: formatCasdoorTime(value.attemptAt), successAt: formatCasdoorTime(value.successAt), count: value.successAt ? value.count : '—', reason: value.reasonCode ? t('casdoor.categoryFailed') : '—' })));
let generation = 0;
let disposed = false;
function publish() { emit('change', { serviceId: selection.serviceId, organizationId: selection.organizationId }); }
async function selectService(id: number, restore = 0) {
    const request = ++generation;
    resultVisible.value = false;
    selection.serviceId = id; selection.organizationId = 0; organizations.value = []; failure.value = false; publish();
    loading.value = true;
    try {
        const rows = await serviceApi.organizations.request({ serviceId: id });
        if (request !== generation) return;
        organizations.value = rows;
        selection.organizationId = restore ? (rows.some((row) => row.id === restore) ? restore : 0) : rows[0]?.id || 0;
        publish();
    } catch { if (request === generation) failure.value = true; }
    finally { if (request === generation) loading.value = false; }
}
function selectOrganization(id: number) { resultVisible.value = false; selection.organizationId = organizations.value.some((org) => org.id === id) ? id : 0; publish(); }
async function sync() {
    const context = { serviceId: selection.serviceId, organizationId: selection.organizationId };
    const request = generation;
    syncing.value = true;
    try {
        const result = await serviceApi.sync.request(context);
        if (request !== generation || context.organizationId !== selection.organizationId) return;
        results.value = result; resultVisible.value = true; publish();
    } finally {
        syncing.value = false;
        if (request === generation && !disposed) {
            try {
                const rows = await serviceApi.organizations.request({ serviceId: context.serviceId });
                if (request === generation && !disposed) organizations.value = rows;
            } catch { if (request === generation && !disposed) failure.value = true; }
        }
    }
}
onMounted(async () => {
    try {
        const rows = await serviceApi.list.request();
        if (disposed) return;
        services.value = rows;
        const id = selection.serviceId || services.value[0]?.id;
        if (id && services.value.some((service) => service.id === id)) await selectService(id, selection.organizationId);
        else { selection.serviceId = 0; selection.organizationId = 0; publish(); }
    } catch { if (!disposed) failure.value = true; }
    finally { if (!disposed) loading.value = false; }
});
onBeforeUnmount(() => { disposed = true; generation++; });
</script>
