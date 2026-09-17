<script setup lang="ts">
/**
 * 登录 / 注册面板（纯展示组件）。
 *
 * 只负责「长什么样」和「点了以后通知父级」，状态与请求全在
 * composables/useAuthPanel.ts 里。放在 3D canvas 上方绝对定位显示。
 *
 * 注意 loginForm / registerForm 是父级创建的同一个响应式对象，子组件通过
 * v-model 写它的字段（不重新赋值 prop 本身）——这是 Vue 里共享表单模型的
 * 常规做法，父级始终是这两个对象的唯一所有者。
 */
import type { LoginForm, RegisterForm } from "@/auth/form";
import type { AuthMode } from "@/auth/submit";

import AuthField from "./AuthField.vue";

defineProps<{
  mode: AuthMode;
  loginForm: LoginForm;
  registerForm: RegisterForm;
  /** 字段级错误，键为字段名 */
  errors: Record<string, string>;
  /** 成功类提示（例如验证码已发送）*/
  notice: string;
  /** 整表级别的错误（网络/后端返回）*/
  submitError: string;
  /** 提交中：锁定输入与 tab */
  submitting: boolean;
  sendingCode: boolean;
  /** 验证码重发冷却剩余秒数 */
  codeCooldown: number;
  /** 提交按钮文案（由 useAuthPanel 计算，避免样式组件里塞业务分支）*/
  submitLabel: string;
  /** 验证码按钮文案 */
  codeButtonLabel: string;
}>();

defineEmits<{
  (event: "update:mode", mode: AuthMode): void;
  (event: "submit"): void;
  (event: "send-code"): void;
}>();
</script>

<template>
  <section class="auth-panel" aria-labelledby="studio-panel-title">
    <header class="auth-panel__head">
      <p class="auth-panel__eyebrow">IoT 全栈工作室 · 招新</p>
      <h1 id="studio-panel-title" class="auth-panel__title">
        {{ mode === "login" ? "回到工作台" : "加入工作室" }}
      </h1>
      <p class="auth-panel__subtitle">
        {{
          mode === "login"
            ? "使用工作室账号继续协作。"
            : "填写资料，加入一群认真做作品的人。"
        }}
      </p>
    </header>

    <div class="auth-panel__tabs" role="tablist" aria-label="认证方式">
      <button
        id="studio-tab-login"
        type="button"
        role="tab"
        :aria-selected="mode === 'login'"
        :disabled="submitting"
        @click="$emit('update:mode', 'login')"
      >
        登录
      </button>
      <button
        id="studio-tab-register"
        type="button"
        role="tab"
        :aria-selected="mode === 'register'"
        :disabled="submitting"
        @click="$emit('update:mode', 'register')"
      >
        注册
      </button>
    </div>

    <form
      class="auth-panel__form"
      novalidate
      :aria-labelledby="
        mode === 'login' ? 'studio-tab-login' : 'studio-tab-register'
      "
      @submit.prevent="$emit('submit')"
    >
      <template v-if="mode === 'login'">
        <AuthField
          id="studio-username"
          v-model="loginForm.username"
          label="用户名"
          autocomplete="username"
          placeholder="输入用户名"
          :error="errors.username"
        />
        <AuthField
          id="studio-password"
          v-model="loginForm.password"
          label="密码"
          type="password"
          name="password"
          autocomplete="current-password"
          placeholder="输入密码"
          :error="errors.password"
        />
      </template>

      <template v-else>
        <AuthField
          id="studio-name"
          v-model="registerForm.name"
          label="姓名"
          autocomplete="name"
          placeholder="3–20 个字符"
          :error="errors.name"
        />
        <AuthField
          id="studio-email"
          v-model="registerForm.email"
          label="邮箱"
          type="email"
          autocomplete="email"
          inputmode="email"
          placeholder="name@example.com"
          :error="errors.email"
        />
        <AuthField
          id="studio-reg-password"
          v-model="registerForm.password"
          label="密码"
          type="password"
          name="password"
          autocomplete="new-password"
          placeholder="6–20 位"
          :error="errors.password"
        />
        <AuthField
          id="studio-confirm"
          v-model="registerForm.confirmPassword"
          label="确认密码"
          type="password"
          name="confirmPassword"
          autocomplete="new-password"
          placeholder="再次输入密码"
          :error="errors.confirmPassword"
        />
        <AuthField
          id="studio-code"
          v-model="registerForm.code"
          label="邮箱验证码"
          inputmode="numeric"
          autocomplete="one-time-code"
          :maxlength="6"
          placeholder="6 位数字"
          :error="errors.code"
        >
          <template #action>
            <button
              type="button"
              class="auth-panel__code-button"
              :disabled="sendingCode || codeCooldown > 0 || submitting"
              @click="$emit('send-code')"
            >
              {{ codeButtonLabel }}
            </button>
          </template>
        </AuthField>
      </template>

      <p v-if="notice" class="auth-panel__notice" role="status">{{ notice }}</p>
      <p v-if="submitError" class="auth-panel__alert" role="alert">
        {{ submitError }}
      </p>

      <button
        class="auth-panel__submit"
        type="submit"
        :disabled="submitting"
        :aria-busy="submitting"
      >
        {{ submitLabel }}
      </button>
    </form>

    <p class="auth-panel__note">
      {{
        mode === "login"
          ? "登录即代表你同意遵守工作室协作规范。"
          : "注册需要邮箱验证码，请先点击「获取验证码」。"
      }}
    </p>
  </section>
