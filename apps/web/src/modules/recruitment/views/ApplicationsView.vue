<script setup lang="ts">
/**
 * 招新申请审核（issue #89 管理端部分）。
 *
 * 真实接口（均需管理员，见 api/recruitment.ts）：
 *   - 申请列表 GET /admin/recruitment/applications（?status= 筛选）
 *   - 审核流转 PATCH .../applications/:id/review
 *     applied→reviewing，reviewing→accepted / rejected；非法目标后端拒绝
 *   - 轮次开关 GET/PATCH /admin/recruitment/rounds
 *   - 方向开关 PATCH /admin/recruitment/directions/:code
 *
 * 本页处理基础审核；邀请记录链接进入 #91 的真实邀请与候选人流程。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import { AuthRequestError } from "@/api/auth";
import {
  fetchAdminApplications,
  fetchAdminRounds,
  fetchDirections,
  reviewApplication,
  setDirectionOpen,
  setRoundAccepting,
  type ApplicationStatus,
  type RecruitmentApplication,
  type RecruitmentDirection,
  type RecruitmentRound,
  type ReviewTarget,
} from "@/api/recruitment";
import { presentationOf } from "@/modules/home/directionPresentation";
import { dateTime as formatDateTime } from "../workflow";

const STATUS_LABELS: Record<ApplicationStatus, string> = {
  applied: "待审核",
  reviewing: "审核中",
  accepted: "已通过",
  rejected: "未通过",
};

const applications = ref<RecruitmentApplication[]>([]);
const directions = ref<RecruitmentDirection[]>([]);
const rounds = ref<RecruitmentRound[]>([]);

/**
 * 种子方向的兜底清单（review 反馈的自锁 bug）。
 *
 * 此前方向开关条直接渲染用户端接口的结果，而该接口只返回**开放中**的
 * 方向（ListActiveDirections）——管理员把三个方向全关后接口返回空，
 * 整个开关区块消失，页面上再也无处开启（把自己锁在门外）。
 *
 * 现改为：三条种子方向永远渲染开关条，接口没返回的即为「关闭」
 * （接口语义：不返回 = 未开放）。PATCH 的合法性由后端白名单兜底。
 * 方向支持动态新增（#128）后，此处应换成管理端全量接口。
 */
const SEED_DIRECTION_NAMES: Record<string, string> = {
  "front-end": "前端开发",
  "back-end": "后端开发",
  agent: "智能体",
};

/** 展示用方向列表 = 接口结果 ∪ 种子兜底（缺失即关闭，按 sort_order 排序） */
const displayDirections = computed(() => {
  const byCode = new Map(
    directions.value.map((direction) => [direction.code, direction]),
  );
  for (const [code, name] of Object.entries(SEED_DIRECTION_NAMES)) {
    if (!byCode.has(code)) {
      byCode.set(code, { id: 0, code, name, open: false, sort_order: 0 });
    }
  }
  return [...byCode.values()].sort((a, b) => a.sort_order - b.sort_order);
});
const loading = ref(true);
const loadError = ref("");
const statusFilter = ref<ApplicationStatus | "">("");
/** 正在提交审核/配置的资源 id，用于禁用按钮防重复提交 */
const actingId = ref<number | null>(null);

const filtered = computed(() =>
  statusFilter.value
    ? applications.value.filter((item) => item.status === statusFilter.value)
    : applications.value,
);

const statusCounts = computed(() => {
  const counts = {
    applied: 0,
    reviewing: 0,
    accepted: 0,
    rejected: 0,
  } as Record<ApplicationStatus, number>;
  for (const item of applications.value) {
    counts[item.status] += 1;
  }
  return counts;
});

function directionName(code: string): string {
  return (
    displayDirections.value.find((direction) => direction.code === code)
      ?.name ?? presentationOf(code).headline
  );
}

function describeError(error: unknown, fallback: string): string {
  return error instanceof AuthRequestError ? error.message : fallback;
}

async function loadAll(): Promise<void> {
  loading.value = true;
  loadError.value = "";
  try {
    const [applicationPage, directionList, roundList] = await Promise.all([
      fetchAdminApplications(),
      fetchDirections(),
      fetchAdminRounds(),
    ]);
    applications.value = applicationPage.items;
    directions.value = directionList.items;
    rounds.value = roundList.items;
  } catch (error) {
    loadError.value = describeError(error, "暂时无法加载申请数据，请稍后重试");
  } finally {
    loading.value = false;
  }
}

onMounted(loadAll);

