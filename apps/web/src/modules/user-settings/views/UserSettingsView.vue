<script setup lang="ts">
/**
 * 个人设置页 `/user/settings`。
 *
 * 接线 useUserSettings（拉取 / 保存）与 UserSettingsForm（只读字段 + 表单 + 按钮）。
 * 视图层只负责：骨架屏、错误态、空态、保存结果反馈。
 *
 * 字段契约来自 issue #58，最终字段形状以 #57 后端 PR 落地为准。
 */
import { onMounted } from "vue";

import BackToDashboard from "@/components/common/BackToDashboard.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { nextFormFromProfile, useUserSettings } from "../composables/useUserSettings";
import UserSettingsForm from "../components/UserSettingsForm.vue";

const {
  profile,
  form,
  loading,
  loadError,
  loadUnauthorized,
  saving,
  saveError,
  needsCompletion,
  load,
  save,
} = useUserSettings();

onMounted(load);

/** 「重置」=把表单拉回 profile 当前值，放弃本地编辑 */
function resetForm(): void {
  if (profile.value !== null) {
    const next = nextFormFromProfile(profile.value);
    form.class = next.class;
    form.studentId = next.studentId;
    form.qq = next.qq;
    form.direction = next.direction;
  }
}
</script>

<template>
  <div class="page">
    <main class="page__body">
      <BackToDashboard />

      <PageHeader
        eyebrow="iot-pillot · 用户侧"
        title="个人设置"
        description="补全与维护你自己的资料。姓名、邮箱为登录凭据，本期不在此处变更。"
      />

      <!-- 加载中：用骨架屏而非「正在加载…」单行，避开从 50px 跳到 220px 的重排 -->
      <section v-if="loading" class="panel panel--pad" role="status" aria-label="正在加载资料">
        <div class="skeleton skeleton--title" />
        <div class="skeleton skeleton--row" />
        <div class="skeleton skeleton--row" />
        <div class="skeleton skeleton--field" />
        <div class="skeleton skeleton--field" />
        <div class="skeleton skeleton--field" />
      </section>

      <!-- 加载失败：401 不出「重试」，要重新登录才有意义 -->
      <section
        v-else-if="loadError"
        class="panel panel--pad settings-error"
        role="alert"
      >
        <p class="settings-error__msg">{{ loadError }}</p>
        <el-button
          v-if="!loadUnauthorized"
          link
          type="primary"
          :disabled="loading"
          @click="load"
        >
          重试
        </el-button>
      </section>

      <UserSettingsForm
        v-else-if="profile !== null"
        :profile="profile"
        :form="form"
        :needs-completion="needsCompletion"
        :saving="saving"
        @submit="save"
        @cancel="resetForm"
      />

      <!-- 保存失败的局部提示：toast 容易被错过，把这条放在表单下面兜底 -->
      <p v-if="saveError" class="settings-save-error" role="alert">
        {{ saveError }}
      </p>
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
  gap: 28px;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

/* —— 骨架屏 —— */
.skeleton {
  background: linear-gradient(
    90deg,
    rgba(0, 0, 0, 0.06) 25%,
    rgba(0, 0, 0, 0.1) 37%,
    rgba(0, 0, 0, 0.06) 63%
  );
  background-size: 400% 100%;
  border-radius: 6px;
  animation: skeleton-shimmer 1.4s ease infinite;
}

.skeleton--title {
  width: 220px;
  height: 18px;
  margin-bottom: 20px;
}

.skeleton--row {
  width: 50%;
  height: 14px;
  margin-bottom: 10px;
}

.skeleton--field {
  width: 100%;
  height: 36px;
  margin-bottom: 12px;
}

@keyframes skeleton-shimmer {
  0% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0 50%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .skeleton {
    animation: none;
    background: rgba(0, 0, 0, 0.06);
  }
}

/* —— 错误态 —— */
.settings-error {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  color: #a33d32;
}

.settings-error__msg {
  margin: 0;
  font-size: 13px;
}

.settings-save-error {
  margin: 0;
  padding: 12px 16px;
  color: #a33d32;
  border-left: 3px solid #a33d32;
  border-radius: 6px;
  background: #fdf3f1;
  font-size: 13px;
}
</style>