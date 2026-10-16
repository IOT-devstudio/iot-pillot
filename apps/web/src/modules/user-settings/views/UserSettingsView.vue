<script setup lang="ts">
/**
 * 个人设置页 `/user/settings`。
 *
 * 接线 useUserSettings（拉取 / 保存）与 UserSettingsForm（只读字段 + 表单 + 按钮）。
 * 视图层只负责：骨架屏、错误态、空态、保存结果反馈。
 *
 * 字段契约来自已合并的 #57 / #86，姓名编辑对应 #87。
 */
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { onBeforeRouteLeave, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import { deleteCurrentUser } from "@/api/profile";
import { clearAuthSession } from "@/auth/session";
import { SIGNAL_TRANSITION } from "@/modules/swiss-signal/context";
import { useUserSettings } from "../composables/useUserSettings";
import ChangePasswordCard from "../components/ChangePasswordCard.vue";
import UserSettingsForm from "../components/UserSettingsForm.vue";

const {
  profile,
  form,
  loading,
  loadError,
  loadUnauthorized,
  saving,
  saveError,
  nameError,
  needsCompletion,
  editing,
  fieldErrors,
  latestDirection,
  directionError,
  edit,
  reset,
  cancel,
  retryDirection,
  load,
  save,
  autoSave,
  persistOnUnload,
} = useUserSettings();

/**
 * 离开页面时静默自动保存（review 反馈：改了没存就走应自动保存）。
 * - SPA 内导航（顶栏返回/去选择/UserMenu 等）走路由离开钩子；
 * - 标签页关闭/刷新在 beforeunload 里用 keepalive fetch 兜底
 *   （beforeunload 无法 await，两路都会触发，但 isDirty 在第一次
 *   autoSave 成功后即为 false，第二次自动空转，不会重复写库）。
 */
onBeforeRouteLeave(() => {
  void autoSave();
});

onMounted(() => {
  void load();
  window.addEventListener("beforeunload", handleBeforeUnload);
});

onBeforeUnmount(() => {
  window.removeEventListener("beforeunload", handleBeforeUnload);
  void autoSave();
});

function handleBeforeUnload(): void {
  persistOnUnload();
}

const router = useRouter();
/**
 * 成员方向未定时「去选择」：走擦除转场跳首页方向版块（review 反馈）。
 * 首页 UserHomeView 处理外链 hash，挂载后平滑滚到 #directions。
 */
const transition = inject(
  SIGNAL_TRANSITION,
  async (change: () => unknown) => {
    await change();
  },
);

function pickDirection(): void {
  void transition(() => router.push("/user/home#directions"));
}

/**
 * 自助注销（issue #159 Q1/Q2）：
 * - 创始管理员隐藏整个入口（后端同样拒绝，前端只是体验）；
 * - 确认形式是轻量确认弹窗（Q2 拍板：不要求输用户名/密码）；
 * - 成功后后端已踢全部会话，本地清会话跳登录页带 account-deleted 提示。
 */
const deleting = ref(false);
const isFounding = computed(() => profile.value?.is_founding === true);

async function handleDeleteAccount(): Promise<void> {
  if (deleting.value) return;
  try {
    await ElMessageBox.confirm(
      "注销后将无法再登录，原报名与申请记录会保留为历史存档。确定注销账号？",
      "注销账号",
      {
        type: "warning",
        confirmButtonText: "确认注销",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
      },
    );
  } catch {
    return; // 用户取消
  }

  deleting.value = true;
  try {
    await deleteCurrentUser();
    clearAuthSession();
    await router.replace("/?reason=account-deleted");
  } catch (error: unknown) {
    const message =
      error instanceof Error && error.message
        ? error.message
        : "注销失败，请稍后重试";
    ElMessage.error(message);
    deleting.value = false;
  }
}

</script>

<template>
  <div class="page settings-page">
    <!-- 品牌色柔光晕背景（issue #85 改造）：给液态玻璃卡「可透之物」 -->
    <div class="glass-backdrop" aria-hidden="true"></div>
    <main class="page__body">
      <PageHeader
        eyebrow="iot-pillot · 用户侧"
        title="个人设置"
        description="维护你的姓名与个人资料。账号绑定邮箱不可修改。"
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

      <template v-else-if="profile !== null">
        <UserSettingsForm
          :profile="profile"
          :form="form"
          :needs-completion="needsCompletion"
          :saving="saving"
          :name-error="nameError"
          :editing="editing"
          :field-errors="fieldErrors"
          :latest-direction="latestDirection"
          :direction-error="directionError"
          @submit="save"
          @edit="edit"
          @reset="reset"
          @cancel="cancel"
          @retry="retryDirection"
          @pick-direction="pickDirection"
        />

        <!-- 修改密码入口（issue #85 重构）：一行字样 + 液态玻璃弹窗，
             弹窗内走邮箱验证码流程（复用忘记密码后端契约） -->
        <ChangePasswordCard :email="profile.detail?.email ?? ''" />

        <!-- 自助注销（issue #159）：创始管理员整个隐藏（Q1，
             后端 DELETE /me 同样拒绝，这里只是体验层）。
             卡片与同页其他卡同款液态玻璃，流光与文字换成警示红调
             （.liquid-glass--danger，全局设计系统变体） -->
        <section
          v-if="!isFounding"
          class="liquid-glass liquid-glass--danger danger-zone"
          aria-label="注销账号"
        >
          <div class="danger-zone__text">
            <h3>注销账号</h3>
            <p>
              注销后无法再登录；原报名与申请记录会保留为历史存档。
              如需恢复请联系工作室管理员。
            </p>
          </div>
          <el-button
            type="danger"
            plain
            :disabled="deleting"
            :loading="deleting"
            @click="handleDeleteAccount"
          >
            {{ deleting ? "注销中…" : "注销账号" }}
          </el-button>
        </section>
      </template>

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
  position: relative;
  z-index: 1;
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

/* 自助注销危险区（issue #159）：玻璃本体与流光来自全局
   .liquid-glass--danger（同款液态玻璃 + 红流光），这里只管布局与文字——
   标题用陶土红（--red），正文用降饱和红调，危险但不刺眼 */
.danger-zone {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 24px 26px;
}

.danger-zone__text h3 {
  margin: 0 0 4px;
  font-size: 15px;
  color: var(--red, #b84235);
}

.danger-zone__text p {
  margin: 0;
  color: #8a5a52;
  font-size: 13px;
  line-height: 1.6;
}

@media (max-width: 640px) {
  .danger-zone {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
