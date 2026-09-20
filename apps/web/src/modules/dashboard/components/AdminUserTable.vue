<script setup lang="ts">
/**
 * 注册用户表（管理台的一块）。
 *
 * 从 modules/admin/views/AdminDashboardView.vue 抽出来：那个独立页面并入管理台
 * 之后，用户名单作为其中一块卡片保留，`/api/v1/admin/users` 的调用链原样搬过来，
 * 只把渲染换成本项目的暖纸面板 + el-table。
 *
 * 为什么先调 /me 再调列表：/me 可能刚刚轮换过 access token，必须重读会话，
 * 否则列表请求会带着已经失效的旧令牌（照搬原实现的处理）。
 */
import { computed, onMounted, ref } from "vue";

import {
  AuthRequestError,
  listAdminUsers,
  type AdminUser,
  type CurrentUser,
} from "@/api/auth";
import { fetchCurrentUserWithRefresh } from "@/auth/current-user";
import { readAuthSession } from "@/auth/session";

const currentUser = ref<CurrentUser | null>(null);
const users = ref<AdminUser[]>([]);
const total = ref(0);
const loading = ref(true);
const errorMessage = ref("");

/** 右上角标注：当前登录人 + 总人数，两者都拿不到时整块隐藏 */
const metaText = computed(() => {
  const parts: string[] = [];
  if (currentUser.value) {
    parts.push(`${currentUser.value.username} · 管理员`);
  }
  if (!loading.value && !errorMessage.value) {
    parts.push(`共 ${total.value} 人`);
  }
  return parts.join(" · ");
});

function messageOf(error: unknown): string {
  return error instanceof AuthRequestError
    ? error.message
    : "暂时无法加载管理数据，请稍后重试";
}

onMounted(async () => {
  const session = readAuthSession();
  if (session === null) {
    errorMessage.value = "登录状态已失效，请重新登录";
    loading.value = false;
    return;
  }

  try {
    currentUser.value = await fetchCurrentUserWithRefresh(session.access_token);
    // /me 可能刚刚轮换过令牌，重新读取会话以避免列表请求继续使用旧 token。
    const currentSession = readAuthSession();
    if (currentSession === null) {
      errorMessage.value = "登录状态已失效，请重新登录";
      return;
    }

    const result = await listAdminUsers(currentSession.access_token);
    users.value = result.items;
    total.value = result.total;
  } catch (error: unknown) {
    errorMessage.value = messageOf(error);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <section class="panel users">
    <header class="users__head">
      <h2 class="users__title">注册用户</h2>
      <span v-if="metaText" class="users__meta">{{ metaText }}</span>
    </header>

    <p v-if="loading" class="users__state" role="status">正在加载用户列表…</p>
    <p
      v-else-if="errorMessage"
      class="users__state users__state--error"
      role="alert"
    >
      {{ errorMessage }}
    </p>
    <p v-else-if="users.length === 0" class="users__state" role="status">
      暂无注册用户
    </p>
    <el-table v-else :data="users">
      <el-table-column prop="user_id" label="用户 ID" width="120" />
      <el-table-column prop="name" label="用户名" min-width="180" />
      <el-table-column prop="created_at" label="注册时间" min-width="200" />
    </el-table>
  </section>
</template>

<style scoped>
.users {
  padding: 24px 26px;
}

.users__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  margin: 0 0 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--line);
}

.users__title {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.users__meta {
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 12px;
  letter-spacing: 0.04em;
  white-space: nowrap;
}

.users__state {
  margin: 0;
  padding: 20px 0 8px;
  color: var(--ink-soft);
  font-size: 14px;
  text-align: center;
}

.users__state--error {
  color: var(--red);
}

:deep(.el-table) {
  --el-table-border-color: var(--line);
  font-size: 13px;
}

:deep(.el-table th.el-table__cell) {
  font-weight: 700;
  letter-spacing: 0.04em;
}

:deep(.el-table .cell) {
  color: var(--ink);
}

@media (max-width: 560px) {
  .users {
    padding: 18px 16px;
  }

  .users__head {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }
}
</style>
