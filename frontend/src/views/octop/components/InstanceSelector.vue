<template>
    <div class="flex items-center gap-2">
        <el-select
            :model-value="modelValue"
            filterable
            :loading="loading"
            :placeholder="$t('octop.selectInstance')"
            :class="selectClass"
            :aria-label="$t('octop.selectInstance')"
            @update:model-value="onChange"
        >
            <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <span v-if="current?.apiUrl" class="text-xs text-gray-500 truncate max-w-64" :title="current.apiUrl">{{ current.apiUrl }}</span>
    </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref, watch } from 'vue';
import { octopInstanceApi } from '../api';
import type { InstanceOverview } from '../types';

const props = withDefaults(
    defineProps<{
        modelValue?: number;
        /** 无上下文时自动选择第一个有权实例；URL 显式给出无效实例时不会改选 */
        autoSelect?: boolean;
        selectClass?: string;
    }>(),
    { autoSelect: true, selectClass: 'w-64' }
);

const emit = defineEmits<{
    'update:modelValue': [id: number | undefined];
    change: [id: number | undefined];
    loaded: [list: InstanceOverview[]];
}>();

/** 当前 modelValue 是否在授权实例列表内；未指定实例或加载完成前为 undefined */
const valid = defineModel<boolean | undefined>('valid');

const instances = ref<InstanceOverview[]>([]);
const loading = ref(false);

const current = computed(() => instances.value.find((i) => i.id === props.modelValue));

const syncValid = () => {
    valid.value = props.modelValue === undefined ? undefined : instances.value.some((i) => i.id === props.modelValue);
};

const onChange = (id: number | undefined) => {
    emit('update:modelValue', id);
    emit('change', id);
};

const load = async () => {
    loading.value = true;
    try {
        instances.value = (await octopInstanceApi.overview.request()) || [];
        emit('loaded', instances.value);
        if (props.modelValue === undefined && props.autoSelect && instances.value.length > 0) {
            onChange(instances.value[0].id);
        }
    } finally {
        loading.value = false;
    }
    syncValid();
};

defineExpose({ reload: load, instances });

onMounted(load);

// 外部（URL）改变选中实例后重新判定有效性
watch(
    () => props.modelValue,
    () => {
        if (loading.value) return;
        if (instances.value.length > 0) syncValid();
        else if (props.modelValue === undefined) valid.value = undefined;
    }
);
</script>
