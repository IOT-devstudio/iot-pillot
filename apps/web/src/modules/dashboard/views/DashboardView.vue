<script setup lang="ts">
/**
 * 管理台 `/dashboard`（轻量运营工作台，issue #53）。
 *
 * 四个数据源并行加载、**各自持有 loading/错误/重试状态**——任一源失败只坏
 * 自己那块（指标卡或列表），不拖白整页。数据源与请求次数：
 *   fetchReady()        → 就绪指标（/health/ready，503 是合法答案）
 *   listAdminUsers(1,8) → 注册用户数(total) + 最近注册用户
 *   listAdmins()        → 管理员数(total)
 *   listMails(1,8)      → 发信总数(total) + 最近发信
 * 后端没有聚合接口（#53 注：第一版接受并行 4 个请求，量大再上 /admin/overview）。
 *
 * 「发信总数」不叫「近期发信数」：total 是累计值，第一版没有按时间过滤的接口，
 * 名字不骗人。完整用户列表在 /dashboard/admins，本页只留最近 8 条 + 入口。
 * 原 AdminUserTable 卡片随之移除（唯一引用者）。
 */
import { onMounted, ref } from "vue";

import {
  listAdminUsers,
  listAdmins,
  listMails,
  type AdminUser,
  type AdminUserList,
  type MailRecord,
} from "@/api/admin";
import { describeHealthError, fetchReady, type ReadyData } from "@/api/health";
import AppHeader from "@/components/common/AppHeader.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import SignOutButton from "@/components/SignOutButton.vue";
import { describeAuthError } from "../composables/useAdminPermissions";

/** 每个数据源一块：加载 / 错误文案（空 = 成功），互不影响。 */
const readyState = ref<{ loading: boolean; error: string; data: ReadyData | null }>({
  loading: true,
  error: "",
  data: null,
});
const usersState = ref<{
  loading: boolean;
  error: string;
  data: AdminUserList | null;
}>({ loading: true, error: "", data: null });
const adminsState = ref<{ loading: boolean; error: string; total: number | null }>({
  loading: true,
  error: "",
  total: null,
});
const mailsState = ref<{
  loading: boolean;
  error: string;
  total: number | null;
  items: MailRecord[];
}>({ loading: true, error: "", total: null, items: [] });

async function loadReady(): Promise<void> {
  readyState.value = { loading: true, error: "", data: null };
  try {
    readyState.value.data = await fetchReady();
    readyState.value.error = "";
  } catch (e: unknown) {
    readyState.value.error = describeHealthError(e);
  } finally {
    readyState.value.loading = false;
  }
}

async function loadUsers(): Promise<void> {
  usersState.value = { loading: true, error: "", data: null };
  try {
    usersState.value.data = await listAdminUsers(1, 8);
    usersState.value.error = "";
  } catch (e: unknown) {
    usersState.value.error = describeAuthError(e).message;
  } finally {
    usersState.value.loading = false;
  }
}

async function loadAdmins(): Promise<void> {
  adminsState.value = { loading: true, error: "", total: null };
  try {
    adminsState.value.total = (await listAdmins()).total;
    adminsState.value.error = "";
  } catch (e: unknown) {
    adminsState.value.error = describeAuthError(e).message;
  } finally {
    adminsState.value.loading = false;
  }
}

async function loadMails(): Promise<void> {
  mailsState.value = { loading: true, error: "", total: null, items: [] };
  try {
    const result = await listMails(1, 8);
    mailsState.value.total = result.total;
    mailsState.value.items = result.items;
    mailsState.value.error = "";
  } catch (e: unknown) {
    mailsState.value.error = describeAuthError(e).message;
  } finally {
    mailsState.value.loading = false;
  }
}

/** 四个源并发拉；失败已在各自函数里落进自己的 error，不会互相牵连。 */
onMounted(() => {
  void Promise.all([loadReady(), loadUsers(), loadAdmins(), loadMails()]);
});

/** 就绪指标的显示态：数据值或错误文案，二选一。 */
function readyDisplay(): string {
  if (readyState.value.error) return readyState.value.error;
  const d = readyState.value.data;
  if (!d) return "检测中…";
  return d.status === "ready" ? "就绪" : "依赖不可用";
}

/** 指标数值：加载中 … / 失败 —（错误文案在各自重试行里给）。 */
function metricValue(
  state: { loading: boolean; error: string },
  value: () => string,
): string {
  if (state.loading) return "…";
  if (state.error) return "—";
  return value();
}

/** 快捷入口：保持原卡片设计（设计反馈），位于页面底部的次级区；顺序与顶栏一致 */
const entries = [
  { title: "意向成员", to: "/recruitment/prospects" },
  { title: "表单管理", to: "/forms" },
  { title: "邮件模板", to: "/templates" },
  { title: "用户与权限", to: "/dashboard/admins" },
  { title: "系统设置", to: "/settings" },
];

