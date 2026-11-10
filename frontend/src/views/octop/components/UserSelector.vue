<template>
    <el-select
        :model-value="modelValue"
        filterable
        remote
        clearable
        reserve-keyword
        :remote-method="onSearch"
        :loading="loading"
        :disabled="!instanceId"
        :placeholder="$t('octop.selectUser')"
        :class="selectClass"
        :aria-label="$t('octop.selectUser')"
        @update:model-value="onChange"
        @visible-change="onVisible"
    >
        <el-option v-for="u in options" :key="u.userId" :label="userLabel(u)" :value="u.userId">
            <span>{{ u.displayName || u.username || u.userId }}</span>
            <span class="ml-2 text-xs text-gray-500">{{ u.username }} · {{ u.userId }}</span>
        </el-option>
    </el-select>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue';
import { octopBusinessApi } from '../api';
import type { BizUserOption } from '../types';

const props = withDefaults(
    defineProps<{
        instanceId?: number;
        modelValue?: string;
        selectClass?: string;
    }>(),
    { selectClass: 'w-72' }
);

const emit = defineEmits<{
    'update:modelValue': [id: string | undefined];
    change: [user: BizUserOption | undefined];
}>();

const PAGE_SIZE = 20;
const options = ref<BizUserOption[]>([]);
const loading = ref(false);
/** 每次实例变化自增，用于丢弃过期的选项响应 */
let seq = 0;

const userLabel = (u: BizUserOption) => [u.displayName, u.username, u.userId].filter(Boolean).join(' · ');

const fetchOptions = async (keyword: string): Promise<BizUserOption[]> => {
    if (!props.instanceId) return [];
    const res = await octopBusinessApi.userOptions.request({ id: props.instanceId, keyword, pageNum: 1, pageSize: PAGE_SIZE });
    return res?.list || [];
};

const search = async (keyword: string) => {
    const mine = ++seq;
    loading.value = true;
    try {
        const list = await fetchOptions(keyword);
        if (mine !== seq) return;
        options.value = list;
        ensureSelected();
    } catch {
        if (mine === seq) options.value = [];
    } finally {
        if (mine === seq) loading.value = false;
    }
};

/** 已选用户不在当前选项内时，补一个占位选项（仅显示 ID），避免下拉框显示成空白 */
const ensureSelected = () => {
    if (props.modelValue && !options.value.some((u) => u.userId === props.modelValue)) {
        options.value = [{ userId: props.modelValue, username: '', displayName: '' }, ...options.value];
    }
};

/** 已选用户（来自 URL）解析显示名：按 ID 关键字搜索并精确匹配 */
const resolveSelected = async () => {
    if (!props.instanceId || !props.modelValue) return;
    const mine = ++seq;
    loading.value = true;
    try {
        const list = await fetchOptions(props.modelValue);
        if (mine !== seq) return;
        const hit = list.find((u) => u.userId === props.modelValue);
        options.value = hit ? [hit, ...list.filter((u) => u.userId !== hit.userId)] : list;
        ensureSelected();
    } catch {
        if (mine === seq) ensureSelected();
    } finally {
        if (mine === seq) loading.value = false;
    }
};

const onSearch = (keyword: string) => {
    void search(keyword);
};

const onVisible = (visible: boolean) => {
    if (visible && options.value.length <= 1) void search('');
};

const onChange = (id: string | undefined) => {
    const user = options.value.find((u) => u.userId === id);
    emit('update:modelValue', id || undefined);
    emit('change', id ? user : undefined);
};

watch(
    () => props.instanceId,
    () => {
        seq++;
        options.value = [];
        loading.value = false;
        void resolveSelected();
    }
);

watch(
    () => props.modelValue,
    (v, old) => {
        if (v && v !== old && !options.value.some((u) => u.userId === v && (u.username || u.displayName))) void resolveSelected();
    },
    { immediate: true }
);
</script>
