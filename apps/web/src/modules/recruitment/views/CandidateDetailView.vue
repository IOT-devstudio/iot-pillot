<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute } from "vue-router";
import type {
  CandidateDetailView,
  AdvanceCandidateInput,
} from "@iot-pillot/shared-types";
import { getCandidate, advanceCandidate } from "@/api/invitations";
import { readAuthSession } from "@/auth/session";
import RecruitmentMailFields from "../components/RecruitmentMailFields.vue";
import {
  actionLabels,
  availableActions,
  candidateLabels,
  dateTime,
  errorMessage,
} from "../workflow";
import "../recruitment.css";
const route = useRoute(),
  userId = readAuthSession()?.user_id;
const detail = ref<CandidateDetailView>(),
  loading = ref(false),
  busy = ref(false),
  error = ref(""),
  success = ref("");
const action = ref<AdvanceCandidateInput["action"]>(),
  note = ref(""),
  mail = ref<{ template_id?: number; vars: Record<string, string> }>({
    vars: {},
  });
onBeforeRouteLeave(() => !busy.value);
onBeforeRouteUpdate(() => !busy.value);
const actions = computed(() =>
  detail.value ? availableActions(detail.value.candidate.status) : [],
);
let generation = 0;
async function load() {
  const id = Number(route.params.id),
    current = ++generation;
  loading.value = true;
  error.value = "";
  detail.value = undefined;
  try {
    if (!Number.isSafeInteger(id) || id < 1) throw new Error("候选人编号无效");
    const result = await getCandidate(id);
    if (current === generation) detail.value = result;
  } catch (e) {
    if (current === generation) error.value = errorMessage(e);
  } finally {
    if (current === generation) loading.value = false;
  }
}
function choose(value: AdvanceCandidateInput["action"]) {
  action.value = value;
  note.value = "";
  mail.value = { vars: {} };
  error.value = "";
}
async function submit() {
  if (
    busy.value ||
    !detail.value ||
    !action.value ||
    !actions.value.includes(action.value) ||
    userId === undefined ||
    (action.value !== "hire" && !mail.value.template_id)
  )
    return;
  busy.value = true;
  error.value = "";
  success.value = "";
  try {
    await advanceCandidate(
      detail.value.candidate.id,
      {
        action: action.value,
        note: note.value.trim(),
        ...(action.value === "hire" ? {} : mail.value),
      },
      userId,
    );
    success.value =
      action.value === "hire"
        ? "已确认录用。"
        : "邮件已发送，候选人状态已更新。";
    action.value = undefined;
    await load();
  } catch (e) {
    error.value = errorMessage(e);
  } finally {
    busy.value = false;
  }
}
watch(
  () => route.params.id,
  () => {
    action.value = undefined;
    success.value = "";
    void load();
  },
  { immediate: true },
);
</script>
<template>
  <main class="recruitment-page">
    <RouterLink to="/recruitment/candidates">← 候选人列表</RouterLink>
    <h1>候选人详情</h1>
    <el-alert
      v-if="success"
      :title="success"
      type="success"
      :closable="false"
      role="status"
    /><el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      role="alert"
    /><el-button :loading="loading" :disabled="busy" @click="load"
      >刷新记录</el-button
    >
    <template v-if="detail"
      ><section class="recruitment-card">
        <h2>
          {{ detail.candidate.name }}
          <el-tag>{{ candidateLabels[detail.candidate.status] }}</el-tag>
        </h2>
        <dl>
          <dt>邮箱</dt>
          <dd>{{ detail.candidate.email }}</dd>
          <dt>方向</dt>
          <dd>{{ detail.candidate.direction }}</dd>
          <dt>来源申请</dt>
          <dd>
            <RouterLink
              :to="{
                path: '/recruitment/invitations',
                query: { application_id: detail.candidate.application_id },
              }"
              >#{{ detail.candidate.application_id }} · 邀请记录</RouterLink
            >
          </dd>
        </dl>
        <div class="recruitment-toolbar">
          <el-button
            v-for="item in actions"
            :key="item"
            :type="item === 'reject' ? 'danger' : 'primary'"
            :disabled="busy || loading"
            @click="choose(item)"
            >{{ actionLabels[item] }}</el-button
          >
        </div>
        <p v-if="!actions.length">该候选人流程已结束。</p>
      </section>
      <section class="recruitment-card">
        <h2>处理时间线</h2>
        <el-empty
          v-if="!detail.timeline?.length"
          description="暂无时间记录"
        /><el-timeline v-else
          ><el-timeline-item
            v-for="(entry, index) in detail.timeline"
            :key="index"
            :timestamp="dateTime(entry.created_at)"
            ><strong>{{ candidateLabels[entry.to_status] }}</strong>
            <p
              v-if="entry.note"
              style="white-space: pre-wrap; overflow-wrap: anywhere"
            >
              {{ entry.note }}
            </p>
            <small>操作人 #{{ entry.operator_id }}</small></el-timeline-item
          ></el-timeline
        >
      </section></template
    >
    <el-dialog
      :model-value="!!action"
      :title="action ? actionLabels[action] : ''"
      width="min(620px, 94vw)"
      :close-on-click-modal="false"
      :close-on-press-escape="!busy"
      :show-close="!busy"
      destroy-on-close
      @close="action = undefined"
    >
      <p>{{ detail?.candidate.name }} · {{ detail?.candidate.email }}</p>
      <p>
        {{
          action === "hire"
            ? "确认录用后流程结束，此操作不发送邮件。"
            : "确认后将发送对应结果邮件，发送成功后再更新候选人状态。"
        }}
      </p>
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <el-form label-position="top"
        ><el-form-item label="处理备注"
          ><el-input
            v-model="note"
            type="textarea"
            :maxlength="500"
            show-word-limit
            :disabled="busy" /></el-form-item
        ><RecruitmentMailFields
          v-if="action && action !== 'hire'"
          :key="action"
          :type="action === 'send_offer' ? 'offer' : 'rejection'"
          :disabled="busy"
          @change="mail = $event"
      /></el-form>
      <template #footer
        ><el-button :disabled="busy" @click="action = undefined">取消</el-button
        ><el-button
          type="primary"
          :loading="busy"
          :disabled="action !== 'hire' && !mail.template_id"
          @click="submit"
          >确认{{ action === "hire" ? "录用" : "发送" }}</el-button
        ></template
      >
    </el-dialog>
  </main>
</template>
