<template>
    <div class="p-4">
        <h2 class="mb-4">{{ $t('casdoor.operations') }}</h2>
        <el-select v-model="query.connectionId" :placeholder="$t('casdoor.chooseConnection')" class="mb-4" @change="search">
            <el-option v-for="item in connections" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
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
import { onMounted, reactive, ref } from 'vue';
import { casdoorApi, type Connection, type Operation } from '../api';
const connections = ref<Connection[]>([]);
const rows = ref<Operation[]>([]);
const total = ref(0);
const loading = ref(false);
const query = reactive({ connectionId: 0, keyword: '', pageNum: 1, pageSize: 10 });
async function load() {
    if (!query.connectionId) { rows.value = []; total.value = 0; return; }
    loading.value = true;
    try { const result = await casdoorApi.operations.request(query); rows.value = result.list; total.value = result.total; }
    finally { loading.value = false; }
}
function search() { query.pageNum = 1; void load(); }
function changePage(page: number) { query.pageNum = page; void load(); }
onMounted(async () => { connections.value = await casdoorApi.connections.request() || []; query.connectionId = connections.value[0]?.id || 0; await load(); });
</script>
