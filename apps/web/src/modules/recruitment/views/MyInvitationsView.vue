<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { onBeforeRouteLeave, useRoute } from "vue-router";
import type {
  MyInvitationView,
  MyCandidateView,
} from "@iot-pillot/shared-types";
import {
  myInvitations,
  myCandidates,
  acceptInvitation,
} from "@/api/invitations";
import { readAuthSession } from "@/auth/session";
import {
  candidateLabels,
  invitationLabel,
  dateTime,
  errorMessage,
} from "../workflow";
import "../recruitment.css";
const route = useRoute();
const userId = readAuthSession()?.user_id;
const invitations = ref<MyInvitationView[]>([]);
const candidates = ref<MyCandidateView[]>([]);
const loading = ref(false),
  loaded = ref(false),
  busy = ref(false),
  error = ref(""),
  success = ref("");
onBeforeRouteLeave(() => !busy.value);
const token = computed(() =>
  typeof route.query.token === "string" ? route.query.token : "",
);
const linked = computed(() =>
  invitations.value.find((item) => item.token === token.value),
);
const visible = computed(() =>
  linked.value
    ? [
        linked.value,
        ...invitations.value.filter((item) => item !== linked.value),
      ]
    : invitations.value,
);
async function load() {
  if (loading.value || userId === undefined) return;
  loading.value = true;
  error.value = "";
  try {
    const [invites, results] = await Promise.all([
      myInvitations(userId),
      myCandidates(userId),
    ]);
    invitations.value = invites.items ?? [];
    candidates.value = results.items ?? [];
    loaded.value = true;
  } catch (e) {
    error.value = errorMessage(e);
    loaded.value = false;
  } finally {
    loading.value = false;
  }
}
async function accept(item: MyInvitationView) {
  if (
    busy.value ||
    loading.value ||
    userId === undefined ||
    item.expired ||
    item.status !== "pending"
  )
    return;
  busy.value = true;
  error.value = "";
  success.value = "";
  try {
    const result = await acceptInvitation(item.token, userId);
    candidates.value = [
      ...candidates.value.filter(
        (candidate) =>
          candidate.application_id !== result.candidate.application_id,
      ),
      result.candidate,
    ];
    success.value = "邀请已接受，已进入候选人流程。";
    await load();
  } catch (e) {
    error.value = errorMessage(e);
  } finally {
    busy.value = false;
  }
}
onMounted(load);
</script>
<template>
  <main class="recruitment-page" :aria-busy="loading || busy">
    <RouterLink to="/user/home">← 返回个人空间</RouterLink>
    <p class="eyebrow">YOUR NEXT STEP</p>
    <h1>邀请与招新结果</h1>
    <p class="intro">
      查看工作室邀请，确认接受后进入面试流程。处理结果以工作室最新记录为准。
    </p>
    <el-alert
      v-if="success"
      :title="success"
      type="success"
      :closable="false"
      role="status"
    />
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      role="alert"
    />
    <el-button :loading="loading" :disabled="busy" @click="load"
      >刷新记录</el-button
    >
    <el-alert
      v-if="loaded && token && !linked"
      title="此邀请不属于当前账号，或链接已失效。请使用收件邮箱对应的账号登录，或联系工作室重新发送邀请。"
      type="warning"
      :closable="false"
    />
    <h2>我的邀请</h2>
    <p v-if="loading" role="status">正在获取最新记录…</p>
    <el-empty
      v-else-if="loaded && !invitations.length"
      description="暂时没有邀请，请耐心等待审核"
    />
    <article v-for="item in visible" :key="item.token" class="recruitment-card">
      <h3>{{ item.direction }} · 申请 #{{ item.application_id }}</h3>
      <el-tag>{{ invitationLabel(item) }}</el-tag>
      <dl>
        <dt>邀请时间</dt>
        <dd>{{ dateTime(item.created_at) }}</dd>
        <dt>有效期至</dt>
        <dd>{{ dateTime(item.expires_at) }}</dd>
        <template v-if="item.accepted_at"
          ><dt>接受时间</dt>
          <dd>{{ dateTime(item.accepted_at) }}</dd></template
        >
      </dl>
      <p v-if="item.status === 'pending' && !item.expired">
        接受后将进入面试阶段，请确认参加该方向的招新。
      </p>
      <el-button
        v-if="item.status === 'pending' && !item.expired"
        type="primary"
        :disabled="busy || loading || !loaded"
        @click="accept(item)"
        >{{ busy ? "正在确认…" : "确认接受邀请" }}</el-button
      >
      <p v-else-if="item.status !== 'accepted'">
        此邀请已无法接受，请联系工作室。
      </p>
    </article>
    <h2>招新结果</h2>
    <el-empty
      v-if="loaded && !candidates.length"
      description="接受邀请后，候选人进度会显示在这里"
    />
    <article
      v-for="item in candidates"
      :key="item.application_id"
      class="recruitment-card"
    >
      <h3>{{ item.direction }} · {{ candidateLabels[item.status] }}</h3>
      <p>
        {{
          item.status === "hired"
            ? "欢迎加入工作室！"
            : item.status === "rejected"
              ? "感谢你的参与与投入，期待未来再次相遇。"
              : item.status === "offered"
                ? "Offer 已发送，请查看收件邮箱并等待录用确认。"
                : "你已进入面试阶段，请留意邮箱中的安排。"
        }}
      </p>
      <dl>
        <dt>进入候选</dt>
        <dd>{{ dateTime(item.created_at) }}</dd>
        <dt>最近更新</dt>
        <dd>{{ dateTime(item.updated_at) }}</dd>
      </dl>
    </article>
  </main>
</template>