</template>

<style scoped>
.auth-panel {
  padding: 26px 24px 22px;
  color: var(--opener-ink);
  border: 1px solid var(--opener-card-border);
  border-radius: 14px;
  background: var(--opener-card-bg);
  box-shadow: var(--opener-card-shadow);
  backdrop-filter: blur(18px) saturate(140%);
  font-family: var(--opener-font);
}

.auth-panel__head {
  margin-bottom: 20px;
}

.auth-panel__eyebrow {
  margin: 0 0 8px;
  color: var(--opener-blue-bright);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.auth-panel__title {
  margin: 0;
  font-family: var(--opener-serif);
  font-size: 26px;
  font-weight: 600;
  letter-spacing: -0.02em;
}

.auth-panel__subtitle {
  margin: 8px 0 0;
  color: var(--opener-soft);
  font-size: 13px;
  line-height: 1.6;
}

.auth-panel__tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--opener-divider);
}

.auth-panel__tabs button {
  position: relative;
  padding: 10px 8px;
  color: var(--opener-tab);
  border: 0;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.1em;
}

.auth-panel__tabs button::after {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: var(--opener-blue);
  content: "";
  opacity: 0;
  transform: scaleX(0.35);
  transition: opacity 180ms ease, transform 180ms ease;
}

.auth-panel__tabs button[aria-selected="true"] {
  color: var(--opener-blue);
}

.auth-panel__tabs button[aria-selected="true"]::after {
  opacity: 1;
  transform: scaleX(1);
}

.auth-panel__tabs button:disabled {
  cursor: wait;
}

.auth-panel__form {
  display: grid;
  gap: 14px;
}

.auth-panel__code-button {
  min-width: 104px;
  padding: 0 10px;
  color: var(--opener-blue);
  border: 1px dashed var(--opener-code-border);
  border-radius: 6px;
  background: var(--opener-code-bg);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
  white-space: nowrap;
}

.auth-panel__code-button:disabled {
  color: #8b96a3;
  border-color: var(--opener-code-border-disabled);
  background: var(--opener-code-bg-disabled);
  cursor: not-allowed;
}

.auth-panel__notice {
  margin: 0;
  padding: 9px 12px;
  color: var(--opener-success);
  border-left: 2px solid var(--opener-success);
  background: var(--opener-success-bg);
  font-size: 12px;
  line-height: 1.5;
}

.auth-panel__alert {
  margin: 0;
  padding: 9px 12px;
  color: #8e2f27;
  border-left: 2px solid var(--opener-danger);
  background: var(--opener-danger-bg);
  font-size: 12px;
  line-height: 1.5;
}

.auth-panel__submit {
  height: 46px;
  margin-top: 4px;
  color: #fffdf7;
  border: 1px solid var(--opener-blue);
  border-radius: 6px;
  background: var(--opener-blue);
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.08em;
  transition: background 180ms ease, transform 180ms ease;
}

.auth-panel__submit:hover:not(:disabled) {
  background: var(--opener-blue-deep);
}

.auth-panel__submit:active:not(:disabled) {
  transform: translateY(1px);
}

.auth-panel__submit:disabled {
  cursor: wait;
  opacity: 0.68;
}

.auth-panel__note {
  margin: 16px 0 0;
  color: #74808d;
  font-size: 11px;
  line-height: 1.6;
}

@media (prefers-reduced-motion: reduce) {
  .auth-panel__tabs button::after,
  .auth-panel__submit {
    transition: none;
  }
}
</style>
