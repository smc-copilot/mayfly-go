<template>
    <div class="p-4">
        <h2 class="mb-4">{{ $t(kind === 'applications' ? 'casdoor.applications' : 'casdoor.userDirectory') }}</h2>
        <div class="flex flex-wrap gap-3 mb-4">
            <ServiceOrganizationSelect class="w-full" @change="changeContext" />
            <el-input v-model="query.keyword" :placeholder="$t('casdoor.keyword')" style="width: 240px" @keyup.enter="search" />
            <el-button @click="search">{{ $t('casdoor.search') }}</el-button>
        </div>
        <el-table v-loading="loading" :data="rows" row-key="id">
            <el-table-column v-if="kind === 'applications'" prop="name" :label="$t('common.name')" />
            <el-table-column v-else prop="username" :label="$t('casdoor.username')" />
            <el-table-column prop="displayName" :label="$t('casdoor.displayName')" />
            <el-table-column v-if="kind === 'applications'" prop="appOwner" :label="$t('casdoor.nativeOwner')" />
            <el-table-column v-else prop="email" :label="$t('casdoor.email')" />
            <el-table-column prop="organization" :label="$t('casdoor.organization')" />
            <el-table-column :label="$t('casdoor.status')">
                <template #default="{ row }">{{ $t(row.enabled === 1 ? 'casdoor.enabled' : row.enabled === -1 ? 'casdoor.disabled' : 'casdoor.unknown') }}</template>
            </el-table-column>
            <el-table-column :label="$t('casdoor.presence')">
                <template #default="{ row }">{{ $t(row.present === 1 ? 'casdoor.present' : row.present === -1 ? 'casdoor.absent' : 'casdoor.unknown') }}</template>
            </el-table-column>
            <el-table-column :label="$t('casdoor.lastSync')"><template #default="{ row }">{{ formatTime(row.lastSyncAt) }}</template></el-table-column>
        </el-table>
        <el-pagination class="mt-3" :current-page="query.pageNum" :page-size="query.pageSize" :total="total" layout="total, prev, pager, next" @current-change="changePage" />
    </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue';
import { type Application, type User } from '../api';
import { serviceApi, type ServiceContext } from '../service-api';
import ServiceOrganizationSelect from './ServiceOrganizationSelect.vue';
import { formatTime } from '../time';
const props = defineProps<{ kind: 'applications' | 'users' }>();
const query = reactive({ serviceId: 0, organizationId: 0, keyword: '', pageNum: 1, pageSize: 10 });
const rows = ref<(Application | User)[]>([]);
const total = ref(0);
const loading = ref(false);
let generation = 0;
async function load() {
    const request = ++generation;
    if (!query.serviceId || !query.organizationId) { rows.value = []; total.value = 0; loading.value = false; return; }
    loading.value = true;
    try {
        const params = { ...query };
        const result = props.kind === 'applications' ? await serviceApi.applications.request(params) : await serviceApi.users.request(params);
        if (request === generation) { rows.value = result.list; total.value = result.total; }
    } finally { if (request === generation) loading.value = false; }
}
function changeContext(context: ServiceContext) { Object.assign(query, context); rows.value = []; total.value = 0; query.pageNum = 1; void load(); }
function search() { query.pageNum = 1; void load(); }
function changePage(page: number) { query.pageNum = page; void load(); }
onBeforeUnmount(() => { generation++; });
</script>
