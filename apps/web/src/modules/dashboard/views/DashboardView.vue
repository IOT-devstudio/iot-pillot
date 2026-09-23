<script setup lang="ts">
/**
 * 管理台 `/dashboard`。四个数据源并行加载，并各自持有 loading、错误与重试状态，
 * 单个接口失败时只影响对应指标或列表。
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
import PageHeader from "@/components/common/PageHeader.vue";
import { describeAuthError } from "../composables/useAdminPermissions";

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
  } catch (error: unknown) {
    readyState.value.error = describeHealthError(error);
  } finally {
    readyState.value.loading = false;
  }
}

async function loadUsers(): Promise<void> {
  usersState.value = { loading: true, error: "", data: null };
  try {
    usersState.value.data = await listAdminUsers(1, 8);
  } catch (error: unknown) {
    usersState.value.error = describeAuthError(error).message;
  } finally {
    usersState.value.loading = false;
  }
}

async function loadAdmins(): Promise<void> {
  adminsState.value = { loading: true, error: "", total: null };
  try {
    adminsState.value.total = (await listAdmins()).total;
  } catch (error: unknown) {
    adminsState.value.error = describeAuthError(error).message;
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
  } catch (error: unknown) {
    mailsState.value.error = describeAuthError(error).message;
  } finally {
    mailsState.value.loading = false;
  }
}

onMounted(() => {
  void Promise.all([loadReady(), loadUsers(), loadAdmins(), loadMails()]);
});

function readyDisplay(): string {
  if (readyState.value.error) return readyState.value.error;
  const data = readyState.value.data;
  if (!data) return "检测中…";
  return data.status === "ready" ? "就绪" : "依赖不可用";
}

function metricValue(
  state: { loading: boolean; error: string },
  value: () => string,
): string {
  if (state.loading) return "…";
  if (state.error) return "—";
  return value();
}

/** 最近注册用户的角色标签 */
function roleLabel(user: AdminUser): string {
  return user.is_admin ? "管理员" : "普通用户";
}
</script>

<template>
  <div class="page">
    <main class="page__body">
      <PageHeader eyebrow="IOT-PILLOT · 招新管理" title="控制台" />

      <section class="overview" aria-label="运营概览">
        <article class="metric metric--primary">
          <div>
            <span class="metric__label">注册用户</span>
            <span class="metric__value metric__value--primary">
              {{ metricValue(usersState, () => String(usersState.data?.total ?? 0)) }}
            </span>
          </div>
          <div class="metric__foot">
            <button
              v-if="usersState.error"
              class="metric__retry"
              type="button"
              @click="loadUsers"
            >
              重试
            </button>
            <router-link to="/dashboard/admins" class="metric__link">
              查看用户 <span aria-hidden="true">→</span>
            </router-link>
          </div>
        </article>

        <div class="overview__secondary">
          <article class="metric metric--compact">
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
          </article>

          <article class="metric metric--compact">
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
          </article>

          <article
            class="metric metric--status"
            :class="{
              'metric--ok': readyState.data?.status === 'ready',
              'metric--down':
                readyState.error.length > 0 ||
                (readyState.data !== null && readyState.data.status !== 'ready'),
            }"
          >
            <span class="metric__label">系统状态</span>
            <span class="metric__status-value">
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
          </article>
        </div>
      </section>

      <section class="activity" aria-label="最近动态">
        <header class="activity__heading">
          <h2>最近动态</h2>
        </header>

        <div class="panels">
          <section class="panel panel--pad activity-panel">
            <header class="panel-head">
              <h3 class="panel-head__title">最近注册用户</h3>
              <router-link to="/dashboard/admins" class="panel-head__link">
                查看全部 <span aria-hidden="true">→</span>
              </router-link>
            </header>

            <div
              v-if="usersState.loading"
              class="activity-panel__loading"
              role="status"
              aria-label="正在加载用户"
            >
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
              <li v-for="user in usersState.data?.items" :key="user.user_id" class="row">
                <span class="row__main">
                  <span class="row__title">{{ user.name }}</span>
                  <span class="row__sub">#{{ user.user_id }}</span>
                </span>
                <time class="row__meta">{{ user.created_at }}</time>
                <el-tag
                  :type="user.is_admin ? 'success' : 'info'"
                  size="small"
                  effect="plain"
                >
                  {{ roleLabel(user) }}
                </el-tag>
              </li>
            </ul>
          </section>

          <section class="panel panel--pad activity-panel">
            <header class="panel-head">
              <h3 class="panel-head__title">最近发信</h3>
              <span class="panel-head__meta">
                {{ mailsState.loading ? "" : `累计 ${mailsState.error ? "—" : (mailsState.total ?? 0)} 封` }}
              </span>
            </header>

            <div
              v-if="mailsState.loading"
              class="activity-panel__loading"
              role="status"
              aria-label="正在加载发信记录"
            >
              <el-skeleton :rows="4" animated />
            </div>
            <div v-else-if="mailsState.error" class="state state--error" role="alert">
              <span>{{ mailsState.error }}</span>
              <el-button link type="primary" @click="loadMails">重试</el-button>
            </div>
            <div v-else-if="mailsState.items.length === 0" class="empty-state" role="status">
              <span class="empty-state__mark" aria-hidden="true">
                <svg viewBox="0 0 48 36">
                  <path d="M5 6h38v25H5zM6 7l18 14L42 7M12 12l8 6M36 12l-8 6" />
                </svg>
              </span>
              <span>暂无发信记录</span>
            </div>
            <ul v-else class="rows">
              <li v-for="mail in mailsState.items" :key="mail.id" class="row row--mail">
                <span class="row__main">
                  <span class="row__title">{{ mail.title }}</span>
                  <span class="row__sub">{{ mail.to_email }}</span>
                </span>
                <time class="row__meta">{{ mail.created_at }}</time>
              </li>
            </ul>
          </section>
        </div>
      </section>
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: calc(100dvh - 59px);
}