/** 审核动作：只按当前状态渲染合法目标，后端仍是最终校验 */
async function review(
  application: RecruitmentApplication,
  target: ReviewTarget,
): Promise<void> {
  if (actingId.value !== null) return;
  actingId.value = application.id;
  try {
    const message = await reviewApplication(application.id, target);
    ElMessage.success(message);
    await loadAll();
  } catch (error) {
    ElMessage.error(describeError(error, "审核失败，请稍后重试"));
  } finally {
    actingId.value = null;
  }
}

/**
 * 方向/轮次开关直接写库并即时作用于用户端（首页方向卡立刻变「暂未开放」，
 * 用户无法再报名）——review 反馈误触把所有方向都关了，所以必须二次确认。
 */
async function toggleDirection(direction: RecruitmentDirection): Promise<void> {
  const next = !direction.open;
  try {
    await ElMessageBox.confirm(
      next
        ? `开启「${direction.name}」后，用户端首页会立即出现该方向并可报名。`
        : `关闭「${direction.name}」后，用户端首页会立即把它变成「暂未开放」，用户无法报名；已提交的申请不受影响。`,
      next ? "开启方向报名" : "关闭方向报名",
      {
        confirmButtonText: next ? "确认开启" : "确认关闭",
        cancelButtonText: "取消",
        type: next ? "info" : "warning",
      },
    );
  } catch {
    return; // 用户取消
  }
  try {
    const message = await setDirectionOpen(direction.code, next);
    ElMessage.success(message);
    await loadAll();
  } catch (error) {
    ElMessage.error(describeError(error, "方向配置失败，请稍后重试"));
  }
}

async function toggleRound(round: RecruitmentRound): Promise<void> {
  const next = !round.accepting;
  try {
    await ElMessageBox.confirm(
      next
        ? `开启「${round.name}」后，该轮次立即接受新报名。`
        : `关闭「${round.name}」后，该轮次立即停止接收新报名；已提交的申请不受影响。`,
      next ? "开启招新轮次" : "关闭招新轮次",
      {
        confirmButtonText: next ? "确认开启" : "确认关闭",
        cancelButtonText: "取消",
        type: next ? "info" : "warning",
      },
    );
  } catch {
    return; // 用户取消
  }
  try {
    const message = await setRoundAccepting(round.id, next);
    ElMessage.success(message);
    await loadAll();
  } catch (error) {
    ElMessage.error(describeError(error, "轮次配置失败，请稍后重试"));
  }
}

function toggleStatusFilter(status: ApplicationStatus): void {
  statusFilter.value = statusFilter.value === status ? "" : status;
}
</script>

