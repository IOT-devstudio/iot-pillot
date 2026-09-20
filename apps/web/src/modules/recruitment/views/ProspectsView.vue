<script setup lang="ts">
/**
 * 意向成员列表。
 *
 * 按关键词 + 状态筛选；点击姓名跳详情；动作按当前状态决定（防止重复/越权发信）。
 * 数据来自内存 store，列表与详情共享同一份响应式状态。
 */
import { computed, ref } from "vue";
import { useRouter } from "vue-router";

import AppHeader from "@/components/common/AppHeader.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { formatDateTime } from "../fixture";
import type { ProspectStatus } from "../status";
import { PROSPECT_STATUSES, prospectStatusMeta } from "../status";
import { useProspects } from "../store";
import EmailSendDialog from "../components/EmailSendDialog.vue";
import StatusTag from "../components/StatusTag.vue";
import { useProspectActions } from "../composables/useProspectActions";

const router = useRouter();
const prospects = useProspects();

const keyword = ref("");
const status = ref<ProspectStatus | "">("");

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return prospects.value.filter((p) => {
    const matchKeyword =
      !kw ||
      p.name.toLowerCase().includes(kw) ||
      p.email.toLowerCase().includes(kw) ||
      p.className.toLowerCase().includes(kw);
    const matchStatus = !status.value || p.status === status.value;
    return matchKeyword && matchStatus;
  });
});

/** 状态统计条：按状态计数，点击可切换筛选 */
const stats = computed(() =>
  PROSPECT_STATUSES.map((s) => ({
    status: s,
    label: prospectStatusMeta(s).label,
    tone: prospectStatusMeta(s).tagType,
    count: prospects.value.filter((p) => p.status === s).length,
  })),
);

function toggleStatusFilter(s: ProspectStatus) {
  status.value = status.value === s ? "" : s;
}

function resetFilters() {
  keyword.value = "";
  status.value = "";
}

function goDetail(id: string) {
  router.push(`/recruitment/prospects/${id}`);
}

const {
  visible,
  target,
  type,
  title,
  defaultSubject,
  defaultBody,
  openEmail,
  onSend,
  markPassed,
} = useProspectActions();
</script>

<template>
  <div class="page">
    <AppHeader />

    <main class="page__body">
      <PageHeader
        eyebrow="招新管理 · 意向档案"
        title="意向成员"
        :meta="`共 ${prospects.length} 人`"
      />

      <section class="stat-strip" aria-label="按状态筛选">
        <button
          v-for="s in stats"
          :key="s.status"
          type="button"
          class="stat"
          :class="[`stat--${s.tone}`, { 'is-active': status === s.status }]"
          :aria-pressed="status === s.status"
          @click="toggleStatusFilter(s.status)"
        >
          <span class="stat__label">{{ s.label }}</span>
          <span class="stat__count">{{ s.count }}</span>
        </button>
        <div class="stat stat--total">
          <span class="stat__label">合计</span>
          <span class="stat__count">{{ prospects.length }}</span>
        </div>
      </section>

      <section class="panel table-panel">
        <div class="toolbar">
          <div class="toolbar__filters">
            <el-input
              v-model="keyword"
              placeholder="搜索姓名 / 邮箱 / 班级"
              clearable
              style="width: 240px"
            />
            <el-select v-model="status" placeholder="全部状态" clearable style="width: 160px">
              <el-option
                v-for="s in PROSPECT_STATUSES"
                :key="s"
                :label="prospectStatusMeta(s).label"
                :value="s"
              />
            </el-select>
          </div>
          <el-button text @click="resetFilters">重置筛选</el-button>
        </div>

        <el-table :data="filtered">
          <el-table-column label="姓名" min-width="120">
            <template #default="{ row }">
              <el-link type="primary" @click="goDetail(row.id)">{{ row.name }}</el-link>
            </template>
          </el-table-column>
          <el-table-column prop="email" label="邮箱" min-width="200" />
          <el-table-column prop="className" label="班级" min-width="120" />
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <StatusTag :status="row.status" />
            </template>
          </el-table-column>
          <el-table-column label="报名时间" min-width="170">
            <template #default="{ row }">{{ formatDateTime(row.registeredAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <template v-if="row.status === 'ready_first'">
                <el-button link type="primary" @click="markPassed(row)">通过一面</el-button>
                <el-button link type="danger" @click="openEmail(row, 'reject')">拒录</el-button>
              </template>
              <template v-else-if="row.status === 'first_passed'">
                <el-button link type="primary" @click="openEmail(row, 'pass')">发通过邮件</el-button>
              </template>
              <template v-else-if="row.status === 'ready_second'">
                <el-button link type="primary" @click="openEmail(row, 'pass')">录用</el-button>
                <el-button link type="danger" @click="openEmail(row, 'reject')">拒录</el-button>
              </template>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
        </el-table>
      </section>
    </main>

    <EmailSendDialog
      v-model="visible"
      :type="type"
      :recipient-name="target?.name ?? ''"
      :title="title"
      :default-subject="defaultSubject"
      :default-body="defaultBody"
      @send="onSend"
    />
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
}

.page__body {
  display: flex;
  flex-direction: column;
  gap: 28px;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

/* —— 状态统计条：发丝网格 + 左侧色条，点击即筛选 —— */
.stat-strip {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 1px;
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 2px;
  background: var(--line);
}

.stat {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px 20px;
  border: 0;
  background: var(--surface);
  cursor: pointer;
  text-align: left;
  font: inherit;
  transition: background 160ms ease;
}

.stat::after {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 3px;
  background: var(--tone);
  content: "";
  opacity: 0;
  transition: opacity 160ms ease;
}

.stat:hover {
  background: rgb(16 43 78 / 3%);
}

.stat.is-active {
  background: var(--tone-bg);
}

.stat.is-active::after {
  opacity: 1;
}

.stat__label {
  color: var(--ink-soft);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.stat__count {
  font-family: var(--font-mono);
  font-size: 30px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1;
  color: var(--ink);
}

.stat.is-active .stat__count {
  color: var(--tone);
}

/* 每种状态的色调（tagType 复用为 tone） */
.stat--info { --tone: #52657b; --tone-bg: rgb(82 101 123 / 8%); }
.stat--warning { --tone: #a8751a; --tone-bg: rgb(168 117 26 / 9%); }
.stat--primary { --tone: #164d80; --tone-bg: rgb(22 77 128 / 8%); }
.stat--success { --tone: #2e8b57; --tone-bg: rgb(46 139 87 / 8%); }
.stat--danger { --tone: #b84235; --tone-bg: rgb(184 66 53 / 8%); }

.stat--total {
  cursor: default;
  background: #faf7f0;
}

.stat--total:hover {
  background: #faf7f0;
}

.stat--total .stat__count {
  color: var(--ink-soft);
}

/* —— 表格面板 —— */
.table-panel {
  overflow: hidden;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--line);
}

.toolbar__filters {
  display: flex;
  gap: 12px;
}

.muted {
  color: var(--ink-faint);
  font-size: 13px;
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

@media (max-width: 900px) {
  .stat-strip {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 560px) {
  .stat-strip {
    grid-template-columns: repeat(2, 1fr);
  }

  .toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar__filters {
    flex-direction: column;
  }

  .toolbar__filters :deep(.el-input),
  .toolbar__filters :deep(.el-select) {
    width: 100% !important;
  }
}
</style>
