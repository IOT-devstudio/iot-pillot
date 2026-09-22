<script setup lang="ts">
/**
 * 子页面的「返回控制台」链接。
 *
 * 为什么抽成组件：控制台是枢纽，所有能从它跳过去的模块都要能走回来。
 * 这段 `.back-link` 此前在 ProspectDetailView 里手写了一份，再加几个页面
 * 就会变成每个页面复读一遍同样的样式与 router.push。
 *
 * 用固定父级路径而不是 router.back()：这些页面都可以直接输 URL 或从顶栏进入，
 * 后退历史未必存在；固定路径才可预测。
 */
import { useRouter } from "vue-router";

const props = withDefaults(
  defineProps<{
    /** 返回目标，默认控制台 */
    to?: string;
    /** 链接文案 */
    label?: string;
  }>(),
  { to: "/dashboard", label: "← 返回控制台" },
);

const router = useRouter();
</script>

<template>
  <button class="back-link" type="button" @click="router.push(props.to)">
    {{ props.label }}
  </button>
</template>

<style scoped>
.back-link {
  align-self: flex-start;
  padding: 0;
  color: var(--ink-soft);
  border: 0;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  text-decoration: none;
}

.back-link:hover {
  color: var(--blue);
  text-decoration: underline;
  text-underline-offset: 3px;
}
</style>