.page__body {
  display: flex;
  width: 100%;
  flex-direction: column;
  gap: clamp(26px, 3vw, 39px);
  margin: 0;
  padding: clamp(27px, 3.8vw, 48px) 0 42px;
}

.overview {
  display: grid;
  grid-template-columns: minmax(290px, 1.15fr) minmax(340px, 1fr);
  gap: 20px;
}

.metric {
  min-width: 0;
  border: 1px solid var(--line);
  background: var(--surface);
  box-shadow: 0 2px 12px rgb(16 43 78 / 4%);
}

.metric--primary {
  display: flex;
  min-height: 218px;
  flex-direction: column;
  justify-content: space-between;
  padding: 28px 34px 23px;
}

.metric__label {
  display: block;
  color: var(--ink-soft);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.metric__value {
  display: block;
  margin-top: 12px;
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: clamp(38px, 3.3vw, 50px);
  font-variant-numeric: tabular-nums;
  font-weight: 500;
  line-height: 1;
}

.metric__value--primary {
  margin-top: 16px;
  font-size: clamp(72px, 6.4vw, 88px);
  letter-spacing: -0.045em;
}

.metric__foot {
  display: flex;
  min-height: 24px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.metric__link,
.panel-head__link {
  color: var(--blue);
  font-size: 12px;
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
}

.metric__link:hover,
.panel-head__link:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}

.metric__retry {
  padding: 0;
  color: var(--blue);
  border: 0;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
  text-decoration: underline;
  text-underline-offset: 3px;
}

.metric__retry:focus-visible,
.metric__link:focus-visible,
.panel-head__link:focus-visible {
  border-radius: 2px;
  outline: 2px solid var(--blue-bright);
  outline-offset: 3px;
}

.overview__secondary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-template-rows: minmax(118px, 1fr) 62px;
  gap: 18px;
}

.metric--compact {
  padding: 22px 25px 17px;
}

.metric--compact .metric__value {
  font-size: clamp(40px, 3.3vw, 50px);
}

.metric--compact .metric__retry {
  margin-top: 7px;
}

.metric--status {
  display: flex;
  grid-column: 1 / -1;
  align-items: center;
  gap: 17px;
  padding: 0 25px;
}

.metric--status .metric__label {
  font-size: 11px;
  white-space: nowrap;
}

.metric__status-value {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
  color: var(--ink-soft);
  font-size: 14px;
  font-weight: 700;
}

.metric--ok .metric__status-value {
  color: var(--green);
}

.metric--down .metric__status-value {
  color: var(--red);
}

