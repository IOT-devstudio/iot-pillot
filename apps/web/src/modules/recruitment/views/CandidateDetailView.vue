<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute } from "vue-router";
import type {
  CandidateDetailView,
  AdvanceCandidateInput,
} from "@iot-pillot/shared-types";
import { getCandidate, advanceCandidate } from "@/api/invitations";
import { convertCandidateToMember } from "@/api/studio-members";
import { readAuthSession } from "@/auth/session";
import RecruitmentMailFields from "../components/RecruitmentMailFields.vue";
import {
  actionLabels,
  availableActions,
  candidateLabels,
  dateTime,
  errorMessage,
} from "../workflow";
import {
  EMPTY_CONVERT_DRAFT,
  canConvertToMember,
  convertDraftErrors,
  toConvertInput,
  type ConvertDraft,
} from "../convert";
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

/**
 * 「转为工作室成员」（#197）。
 *
 * 只在 hired 时给入口：其他状态后端会拒绝，但把注定失败的按钮摆出来
 * 只会让管理员点一次、读一遍错误、再退回来。
 * 姓名与方向不需要填 —— 后端从申请带过来。
 */
const convertOpen = ref(false),
  convertBusy = ref(false),
  convertError = ref(""),
  convertDraft = ref<ConvertDraft>({ ...EMPTY_CONVERT_DRAFT }),
  convertFieldErrors = ref<Record<string, string>>({});
const canConvert = computed(
  () => detail.value !== undefined && canConvertToMember(detail.value.candidate.status),
);
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
function openConvert() {
  convertDraft.value = { ...EMPTY_CONVERT_DRAFT };
  convertFieldErrors.value = {};
  convertError.value = "";
  convertOpen.value = true;
}

async function submitConvert() {
  if (convertBusy.value || !detail.value) return;
  convertFieldErrors.value = convertDraftErrors(convertDraft.value);
  if (Object.keys(convertFieldErrors.value).length > 0) return;

  convertBusy.value = true;
  convertError.value = "";
  try {
    const member = await convertCandidateToMember(
      toConvertInput(detail.value.candidate.id, convertDraft.value),
    );
    convertOpen.value = false;
    // 转入成功后候选人状态不变（本票明确不改招新状态机），
    // 所以这里只提示，并指路到成员名录。
    success.value = `已转为工作室成员（成员 #${member.id}）。可在「成员名录」查看。`;
  } catch (e) {
    // 非 hired / 已转入 / 缺字段都是 400，message 可直接展示。
    convertError.value = errorMessage(e);
  } finally {
    convertBusy.value = false;
  }
}

watch(
  () => route.params.id,
  () => {
    action.value = undefined;
    success.value = "";
    convertOpen.value = false;
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
        <!--
          转成员入口只在 hired 出现（#197）：其他状态后端会拒绝，
          把注定失败的按钮摆出来只会让人点一次、读一遍错误。
        -->
        <div v-if="canConvert" class="recruitment-toolbar">
          <el-button type="success" :disabled="busy || loading" @click="openConvert"
            >转为工作室成员</el-button
          >
        </div>
        <p v-if="canConvert">
          转入后会新建一份成员档案，候选人状态保持不变；同一人只能转入一次。
        </p>
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
    <!-- 转成员弹窗（#197）：姓名与方向由后端从申请带过来，不需要填 -->
    <el-dialog
      v-model="convertOpen"
      title="转为工作室成员"
      width="min(560px, 94vw)"
      :close-on-click-modal="false"
      :close-on-press-escape="!convertBusy"
      :show-close="!convertBusy"
      destroy-on-close
    >
      <p>{{ detail?.candidate.name }} · {{ detail?.candidate.email }}</p>
      <p>
        姓名与方向会从招新申请带过来。以下三项在招新数据里没有，需要在这里补全。
      </p>
      <el-alert
        v-if="convertError"
        :title="convertError"
        type="error"
        :closable="false"
        role="alert"
      />
      <el-form label-position="top">
        <el-form-item
          label="出生年"
          required
          :error="convertFieldErrors.birth_year"
          ><el-input
            v-model="convertDraft.birth_year"
            inputmode="numeric"
            placeholder="如 2003（与姓名共同构成唯一标识）"
            :disabled="convertBusy"
        /></el-form-item>
        <el-form-item label="届别" required :error="convertFieldErrors.cohort"
          ><el-input
            v-model="convertDraft.cohort"
            placeholder="如 2024 级"
            :disabled="convertBusy"
        /></el-form-item>
        <el-form-item
          label="工作室职能"
          required
          :error="convertFieldErrors.role_in_studio"
          ><el-input
            v-model="convertDraft.role_in_studio"
            placeholder="如 前端成员"
            :disabled="convertBusy"
        /></el-form-item>
        <el-form-item label="实习去向" :error="convertFieldErrors.internship"
          ><el-input
            v-model="convertDraft.internship"
            placeholder="选填"
            :disabled="convertBusy"
        /></el-form-item>
        <el-form-item label="就业去向" :error="convertFieldErrors.employment"
          ><el-input
            v-model="convertDraft.employment"
            placeholder="选填"
            :disabled="convertBusy"
        /></el-form-item>
      </el-form>
      <template #footer
        ><el-button :disabled="convertBusy" @click="convertOpen = false"
          >取消</el-button
        ><el-button
          type="primary"
          :loading="convertBusy"
          :disabled="convertBusy"
          @click="submitConvert"
          >确认转入</el-button
        ></template
      >
    </el-dialog>

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
