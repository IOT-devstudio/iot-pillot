<script setup lang="ts">
/**
 * 子页面的「返回」链接：按角色回到自己的落点（issue #94）。
 *
 * 为什么抽成组件：所有从枢纽页跳过来的子页面都要能走回来。这段 .back-link
 * 此前在 ProspectDetailView 里手写了一份，再加几个页面就会变成每个页面
 * 复读一遍同样的样式与 router.push。
 *
 * 不传 to 时按角色选择目标（管理员 → 控制台，普通成员/未知 → 个人首页），
 * 规则见 back-link.ts（纯函数，单独有测试）。角色查询失败不拦人：
 * /user/home 两个角色都能进（访问规则单向），按普通用户落点展示即可，
 * 不会把人送进自己也进不去的地方。
 *
 * 用固定父级路径而不是 router.back()：这些页面都可以直接输 URL 或从顶栏进入，
 * 后退历史未必存在；固定路径才可预测。显式传 to 仍可覆盖（如招新详情返回列表）。
 */
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import { fetchCurrentUser } from "@/api/admin";
import { backLinkLabel, backLinkTarget } from "./back-link";

const props = defineProps<{
  /** 返回目标；缺省时按当前角色落点自动选择 */
  to?: string;
  /** 链接文案；缺省时跟随目标（控制台 / 个人首页） */
  label?: string;
}>();

const router = useRouter();
/** 当前角色；null = 未确认（加载中或查询失败），按普通用户处理 */
const role = ref<string | null>(null);

onMounted(async () => {
  // 已传显式目标就不必查角色了——招新详情页那种「返回固定列表」用不到它
  if (props.to !== undefined) {
    return;
  }
  try {
    role.value = (await fetchCurrentUser()).role;
  } catch {
    role.value = null;
  }
});

const target = computed(() => backLinkTarget(props.to, role.value));
const text = computed(() => backLinkLabel(props.to, props.label, role.value));
</script>

<template>
  <button class="back-link" type="button" @click="router.push(target)">
    {{ text }}
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