.metric__dot {
  width: 8px;
  height: 8px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--ink-faint);
}

.metric--ok .metric__dot {
  background: var(--green);
  box-shadow: 0 0 0 4px rgb(46 139 87 / 10%);
}

.metric--down .metric__dot {
  background: var(--red);
  box-shadow: 0 0 0 4px rgb(184 66 53 / 10%);
}

.metric--status .metric__retry {
  margin-left: auto;
}

.activity__heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  margin: 0 0 15px;
}

.activity__heading h2 {
  margin: 0;
  font-family: var(--font-serif);
  font-size: 23px;
  font-weight: 600;
  letter-spacing: -0.015em;
}

.activity__heading > span {
  color: var(--ink-faint);
  font-size: 11px;
  letter-spacing: 0.04em;
}

.panels {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.activity-panel {
  min-height: 372px;
  padding: 22px 26px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 0;
  padding-bottom: 15px;
  border-bottom: 1px solid var(--line);
}

.panel-head__title {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.panel-head__meta {
  color: var(--ink-faint);
  font-size: 11px;
  white-space: nowrap;
}

.activity-panel__loading {
  padding-top: 20px;
}

.rows {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}

.row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 13px;
  min-height: 67px;
  padding: 9px 2px;
  border-bottom: 1px solid var(--line-soft);
}

.row:last-child {
  border-bottom: 0;
}

.row__main {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 9px;
}

.row__title,
.row__sub {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__title {
  color: var(--ink);
  font-size: 13px;
  font-weight: 650;
}

.row__sub {
  color: var(--ink-faint);
  font-size: 11px;
}

.row--mail .row__main {
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.row__meta {
  max-width: 164px;
  overflow: hidden;
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.state {
  display: flex;
  min-height: 260px;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin: 0;
  padding: 20px 0;
  color: var(--ink-soft);
  font-size: 13px;
  text-align: center;
}

.state--error {
  color: var(--red);
}

.empty-state {
  display: flex;
  min-height: 278px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 13px;
  color: var(--ink-soft);
  font-size: 13px;
}

.empty-state__mark {
  display: grid;
  width: 58px;
  height: 48px;
  place-items: center;
  color: #98a9b7;
  border: 1px solid rgb(22 77 128 / 16%);
  transform: rotate(-3deg);
}

.empty-state__mark svg {
  width: 38px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.1;
}

@media (max-width: 1100px) {
  .overview {
    grid-template-columns: 1fr;
  }

  .metric--primary {
    min-height: 182px;
  }

  .overview__secondary {
    grid-template-rows: minmax(106px, 1fr) 60px;
  }

  .panels {
    grid-template-columns: 1fr;
  }

  .activity-panel {
    min-height: 310px;
  }

  .state {
    min-height: 205px;
  }

  .empty-state {
    min-height: 215px;
  }
}

@media (max-width: 680px) {
  .page {
    min-height: calc(100dvh - 59px);
  }

  .page__body {
    gap: 24px;
    padding-top: 21px;
  }

  .overview {
    gap: 12px;
  }

  .metric--primary {
    min-height: 166px;
    padding: 20px 20px 17px;
  }

  .metric__value--primary {
    font-size: 68px;
  }

  .overview__secondary {
    gap: 12px;
    grid-template-rows: 104px 58px;
  }

  .metric--compact {
    padding: 17px 17px 13px;
  }

  .metric--compact .metric__value {
    margin-top: 10px;
    font-size: 39px;
  }

  .metric--status {
    gap: 12px;
    padding: 0 16px;
  }

  .metric__status-value {
    gap: 7px;
    font-size: 12px;
  }

  .activity__heading h2 {
    font-size: 21px;
  }

  .activity-panel {
    min-height: 0;
    padding: 18px 17px;
  }

  .row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 6px 8px;
    min-height: 62px;
  }

  .row__main {
    grid-column: 1;
    grid-row: 1 / span 2;
  }

  .row__meta {
    grid-column: 2;
    grid-row: 1;
    max-width: 125px;
    justify-self: end;
  }

  .row :deep(.el-tag) {
    grid-column: 2;
    grid-row: 2;
    justify-self: end;
  }

  .state {
    min-height: 174px;
  }

  .empty-state {
    min-height: 180px;
  }
}
</style>
