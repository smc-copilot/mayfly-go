<template>
    <div class="p-4">
        <h2 class="mb-4">{{ $t('casdoor.operations') }}</h2>
        <ServiceOrganizationSelect @change="changeContext" />
        <el-table v-loading="loading" :data="rows" row-key="id">
            <el-table-column prop="action" :label="$t('casdoor.action')" />
            <el-table-column prop="actorId" :label="$t('casdoor.actor')" />
            <el-table-column prop="status" :label="$t('casdoor.status')" />
            <el-table-column prop="submittedAt" :label="$t('casdoor.submittedAt')" />
            <el-table-column prop="verifiedAt" :label="$t('casdoor.verifiedAt')" />
        </el-table>
        <el-pagination class="mt-3" :current-page="query.pageNum" :page-size="query.pageSize" :total="total" layout="total, prev, pager, next" @current-change="changePage" />
    </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue';
import { type AuthorizationObject, type Operation } from '../api';
import { serviceApi, type ServiceContext } from '../service-api';
import ServiceOrganizationSelect from '../components/ServiceOrganizationSelect.vue';
const rows = ref<Operation[]>([]);
const total = ref(0);
const loading = ref(false);
const query = reactive({ serviceId: 0, organizationId: 0, keyword: '', kind: 'role', pageNum: 1, pageSize: 10 });
let generation = 0;
async function load() {
    const request = ++generation;
    if (!query.serviceId || !query.organizationId) { rows.value = []; total.value = 0; loading.value = false; return; }
    loading.value = true;
    try { const result = await serviceApi.operations.request({ ...query }); if (request === generation) { rows.value = result.list; total.value = result.total; } }
    finally { if (request === generation) loading.value = false; }
}
function changeContext(context: ServiceContext) { Object.assign(query, context); rows.value = []; total.value = 0; query.pageNum = 1; void load(); }
function search() { query.pageNum = 1; void load(); }
function changePage(page: number) { query.pageNum = page; void load(); }
onBeforeUnmount(() => { generation++; });
</script>
