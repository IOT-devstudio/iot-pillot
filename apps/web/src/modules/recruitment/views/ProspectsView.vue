<script setup lang="ts">
/**
 * 意向成员列表。
 *
 * 按关键词 + 状态筛选；点击姓名跳详情；动作按当前状态决定（防止重复/越权发信）。
 * 数据来自内存 store，列表与详情共享同一份响应式状态。
 */
import { computed, ref } from "vue";
import { useRouter } from "vue-router";

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
  <div class="prospects-page">
    <div class="page-head">
      <div>
        <h2>意向成员</h2>
        <p class="muted">两轮面试 · 按状态推进，邮件按当前状态筛选发送，防止重复</p>
      </div>
    </div>

    <el-card shadow="never">
      <div class="filters">
        <el-input
          v-model="keyword"
          placeholder="搜索姓名 / 邮箱 / 班级"
          clearable
          style="width: 220px"
        />
        <el-select v-model="status" placeholder="全部状态" clearable style="width: 160px">
          <el-option
            v-for="s in PROSPECT_STATUSES"
            :key="s"
            :label="prospectStatusMeta(s).label"
            :value="s"
          />
        </el-select>
        <el-button @click="resetFilters">重置</el-button>
      </div>

      <el-table :data="filtered" stripe>
        <el-table-column label="姓名" min-width="120">
          <template #default="{ row }">
            <el-link type="primary" @click="goDetail(row.id)">{{ row.name }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="className" label="班级" min-width="120" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <StatusTag :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column label="报名时间" min-width="160">
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
    </el-card>

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
.prospects-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.page-head h2 {
  margin: 0;
}
.muted {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.filters {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
</style>
