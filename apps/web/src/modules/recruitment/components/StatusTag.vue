<script setup lang="ts">
/**
 * 状态标签：把意向成员的状态统一渲染成 el-tag。
 */
import { computed } from "vue";
import type { ProspectStatus } from "../status";
import { prospectStatusMeta } from "../status";

const props = defineProps<{ status: ProspectStatus }>();

const meta = computed(() => prospectStatusMeta(props.status));
</script>

<template>
  <el-tag :type="meta.tagType" size="small">{{ meta.label }}</el-tag>
</template>