/** 最近注册用户的角色标签 */
function roleLabel(user: AdminUser): string {
  return user.is_admin ? "管理员" : "普通用户";
}
</script>

<template>
  <div class="page">
    <AppHeader />

    <main class="page__body">
      <PageHeader eyebrow="iot-pillot · 招新管理" title="控制台">
        <template #actions>
          <SignOutButton />
        </template>
      </PageHeader>

      <!-- 顶部指标：四个源各管各的卡，谁失败谁重试 -->
      <section class="metrics" aria-label="运营概览">
        <div class="metric">
          <span class="metric__label">注册用户</span>
          <span class="metric__value">
            {{ metricValue(usersState, () => String(usersState.data?.total ?? 0)) }}
          </span>
          <button
            v-if="usersState.error"
            class="metric__retry"
            type="button"
            @click="loadUsers"
          >
            重试
          </button>
        </div>

        <div class="metric">
          <span class="metric__label">管理员</span>
          <span class="metric__value">
            {{ metricValue(adminsState, () => String(adminsState.total ?? 0)) }}
          </span>
          <button
            v-if="adminsState.error"
            class="metric__retry"
            type="button"
            @click="loadAdmins"
          >
            重试
          </button>
        </div>

        <div class="metric">
          <span class="metric__label">发信总数</span>
          <span class="metric__value">
            {{ metricValue(mailsState, () => String(mailsState.total ?? 0)) }}
          </span>
          <button
            v-if="mailsState.error"
            class="metric__retry"
            type="button"
            @click="loadMails"
          >
            重试
          </button>
        </div>

        <div
          class="metric"
          :class="{
            'metric--ok': readyState.data?.status === 'ready',
            'metric--down':
              readyState.data?.status !== undefined &&
              readyState.data.status !== 'ready',
          }"
        >
          <span class="metric__label">服务就绪</span>
          <span class="metric__value metric__value--ready">
            <span class="metric__dot" aria-hidden="true"></span>
            {{ readyDisplay() }}
          </span>
          <button
            v-if="readyState.error"
            class="metric__retry"
            type="button"
            @click="loadReady"
          >
            重试
          </button>
        </div>
      </section>

      <!-- 两个最近列表 -->
      <section class="panels" aria-label="最近动态">
        <!-- 最近注册用户 -->
        <div class="panel panel--pad">
          <header class="panel-head">
            <h2 class="panel-head__title">最近注册用户</h2>
            <router-link to="/dashboard/admins" class="panel-head__link">
              查看全部 →
            </router-link>
          </header>

          <div v-if="usersState.loading" role="status" aria-label="正在加载用户">
            <el-skeleton :rows="4" animated />
          </div>
          <div v-else-if="usersState.error" class="state state--error" role="alert">
            <span>{{ usersState.error }}</span>
            <el-button link type="primary" @click="loadUsers">重试</el-button>
          </div>
          <p
            v-else-if="(usersState.data?.items.length ?? 0) === 0"
            class="state"
            role="status"
          >
            暂无注册用户
          </p>
          <ul v-else class="rows">
            <li v-for="u in usersState.data?.items" :key="u.user_id" class="row">
              <span class="row__main">
                <span class="row__title">{{ u.name }}</span>
                <span class="row__sub">#{{ u.user_id }}</span>
              </span>
              <span class="row__meta">{{ u.created_at }}</span>
              <el-tag
                :type="u.is_admin ? 'success' : 'info'"
                size="small"
                effect="plain"
              >
                {{ roleLabel(u) }}
              </el-tag>
            </li>
          </ul>
        </div>

        <!-- 最近发信 -->
        <div class="panel panel--pad">
          <header class="panel-head">
            <h2 class="panel-head__title">最近发信</h2>
            <!-- 完整发信记录页随 #52 提供，这里不放假链接 -->
            <span class="panel-head__hint">全部记录随 #52 提供</span>
          </header>

          <div v-if="mailsState.loading" role="status" aria-label="正在加载发信记录">
            <el-skeleton :rows="4" animated />
          </div>
          <div v-else-if="mailsState.error" class="state state--error" role="alert">
            <span>{{ mailsState.error }}</span>
            <el-button link type="primary" @click="loadMails">重试</el-button>
          </div>
          <p v-else-if="mailsState.items.length === 0" class="state" role="status">
            暂无发信记录
          </p>
          <ul v-else class="rows">
            <li v-for="m in mailsState.items" :key="m.id" class="row">
              <span class="row__main">
                <span class="row__title">{{ m.title }}</span>
                <span class="row__sub">{{ m.to_email }}</span>
              </span>
              <span class="row__meta">{{ m.created_at }}</span>
            </li>
          </ul>
        </div>
      </section>

      <!-- 快捷入口：恢复原卡片设计（设计反馈），保留在页面底部作为次级区 -->
      <section class="entries" aria-label="快捷入口">
        <router-link v-for="e in entries" :key="e.to" :to="e.to" class="entry">
          <span class="entry__title">{{ e.title }}</span>
          <span class="entry__arrow" aria-hidden="true">→</span>
        </router-link>
      </section>
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
}

