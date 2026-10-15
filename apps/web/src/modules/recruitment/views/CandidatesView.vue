<script setup lang="ts">
import { onMounted, ref } from "vue";
import type { CandidateView, CandidateStatus } from "@iot-pillot/shared-types";
import { listCandidates } from "@/api/invitations";
import { candidateLabels, dateTime, errorMessage } from "../workflow";
import "../recruitment.css";
const items = ref<CandidateView[]>([]),
  status = ref<CandidateStatus | "">(""),
  page = ref(1),
  total = ref(0),
  loading = ref(false),
  error = ref("");
let generation = 0;
async function load() {
  const current = ++generation;
  loading.value = true;
  error.value = "";
  try {
    const result = await listCandidates(page.value, status.value || undefined);
    if (current !== generation) return;
    items.value = result.items ?? [];
    total.value = result.total;
  } catch (e) {
    if (current === generation) {
      error.value = errorMessage(e);
      items.value = [];
      total.value = 0;
    }
  } finally {
    if (current === generation) loading.value = false;
  }
}
onMounted(load);
</script>
<template>
  <main class="recruitment-page">
    <p class="eyebrow">RECRUITMENT / CANDIDATES</p>
    <h1>候选人处理</h1>
    <p class="intro">
      接受邀请的申请人会进入面试阶段。进入详情查看时间线、发送结果邮件或确认录用。
    </p>
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      role="alert"
    />
    <div class="recruitment-toolbar">
      <el-select
        v-model="status"
        aria-label="候选人状态"
        @change="
          page = 1;
          load();
        "
        ><el-option label="全部状态" value="" /><el-option
          v-for="(label, key) in candidateLabels"
          :key="key"
          :value="key"
          :label="label" /></el-select
      ><el-button :loading="loading" @click="load">刷新</el-button
      ><RouterLink to="/recruitment/invitations">邀请管理</RouterLink>
    </div>
    <el-table v-loading="loading" :data="items" empty-text="暂无候选人"
      ><el-table-column
        prop="name"
        label="姓名"
        min-width="120"
      /><el-table-column
        prop="email"
        label="邮箱"
        min-width="220"
      /><el-table-column
        prop="direction"
        label="方向"
        min-width="120"
      /><el-table-column label="状态" width="130"
        ><template #default="{ row }">{{
          candidateLabels[row.status as CandidateStatus]
        }}</template></el-table-column
      ><el-table-column label="更新时间" min-width="180"
        ><template #default="{ row }">{{
          dateTime(row.updated_at)
        }}</template></el-table-column
      ><el-table-column label="操作" width="100"
        ><template #default="{ row }"
          ><RouterLink :to="`/recruitment/candidates/${row.id}`"
            >处理详情</RouterLink
          ></template
        ></el-table-column
      ></el-table
    >
    <el-pagination
      v-model:current-page="page"
      :total="total"
      :page-size="20"
      layout="prev, pager, next"
      :disabled="loading"
      @current-change="load"
    />
  </main>
</template>