<template>
  <div class="page">
    <main class="page__body">
      <PageHeader
        eyebrow="招新管理 · 真实数据"
        title="申请审核"
        description="审核真实报名，选择申请人发送邀请，并在候选人页面继续处理面试结果。"
      />

      <!-- 配置条：轮次与方向开关 -->
      <section v-if="rounds.length" class="config-strip" aria-label="轮次配置">
        <span class="config-strip__label">招新轮次</span>
        <button
          v-for="round in rounds"
          :key="round.id"
          type="button"
          class="config-chip"
          :class="{ 'is-on': round.accepting }"
          :aria-pressed="round.accepting"
          @click="toggleRound(round)"
        >
          第{{ round.round_num }}轮 · {{ round.name }} ——
          {{ round.accepting ? "报名中" : "已停止" }}
        </button>
      </section>

      <section class="config-strip" aria-label="方向开关">
        <span class="config-strip__label">方向开放</span>
        <button
          v-for="direction in displayDirections"
          :key="direction.code"
          type="button"
          class="config-chip"
          :class="{ 'is-on': direction.open }"
          :aria-pressed="direction.open"
          @click="toggleDirection(direction)"
        >
          {{ direction.name }} —— {{ direction.open ? "开放" : "关闭" }}
        </button>
      </section>

      <!-- 状态筛选 -->
      <div
        v-if="!loading && !loadError"
        class="status-filter"
        role="group"
        aria-label="按状态筛选"
      >
        <button
          v-for="status in [
            'applied',
            'reviewing',
            'accepted',
            'rejected',
          ] as const"
          :key="status"
          type="button"
          class="status-filter__chip"
          :class="{ 'is-active': statusFilter === status }"
          :aria-pressed="statusFilter === status"
          @click="toggleStatusFilter(status)"
        >
          {{ STATUS_LABELS[status] }} {{ statusCounts[status] }}
        </button>
        <button
          v-if="statusFilter"
          type="button"
          class="status-filter__chip status-filter__clear"
          @click="statusFilter = ''"
        >
          清除筛选
        </button>
      </div>

      <section
        v-if="loading"
        class="panel panel--pad"
        role="status"
        aria-label="正在加载申请"
      >
        <p>正在加载申请…</p>
      </section>

      <section
        v-else-if="loadError"
        class="panel panel--pad app-error"
        role="alert"
      >
        <p>{{ loadError }}</p>
        <button type="button" class="retry-link" @click="loadAll">
          重试 ↗
        </button>
      </section>

      <p v-else-if="!applications.length" class="empty-hint">
        还没有收到报名 —— 用户在个人首页提交后会出现在这里。
      </p>

      <div v-else class="app-table-wrap">
        <table class="app-table">
          <thead>
            <tr>
              <th>#</th>
              <th>姓名</th>
              <th>邮箱</th>
              <th>班级 / 学号</th>
              <th>方向</th>
              <th>状态</th>
              <th>提交时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="application in filtered" :key="application.id">
              <td>{{ application.id }}</td>
              <td>{{ application.name }}</td>
              <td>{{ application.email }}</td>
              <td>
                {{ application.class || "未填写" }} /
                {{ application.student_id || "未填写" }}
              </td>
              <td>{{ directionName(application.direction_code) }}</td>
              <td>
                <span class="status-pill" :data-status="application.status">
                  {{ STATUS_LABELS[application.status] }}
                </span>
              </td>
              <td>{{ formatDateTime(application.created_at) }}</td>
              <td class="app-table__actions">
                <RouterLink
                  :to="{
                    path: '/recruitment/invitations',
                    query: { application_id: application.id },
                  }"
                  >邀请记录</RouterLink
                >
                <template v-if="application.status === 'applied'">
                  <button
                    type="button"
                    class="action-link"
                    :disabled="actingId === application.id"
                    @click="review(application, 'reviewing')"
                  >
                    开始审核
                  </button>
                </template>
                <template v-else-if="application.status === 'reviewing'">
                  <button
                    type="button"
                    class="action-link action-link--ok"
                    :disabled="actingId === application.id"
                    @click="review(application, 'accepted')"
                  >
                    通过
                  </button>
                  <button
                    type="button"
                    class="action-link action-link--bad"
                    :disabled="actingId === application.id"
                    @click="review(application, 'rejected')"
                  >
                    不通过
                  </button>
                </template>
                <span v-else class="app-table__done">已处理</span>
              </td>
            </tr>
            <tr v-if="!filtered.length">
              <td colspan="8" class="app-table__empty">该状态下暂无申请。</td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>

<style scoped>
.page__body {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

.panel {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: #fff;
}

.panel--pad {
  padding: 20px 22px;
}

.config-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.config-strip__label {
  color: var(--ink-soft);
  font-size: 12px;
  font-weight: 600;
}

.config-chip {
  padding: 7px 14px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: #fff;
  color: var(--ink-soft);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}

.config-chip.is-on {
  border-color: var(--blue);
  color: var(--blue);
}

.config-chip:hover {
  border-color: var(--blue);
}

.status-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.status-filter__chip {
  padding: 6px 12px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: #fff;
  color: var(--ink-soft);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}

.status-filter__chip.is-active {
  border-color: var(--blue);
  color: #fff;
  background: var(--blue);
}

.status-filter__clear {
  border-style: dashed;
}

.app-error {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  color: #a33d32;
}

.retry-link {
  border: 0;
  background: none;
  color: var(--blue);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.empty-hint {
  margin: 0;
  color: var(--ink-soft);
  font-size: 13px;
}

.app-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: #fff;
}

.app-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.app-table th,
.app-table td {
  padding: 11px 14px;
  border-bottom: 1px solid var(--line);
  text-align: left;
  white-space: nowrap;
}

.app-table th {
  color: var(--ink-soft);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  background: #fafbf8;
}

.app-table tr:last-child td {
  border-bottom: 0;
}

.status-pill {
  padding: 3px 10px;
  border: 1px solid var(--line);
  border-radius: 999px;
  font-size: 11px;
  white-space: nowrap;
}

.status-pill[data-status="applied"] {
  border-color: #b98a1d;
  color: #8a6512;
}

.status-pill[data-status="reviewing"] {
  border-color: var(--blue);
  color: var(--blue);
}

.status-pill[data-status="accepted"] {
  border-color: #2f7d43;
  color: #2f7d43;
}

.status-pill[data-status="rejected"] {
  border-color: #a33d32;
  color: #a33d32;
}

.app-table__actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.action-link {
  padding: 0;
  border: 0;
  background: none;
  color: var(--blue);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}

.action-link--ok {
  color: #2f7d43;
}

.action-link--bad {
  color: #a33d32;
}

.action-link:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.app-table__done {
  color: var(--ink-faint, #93a0b1);
  font-size: 12px;
}

.app-table__empty {
  color: var(--ink-soft);
  text-align: center;
}
</style>
