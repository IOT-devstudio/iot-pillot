<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute } from "vue-router";
import type { InvitationView } from "@iot-pillot/shared-types";
import {
  listInvitations,
  sendInvitation,
  revokeInvitation,
} from "@/api/invitations";
import { requestWithSession } from "@/api/session-request";
import type { RecruitmentApplicationDetail } from "@/api/recruitment";
import { readAuthSession } from "@/auth/session";
import RecruitmentMailFields from "../components/RecruitmentMailFields.vue";
import { dateTime, errorMessage } from "../workflow";
import "../recruitment.css";
const route = useRoute(),
  userId = readAuthSession()?.user_id;
const items = ref<InvitationView[]>([]),
  page = ref(1),
  total = ref(0),
  applicationId = ref<number>();
const target = ref<RecruitmentApplicationDetail>(),
  loading = ref(false),
  busy = ref(false),
  error = ref(""),
  success = ref("");
const sendOpen = ref(false),
  revokeTarget = ref<InvitationView>(),
  days = ref(7);
onBeforeRouteLeave(() => !busy.value);
onBeforeRouteUpdate(() => !busy.value);
const mail = ref<{ template_id?: number; vars: Record<string, string> }>({
  vars: {},
});
let generation = 0;
async function load() {
  const current = ++generation;
  loading.value = true;
  error.value = "";
  target.value = undefined;
  try {
    const [records, application] = await Promise.all([
      listInvitations(page.value, applicationId.value),
      applicationId.value
        ? requestWithSession<RecruitmentApplicationDetail>(
            `/api/v1/admin/recruitment/applications/${applicationId.value}`,
          )
        : Promise.resolve(undefined),
    ]);
    if (current !== generation) return;
    items.value = records.items ?? [];
    total.value = records.total;
    target.value = application;
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
function readFilter() {
  sendOpen.value = false;
  revokeTarget.value = undefined;
  success.value = "";
  const id = Number(route.query.application_id);
  applicationId.value = Number.isSafeInteger(id) && id > 0 ? id : undefined;
  page.value = 1;
  void load();
}
async function send() {
  if (
    busy.value ||
    loading.value ||
    !target.value ||
    target.value.application.status === "rejected" ||
    !mail.value.template_id ||
    userId === undefined
  )
    return;
  busy.value = true;
  error.value = "";
  success.value = "";
  try {
    await sendInvitation(
      {
        application_id: target.value.application.id,
        template_id: mail.value.template_id,
        vars: mail.value.vars,
        expires_in_days: days.value,
      },
      userId,
    );
    success.value = "邀请邮件已发送。";
    sendOpen.value = false;
    await load();
  } catch (e) {
    error.value = errorMessage(e);
  } finally {
    busy.value = false;
  }
}
async function revoke() {
  if (busy.value || !revokeTarget.value || userId === undefined) return;
  busy.value = true;
  error.value = "";
  success.value = "";
  try {
    await revokeInvitation(revokeTarget.value.id, userId);
    revokeTarget.value = undefined;
    success.value = "邀请已撤销，原链接将无法接受。";
    await load();
  } catch (e) {
    error.value = errorMessage(e);
  } finally {
    busy.value = false;
  }
}
watch(() => route.query.application_id, readFilter);
onMounted(readFilter);
</script>
<template>
  <main class="recruitment-page">
    <p class="eyebrow">RECRUITMENT / INVITATIONS</p>
    <h1>邀请管理</h1>
    <p class="intro">
      从申请审核页选择申请人发送邀请，在这里查看投递与接受状态。
    </p>
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
    />
    <div class="recruitment-toolbar">
      <RouterLink to="/recruitment/applications">选择申请人</RouterLink
      ><RouterLink to="/recruitment/candidates">查看候选人</RouterLink
      ><el-button :disabled="busy" :loading="loading" @click="load"
        >刷新</el-button
      ><RouterLink v-if="applicationId" to="/recruitment/invitations"
        >查看全部邀请</RouterLink
      >
    </div>
    <section v-if="target" class="recruitment-card">
      <h2>{{ target.application.name }} · 申请 #{{ target.application.id }}</h2>
      <p>
        {{ target.application.email || "使用账号绑定邮箱" }} ·
        {{ target.application.direction_code }}
      </p>
      <el-button
        type="primary"
        :disabled="busy || loading || target.application.status === 'rejected'"
        @click="
          mail = { vars: {} };
          days = 7;
          sendOpen = true;
        "
        >发送 / 重发邀请</el-button
      >
      <p>重发会复用待接受邀请，原有效期不变。过期后请先撤销，再重新发送。</p>
    </section>
    <el-table
      v-loading="loading"
      :data="items"
      empty-text="暂无邀请记录"
      style="width: 100%"
    >
      <el-table-column prop="id" label="邀请编号" width="100" /><el-table-column
        prop="application_id"
        label="申请编号"
        width="100"
      />
      <el-table-column label="状态" width="110"
        ><template #default="{ row }">{{
          row.status === "accepted"
            ? "已接受"
            : row.status === "revoked"
              ? "已撤销"
              : "待接受"
        }}</template></el-table-column
      >
      <el-table-column label="邮件投递" min-width="180"
        ><template #default="{ row }">{{
          row.email_sent_at ? dateTime(row.email_sent_at) : "尚未投递成功"
        }}</template></el-table-column
      >
      <el-table-column label="有效期至" min-width="180"
        ><template #default="{ row }">{{
          dateTime(row.expires_at)
        }}</template></el-table-column
      >
      <el-table-column label="接受时间" min-width="180"
        ><template #default="{ row }">{{
          dateTime(row.accepted_at)
        }}</template></el-table-column
      >
      <el-table-column label="操作" width="150"
        ><template #default="{ row }"
          ><RouterLink
            :to="{
              path: '/recruitment/invitations',
              query: { application_id: row.application_id },
            }"
            >申请</RouterLink
          >
          <el-button
            v-if="row.status === 'pending'"
            link
            type="danger"
            :disabled="busy || loading"
            @click="revokeTarget = row"
            >撤销</el-button
          ></template
        ></el-table-column
      >
    </el-table>
    <el-pagination
      v-model:current-page="page"
      :total="total"
      :page-size="20"
      layout="prev, pager, next"
      :disabled="loading || busy"
      @current-change="load"
    />
    <el-dialog
      v-model="sendOpen"
      title="确认发送邀请"
      width="min(620px, 94vw)"
      :close-on-click-modal="false"
      :close-on-press-escape="!busy"
      :show-close="!busy"
      destroy-on-close
    >
      <p>
        收件人：{{ target?.application.name }}（{{
          target?.application.email || "账号绑定邮箱"
        }}）
      </p>
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <el-form label-position="top"
        ><el-form-item label="新邀请有效天数"
          ><el-input-number
            v-model="days"
            :min="1"
            :max="90"
            :disabled="busy" /></el-form-item
        ><RecruitmentMailFields
          v-if="sendOpen"
          type="invitation"
          :disabled="busy"
          @change="mail = $event"
      /></el-form>
      <template #footer
        ><el-button :disabled="busy" @click="sendOpen = false">取消</el-button
        ><el-button
          type="primary"
          :loading="busy"
          :disabled="!mail.template_id"
          @click="send"
          >确认发送邮件</el-button
        ></template
      >
    </el-dialog>
    <el-dialog
      :model-value="!!revokeTarget"
      title="撤销邀请"
      width="min(480px, 94vw)"
      :close-on-click-modal="false"
      :show-close="!busy"
      :close-on-press-escape="!busy"
      @close="revokeTarget = undefined"
      ><p>
        撤销申请 #{{ revokeTarget?.application_id }}
        的邀请后，原链接将立即失效。确认继续？
      </p>
      <el-alert
        v-if="error"
        :title="error"
        type="error"
        :closable="false"
      /><template #footer
        ><el-button :disabled="busy" @click="revokeTarget = undefined"
          >取消</el-button
        ><el-button type="danger" :loading="busy" @click="revoke"
          >确认撤销</el-button
        ></template
      ></el-dialog
    >
  </main>
</template>