.page__body {
  display: flex;
  flex-direction: column;
  gap: clamp(20px, 2.4vw, 28px);
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

/* —— 顶部指标 —— */
.metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: clamp(12px, 1.4vw, 18px);
}

.metric {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: clamp(16px, 1.6vw, 22px);
  border: 1px solid var(--line);
  border-radius: 2px;
  background: var(--surface);
}

.metric__label {
  color: var(--ink-soft);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.metric__value {
  font-family: var(--font-serif);
  font-size: clamp(28px, 2.6vw, 36px);
  font-weight: 600;
  line-height: 1.1;
  color: var(--ink);
}

.metric__value--ready {
  display: inline-flex;
  gap: 10px;
  align-items: center;
  font-size: clamp(20px, 1.8vw, 24px);
}

.metric__dot {
  width: 10px;
  height: 10px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--ink-faint);
}

.metric--ok .metric__dot {
  background: var(--green);
  box-shadow: 0 0 0 3px rgb(46 139 87 / 16%);
}

.metric--down .metric__dot {
  background: var(--red);
  box-shadow: 0 0 0 3px rgb(184 66 53 / 16%);
}

.metric__retry {
  align-self: flex-start;
  padding: 0;
  color: var(--blue);
  border: 0;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  text-decoration: underline;
  text-underline-offset: 3px;
}

/* —— 最近列表 —— */
.panels {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(440px, 100%), 1fr));
  gap: clamp(16px, 1.8vw, 24px);
}

.panel-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  margin: 0 0 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--line);
}

.panel-head__title {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.panel-head__link {
  color: var(--blue);
  font-size: 13px;
  text-decoration: none;
}

.panel-head__link:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}

.panel-head__hint {
  color: var(--ink-faint);
  font-size: 12px;
}

.state {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 20px 0 8px;
  color: var(--ink-soft);
  font-size: 14px;
  text-align: center;
}

.state--error {
  color: var(--red);
}

.rows {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}

.row {
  display: flex;
  gap: 14px;
  align-items: center;
  padding: 11px 2px;
  border-bottom: 1px solid var(--line-soft);
}

.row:last-child {
  border-bottom: 0;
}

.row__main {
  display: flex;
  gap: 10px;
  align-items: baseline;
  min-width: 0;
  flex: 1;
}

.row__title {
  overflow: hidden;
  color: var(--ink);
  font-size: 14px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__sub {
  overflow: hidden;
  color: var(--ink-faint);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__meta {
  flex: 0 0 auto;
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 11px;
  white-space: nowrap;
}

/* —— 快捷入口 —— */
.entries {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: clamp(16px, 1.4vw, 22px);
}

.entry {
  position: relative;
  display: flex;
  gap: clamp(18px, 1.8vw, 24px);
  align-items: center;
  min-height: clamp(140px, 12vw, 176px);
  padding: clamp(28px, 3vw, 40px) clamp(28px, 3.2vw, 44px);
  overflow: hidden;
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 2px;
  background: var(--surface);
  text-decoration: none;
  transition: border-color 180ms ease, box-shadow 180ms ease, transform 180ms ease;
}

.entry::before {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 3px;
  background: var(--blue);
  content: "";
  opacity: 0.34;
  transition: opacity 180ms ease;
}

.entry:hover {
  border-color: var(--line-strong);
  box-shadow: var(--el-box-shadow);
  transform: translateY(-2px);
}

.entry:hover::before {
  opacity: 1;
}

.entry:focus-visible {
  border-color: var(--blue);
  outline: 2px solid var(--blue);
  outline-offset: 2px;
}

.entry__title {
  font-family: var(--font-serif);
  font-size: clamp(24px, 2.2vw, 30px);
  font-weight: 600;
  letter-spacing: -0.01em;
}

.entry__arrow {
  margin-left: auto;
  color: var(--ink-faint);
  font-size: clamp(18px, 1.6vw, 22px);
  transition: color 180ms ease, transform 180ms ease;
}

.entry:hover .entry__arrow {
  color: var(--blue);
  transform: translateX(3px);
}

@media (max-width: 1024px) {
  .entry {
    min-height: clamp(132px, 15vw, 156px);
    padding: 28px 30px;
  }
}

@media (max-width: 720px) {
  .entries {
    grid-template-columns: 1fr;
    gap: 14px;
  }

  .entry {
    min-height: 0;
    gap: 16px;
    padding: 24px 22px;
  }

  .entry__title {
    font-size: 23px;
  }
}

@media (max-width: 420px) {
  .entry__title {
    font-size: 21px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .entry {
    transition: none;
  }

  .entry:hover {
    transform: none;
  }

  .entry:hover .entry__arrow {
    transform: none;
  }
}
</style>
