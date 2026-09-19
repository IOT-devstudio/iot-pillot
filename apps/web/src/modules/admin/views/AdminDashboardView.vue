<script setup lang="ts">
import { onMounted, ref } from "vue";

import {
  AuthRequestError,
  listAdminUsers,
  type AdminUser,
  type CurrentUser,
} from "@/api/auth";
import { fetchCurrentUserWithRefresh } from "@/auth/current-user";
import { readAuthSession } from "@/auth/session";
import SignOutButton from "@/components/SignOutButton.vue";

const currentUser = ref<CurrentUser | null>(null);
const users = ref<AdminUser[]>([]);
const total = ref(0);
const loading = ref(true);
const errorMessage = ref("");

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
  <main class="admin-dashboard">
    <div class="admin-dashboard__inner">
      <header class="admin-dashboard__header">
        <div>
          <p class="admin-dashboard__eyebrow">IOT-PILLOT / ADMIN</p>
          <h1>管理控制台</h1>
          <p v-if="currentUser" class="admin-dashboard__welcome">
            {{ currentUser.username }} · 管理员
          </p>
        </div>
        <SignOutButton />
      </header>

      <section class="admin-card" aria-labelledby="users-title">
        <div class="admin-card__header">
          <div>
            <p class="admin-card__eyebrow">MEMBERS</p>
            <h2 id="users-title">注册用户</h2>
          </div>
          <span v-if="!loading && !errorMessage" class="admin-card__count">
            共 {{ total }} 人
          </span>
        </div>

        <p v-if="loading" class="admin-state" role="status">正在加载用户列表…</p>
        <p v-else-if="errorMessage" class="admin-state admin-state--error" role="alert">
          {{ errorMessage }}
        </p>
        <p v-else-if="users.length === 0" class="admin-state" role="status">
          暂无注册用户
        </p>
        <div v-else class="admin-table-wrap">
          <table class="admin-table">
            <thead>
              <tr>
                <th scope="col">用户 ID</th>
                <th scope="col">用户名</th>
                <th scope="col">注册时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="user in users" :key="user.user_id">
                <td>{{ user.user_id }}</td>
                <td>{{ user.name }}</td>
                <td>{{ user.created_at }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </main>
</template>

<style scoped>
.admin-dashboard {
  min-height: 100vh;
  padding: 36px clamp(16px, 4vw, 56px) 56px;
  color: #102b4e;
  background: #f4f6f8;
  font-family: "Avenir Next", "PingFang SC", "Microsoft YaHei", sans-serif;
}

.admin-dashboard__inner {
  max-width: 960px;
  margin: 0 auto;
}

.admin-dashboard__header,
.admin-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.admin-dashboard__header {
  margin-bottom: 28px;
}

.admin-dashboard__eyebrow,
.admin-card__eyebrow {
  margin: 0 0 8px;
  color: #1e659f;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

h1,
h2 {
  margin: 0;
}

h1 {
  font-size: clamp(26px, 4vw, 38px);
  letter-spacing: -0.03em;
}

h2 {
  font-size: 20px;
}

.admin-dashboard__welcome {
  margin: 8px 0 0;
  color: #5c6b7a;
  font-size: 13px;
}

.admin-card {
  padding: 24px;
  border: 1px solid #d8dee4;
  border-radius: 12px;
  background: #fff;
}

.admin-card__header {
  align-items: center;
  margin-bottom: 20px;
}

.admin-card__count {
  color: #5c6b7a;
  font-size: 13px;
}

.admin-state {
  margin: 0;
  padding: 24px 0 8px;
  color: #5c6b7a;
  font-size: 14px;
  text-align: center;
}

.admin-state--error {
  color: #a33d32;
}

.admin-table-wrap {
  overflow-x: auto;
}

.admin-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
  text-align: left;
}

.admin-table th,
.admin-table td {
  padding: 13px 12px;
  border-bottom: 1px solid #e5e9ed;
  white-space: nowrap;
}

.admin-table th {
  color: #5c6b7a;
  font-size: 12px;
  font-weight: 600;
}

.admin-table tr:last-child td {
  border-bottom: 0;
}
</style>
