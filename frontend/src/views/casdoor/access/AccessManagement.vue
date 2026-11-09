<template>
    <div class="p-4">
        <h2 class="mb-4">{{ $t('casdoor.access') }}</h2>
        <el-alert :title="$t('casdoor.accessReadOnly')" type="warning" :closable="false" class="mb-4" />
        <el-select v-model="query.connectionId" :placeholder="$t('casdoor.chooseConnection')" @change="search">
            <el-option v-for="item in connections" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-tabs v-model="query.kind" @tab-change="search">
            <el-tab-pane :label="$t('casdoor.roles')" name="role" />
            <el-tab-pane :label="$t('casdoor.groups')" name="group" />
            <el-tab-pane :label="$t('casdoor.permissions')" name="permission" />
        </el-tabs>
        <el-table v-loading="loading" :data="rows" row-key="id">
            <el-table-column prop="sourceId" :label="$t('casdoor.objectId')" />
            <el-table-column prop="lastSyncAt" :label="$t('casdoor.lastSync')" />
            <el-table-column :label="$t('casdoor.presence')">
                <template #default="{ row }">{{ $t(row.present === 1 ? 'casdoor.present' : row.present === -1 ? 'casdoor.absent' : 'casdoor.unknown') }}</template>
            </el-table-column>
        </el-table>
        <el-pagination class="mt-3" :current-page="query.pageNum" :page-size="query.pageSize" :total="total" layout="total, prev, pager, next" @current-change="changePage" />
    </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { casdoorApi, type AuthorizationObject, type Connection, type ObjectQuery } from '../api';
const connections = ref<Connection[]>([]);
const rows = ref<AuthorizationObject[]>([]);
const total = ref(0);
const loading = ref(false);
const query = reactive<ObjectQuery>({ connectionId: 0, kind: 'role', pageNum: 1, pageSize: 10 });
async function load() {
    if (!query.connectionId) { rows.value = []; total.value = 0; return; }
    loading.value = true;
    try { const result = await casdoorApi.objects.request(query); rows.value = result.list; total.value = result.total; }
    finally { loading.value = false; }
}
function search() { query.pageNum = 1; void load(); }
function changePage(page: number) { query.pageNum = page; void load(); }
onMounted(async () => { connections.value = await casdoorApi.connections.request() || []; query.connectionId = connections.value[0]?.id || 0; await load(); });
</script>
