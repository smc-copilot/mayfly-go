<template>
    <div class="p-4">
        <h2 class="mb-4">{{ $t(kind === 'applications' ? 'casdoor.applications' : 'casdoor.userDirectory') }}</h2>
        <div class="flex flex-wrap gap-3 mb-4">
            <el-select v-model="query.connectionId" :placeholder="$t('casdoor.chooseConnection')" @change="search">
                <el-option v-for="item in connections" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
            <el-input v-model="query.keyword" :placeholder="$t('casdoor.keyword')" style="width: 240px" @keyup.enter="search" />
            <el-button @click="search">{{ $t('casdoor.search') }}</el-button>
        </div>
        <el-alert v-if="selected?.appOwner && kind === 'applications'" :title="$t('casdoor.scopeWarning')" type="warning" :closable="false" class="mb-4" />
        <el-alert v-if="isStale" :title="$t('casdoor.staleNotice')" type="warning" :closable="false" class="mb-4" />
        <el-table v-loading="loading" :data="rows" row-key="id">
            <el-table-column v-if="kind === 'applications'" prop="name" :label="$t('common.name')" />
            <el-table-column v-else prop="username" :label="$t('casdoor.username')" />
            <el-table-column prop="displayName" :label="$t('casdoor.displayName')" />
            <el-table-column v-if="kind === 'applications'" prop="appOwner" :label="$t('casdoor.owner')" />
            <el-table-column v-else prop="email" :label="$t('casdoor.email')" />
            <el-table-column prop="organization" :label="$t('casdoor.organization')" />
            <el-table-column :label="$t('casdoor.status')">
                <template #default="{ row }">{{ $t(row.enabled === 1 ? 'casdoor.enabled' : row.enabled === -1 ? 'casdoor.disabled' : 'casdoor.unknown') }}</template>
            </el-table-column>
            <el-table-column :label="$t('casdoor.presence')">
                <template #default="{ row }">{{ $t(row.present === 1 ? 'casdoor.present' : row.present === -1 ? 'casdoor.absent' : 'casdoor.unknown') }}</template>
            </el-table-column>
            <el-table-column prop="lastSyncAt" :label="$t('casdoor.lastSync')" />
        </el-table>
        <el-pagination class="mt-3" :current-page="query.pageNum" :page-size="query.pageSize" :total="total" layout="total, prev, pager, next" @current-change="changePage" />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { casdoorApi, type Application, type Connection, type User } from '../api';

const props = defineProps<{ kind: 'applications' | 'users' }>();
const connections = ref<Connection[]>([]);
const selected = computed(() => connections.value.find((item) => item.id === query.connectionId));
const isStale = computed(() => (props.kind === 'applications' ? selected.value?.appSyncStatus : selected.value?.userSyncStatus) === 'stale');
const query = reactive({ connectionId: 0, keyword: '', pageNum: 1, pageSize: 10 });
const rows = ref<(Application | User)[]>([]);
const total = ref(0);
const loading = ref(false);
async function load() {
    if (!query.connectionId) { rows.value = []; total.value = 0; return; }
    loading.value = true;
    try {
        const result = props.kind === 'applications' ? await casdoorApi.applications.request(query) : await casdoorApi.users.request(query);
        rows.value = result.list; total.value = result.total;
    } finally { loading.value = false; }
}
function search() { query.pageNum = 1; void load(); }
function changePage(page: number) { query.pageNum = page; void load(); }
onMounted(async () => { connections.value = await casdoorApi.connections.request() || []; query.connectionId = connections.value[0]?.id || 0; await load(); });
</script>
