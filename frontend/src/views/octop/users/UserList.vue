<template>
    <div class="p-4 h-full overflow-auto">
        <div class="flex items-center justify-between mb-3">
            <h2 class="text-lg font-medium">{{ $t('octop.userList') }}</h2>
            <el-button link type="primary" icon="refresh" :disabled="!instanceId" @click="load">{{ $t('common.refresh') }}</el-button>
        </div>
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.userListHint')" />
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.userFilterBoundary')" />
        <el-alert type="info" :closable="false" class="mb-3" :title="$t('octop.switchInstanceClears')" />

        <div class="flex flex-wrap gap-3 mb-3 items-end">
            <el-form-item :label="$t('octop.selectInstance')" class="mb-0">
                <el-select v-model="instanceId" filterable clearable :placeholder="$t('octop.selectInstance')" class="w-72" @change="onInstanceChange">
                    <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
            </el-form-item>
            <el-form-item :label="$t('common.username')" class="mb-0">
                <el-input v-model="username" clearable class="w-48" @clear="load" @keyup.enter="load" />
            </el-form-item>
            <el-form-item :label="$t('common.email')" class="mb-0">
                <el-input v-model="email" clearable class="w-48" @clear="load" @keyup.enter="load" />
            </el-form-item>
            <el-button type="primary" :disabled="!instanceId" :loading="loading" @click="load">{{ $t('common.search') }}</el-button>
        </div>

        <el-alert v-if="page && page.status && page.status !== 'ok'" class="mb-3" :closable="false" :type="statusAlertType" :title="statusTitle" :description="page.message" />

        <el-table v-loading="loading" :data="page?.list || []" size="small" border>
            <el-table-column prop="id" :label="$t('octop.octopUserId')" min-width="120" />
            <el-table-column prop="username" :label="$t('common.username')" min-width="140" />
            <el-table-column prop="displayName" :label="$t('octop.displayName')" min-width="140">
                <template #default="{ row }">{{ row.displayName || $t('octop.notProvided') }}</template>
            </el-table-column>
            <el-table-column prop="email" :label="$t('common.email')" min-width="180">
                <template #default="{ row }">{{ row.email || $t('octop.notProvided') }}</template>
            </el-table-column>
            <el-table-column prop="status" :label="$t('octop.userStatus')" width="120">
                <template #default="{ row }">
                    <enum-tag :enums="OctopUserStatusEnum" :value="row.status || 'unknown'" />
                </template>
            </el-table-column>
            <template #empty>
                <el-empty :description="emptyText" />
            </template>
        </el-table>

        <div class="mt-3 flex justify-end">
            <el-pagination
                background
                layout="total, prev, pager, next, sizes"
                :total="page?.total || 0"
                v-model:current-page="pageNum"
                v-model:page-size="pageSize"
                :page-sizes="[10, 20, 50]"
                @current-change="load"
                @size-change="onPageSizeChange"
            />
        </div>
    </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import EnumTag from '@/components/enum-tag/EnumTag.vue';
import { octopInstanceApi } from '../api';
import { OctopUserStatusEnum } from '../enums';
import type { InstanceOverview, OctopUserPage } from '../types';

const { t } = useI18n();
const instances = ref<InstanceOverview[]>([]);
const instanceId = ref<number>();
const username = ref('');
const email = ref('');
const pageNum = ref(1);
const pageSize = ref(10);
const loading = ref(false);
const page = ref<OctopUserPage | null>(null);

const statusAlertType = computed(() => {
    const status = page.value?.status;
    if (status === 'forbidden' || status === 'unreachable') return 'error';
    return 'warning';
});

const statusTitle = computed(() => {
    const status = page.value?.status;
    if (status === 'unconfigured') return t('octop.userListUnconfigured');
    if (status === 'forbidden') return t('octop.userListForbidden');
    if (status === 'unreachable') return t('octop.userListUnreachable');
    if (status === 'unexpected') return t('octop.userListUnexpected');
    return t('octop.unsupported');
});

const emptyText = computed(() => {
    if (!instanceId.value) return t('octop.selectInstance');
    if (loading.value) return '';
    return t('common.empty');
});

const clearPage = () => {
    page.value = null;
};

const onInstanceChange = async () => {
    pageNum.value = 1;
    username.value = '';
    email.value = '';
    clearPage();
    if (instanceId.value) {
        await load();
    }
};

const onPageSizeChange = async () => {
    pageNum.value = 1;
    await load();
};

const load = async () => {
    if (!instanceId.value) {
        clearPage();
        return;
    }
    loading.value = true;
    try {
        page.value = await octopInstanceApi.users.request({
            id: instanceId.value,
            username: username.value,
            email: email.value,
            pageNum: pageNum.value,
            pageSize: pageSize.value,
        });
    } catch {
        page.value = {
            total: 0,
            list: [],
            pageNum: pageNum.value,
            pageSize: pageSize.value,
            status: 'unreachable',
            message: '',
            filterMode: 'local_full_list',
            filterNote: '',
        };
    } finally {
        loading.value = false;
    }
};

onMounted(async () => {
    instances.value = (await octopInstanceApi.overview.request()) || [];
});
</script>
