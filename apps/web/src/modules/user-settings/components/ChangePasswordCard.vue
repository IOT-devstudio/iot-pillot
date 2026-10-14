<script setup lang="ts">
/**
 * 修改密码卡片（issue #85；A 档打磨对照大厂改密交互）。
 *
 * 大厂惯例对应的五处改进：
 *   1. 实时规则清单——边输边打勾，不是提交后一次性报错
 *   2. 强度条——长度 + 字符种类分弱/中/强，纯前端计算
 *   3. 错误字段级映射——后端「用户名或密码错误」落到当前密码输入框，
 *      而不是整块网络错误样式
 *   4. 成功态过渡——按钮先变「✓ 密码已修改」，停一秒再跳登录页
 *   5. Caps Lock 提示——密码框聚焦且大写锁定开启时提示
 *
 * 成功后端会撤销该用户**全部**会话（含当前请求的令牌），所以前端必须
 * 清掉本地会话并跳回登录页，带 ?reason=password-changed 让登录页出提示条。
 * 失败保留输入可直接重试；硬校验（6-20 位、必填）仍在 auth/form.ts。
 */
import { computed, reactive, ref } from "vue";
import { useRouter } from "vue-router";

import { AuthRequestError, changePassword } from "@/api/auth";
import {
  toChangePasswordPayload,
  validateChangePasswordForm,
  type ChangePasswordForm,
} from "@/auth/form";
import { clearAuthSession, readAuthSession } from "@/auth/session";
import {
  mapChangePasswordError,
  passwordRules,
  passwordStrength,
} from "../passwordPolicy";

const router = useRouter();

const form = reactive<ChangePasswordForm>({
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});
const errors = ref<Record<string, string>>({});
/** 后端分流回字段的错误（当前密码错 / 新旧相同），随输入清除 */
const fieldErrors = reactive<{
  oldPassword: string;
  newPassword: string;
}>({ oldPassword: "", newPassword: "" });
const serverError = ref("");
const saving = ref(false);
const succeeded = ref(false);
const passwordVisible = reactive({
  old: false,
  next: false,
  confirm: false,
});

/** Caps Lock：大写锁定开启且该字段正被编辑时给出提示 */
const capsLock = ref(false);
const focusedField = ref<"oldPassword" | "newPassword" | "confirmPassword" | null>(
  null,
);

const rules = computed(() =>
  passwordRules(form.oldPassword, form.newPassword, form.confirmPassword),
);
const strength = computed(() => passwordStrength(form.newPassword));

/** keyup 时读取大写锁定状态（事件对象自带，无需额外轮询） */
function trackKey(event: KeyboardEvent): void {
  capsLock.value = event.getModifierState?.("CapsLock") ?? false;
}

function focusField(
  field: "oldPassword" | "newPassword" | "confirmPassword",
): void {
  focusedField.value = field;
}

function blurField(
  field: "oldPassword" | "newPassword" | "confirmPassword",
): void {
  if (focusedField.value === field) {
    focusedField.value = null;
  }
}

/** 输入即清掉该字段的服务端错误与提交校验错误，减少视觉残留 */
function clearFieldError(field: "oldPassword" | "newPassword" | "confirmPassword"): void {
  if (field === "oldPassword" || field === "newPassword") {
    fieldErrors[field] = "";
  }
  if (errors.value[field]) {
    const next = { ...errors.value };
    delete next[field];
    errors.value = next;
  }
}

function capsHintFor(
  field: "oldPassword" | "newPassword" | "confirmPassword",
): string {
  return capsLock.value && focusedField.value === field
    ? "大写锁定已开启"
    : "";
}

async function handleSubmit(): Promise<void> {
  if (saving.value || succeeded.value) return;
  serverError.value = "";
  fieldErrors.oldPassword = "";
  fieldErrors.newPassword = "";

  const validation = validateChangePasswordForm(form);
  errors.value = validation;
  if (Object.keys(validation).length > 0) {
    return;
  }

  const session = readAuthSession();
  if (session === null) {
    serverError.value = "登录状态已失效，请重新登录";
    return;
  }

  saving.value = true;
  try {
    await changePassword(
      session.access_token,
      toChangePasswordPayload(form),
    );
    // 成功态先停一拍再跳：让用户看到「已修改」，而不是瞬间被弹走
    succeeded.value = true;
    clearAuthSession();
    await new Promise((resolve) => setTimeout(resolve, 1000));
    await router.replace("/?reason=password-changed");
  } catch (error: unknown) {
    const message =
      error instanceof AuthRequestError
        ? error.message
        : "服务暂时不可用，请稍后重试";
    const mapped = mapChangePasswordError(message);
    if (mapped !== null) {
      fieldErrors[mapped.field] = mapped.message;
    } else {
      serverError.value = message;
    }
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <form class="pwd" novalidate @submit.prevent="handleSubmit">
    <h2 class="pwd__section-title">修改密码</h2>
    <p class="pwd__hint">
      修改成功后所有登录端（包括本页）都会退出，需要用新密码重新登录。
    </p>

    <label class="pwd__field">
      <span class="pwd__label">当前密码</span>
      <span class="pwd__input-wrap">
        <input
          v-model="form.oldPassword"
          class="pwd__input"
          :type="passwordVisible.old ? 'text' : 'password'"
          autocomplete="current-password"
          placeholder="输入当前密码"
          :aria-invalid="Boolean(errors.oldPassword || fieldErrors.oldPassword)"
          @input="clearFieldError('oldPassword')"
          @keyup="trackKey"
          @focus="focusField('oldPassword')"
          @blur="blurField('oldPassword')"
        />
        <button
          type="button"
          class="pwd__toggle"
          :aria-label="passwordVisible.old ? '隐藏密码' : '显示密码'"
          :aria-pressed="passwordVisible.old"
          @click="passwordVisible.old = !passwordVisible.old"
        >
          {{ passwordVisible.old ? "隐藏" : "显示" }}
        </button>
      </span>
      <span
        v-if="errors.oldPassword || fieldErrors.oldPassword"
        class="pwd__error"
        role="alert"
      >
        {{ errors.oldPassword || fieldErrors.oldPassword }}
      </span>
      <span v-else-if="capsHintFor('oldPassword')" class="pwd__caps" role="status">
        ⌁ 大写锁定已开启
      </span>
    </label>

    <label class="pwd__field">
      <span class="pwd__label">新密码</span>
      <span class="pwd__input-wrap">
        <input
          v-model="form.newPassword"
          class="pwd__input"
          :type="passwordVisible.next ? 'text' : 'password'"
          autocomplete="new-password"
          placeholder="6–20 位新密码"
          :aria-invalid="Boolean(errors.newPassword || fieldErrors.newPassword)"
          @input="clearFieldError('newPassword')"
          @keyup="trackKey"
          @focus="focusField('newPassword')"
          @blur="blurField('newPassword')"
        />
        <button
          type="button"
          class="pwd__toggle"
          :aria-label="passwordVisible.next ? '隐藏密码' : '显示密码'"
          :aria-pressed="passwordVisible.next"
          @click="passwordVisible.next = !passwordVisible.next"
        >
          {{ passwordVisible.next ? "隐藏" : "显示" }}
        </button>
      </span>
      <!-- 强度条：三段计量 + 文案，空密码时不出现 -->
      <span v-if="strength" class="pwd__strength" role="status">
        <span class="pwd__strength-bars" :data-level="strength.level">
          <i></i><i></i><i></i>
        </span>
        <span class="pwd__strength-label">强度{{ strength.label }}</span>
      </span>
      <span
        v-if="errors.newPassword || fieldErrors.newPassword"
        class="pwd__error"
        role="alert"
      >
        {{ errors.newPassword || fieldErrors.newPassword }}
      </span>
      <span v-else-if="capsHintFor('newPassword')" class="pwd__caps" role="status">
        ⌁ 大写锁定已开启
      </span>
    </label>

    <label class="pwd__field">
      <span class="pwd__label">确认新密码</span>
      <span class="pwd__input-wrap">
        <input
          v-model="form.confirmPassword"
          class="pwd__input"
          :type="passwordVisible.confirm ? 'text' : 'password'"
          autocomplete="new-password"
          placeholder="再次输入新密码"
          :aria-invalid="Boolean(errors.confirmPassword)"
          @input="clearFieldError('confirmPassword')"
          @keyup="trackKey"
          @focus="focusField('confirmPassword')"
          @blur="blurField('confirmPassword')"
        />
        <button
          type="button"
          class="pwd__toggle"
          :aria-label="passwordVisible.confirm ? '隐藏密码' : '显示密码'"
          :aria-pressed="passwordVisible.confirm"
          @click="passwordVisible.confirm = !passwordVisible.confirm"
        >
          {{ passwordVisible.confirm ? "隐藏" : "显示" }}
        </button>
      </span>
      <span v-if="errors.confirmPassword" class="pwd__error" role="alert">
        {{ errors.confirmPassword }}
      </span>
      <span
        v-else-if="capsHintFor('confirmPassword')"
        class="pwd__caps"
        role="status"
      >
        ⌁ 大写锁定已开启
      </span>
    </label>

    <!-- 实时规则清单：打勾即满足，红叉为明确违反，空心为还没输入 -->
    <ul class="pwd__rules" aria-label="新密码要求">
      <li
        v-for="rule in rules"
        :key="rule.id"
        :data-state="rule.state"
      >
        <span class="pwd__rule-mark" aria-hidden="true">{{
          rule.state === "ok" ? "✓" : rule.state === "fail" ? "✕" : "○"
        }}</span>
        {{ rule.label }}
      </li>
    </ul>

    <p v-if="serverError" class="pwd__server-error" role="alert">
      {{ serverError }}
    </p>

    <footer class="pwd__actions">
      <p v-if="succeeded" class="pwd__success" role="status">
        密码已修改，所有设备已退出，正在前往登录页…
      </p>
      <button
        type="submit"
        class="pwd__btn"
        :class="{ 'pwd__btn--done': succeeded }"
        :disabled="saving || succeeded"
        :aria-busy="saving"
      >
        {{
          succeeded
            ? "✓ 密码已修改"
            : saving
              ? "修改中…"
              : "修改密码"
        }}
      </button>
    </footer>
  </form>
</template>

<style scoped>
/* 视觉与 UserSettingsForm 的 .settings__* 同一套值：同一张页面上的两张卡，
   间距、圆角、输入框高度必须一致 */
.pwd {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 24px 26px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: #fff;
}

.pwd__section-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--ink);
}

.pwd__hint {
  margin: 0;
  color: var(--ink-soft);
  font-size: 12px;
  line-height: 1.7;
}

.pwd__field {
  display: flex;
  flex-direction: column;
  /* 必须显式压掉 signal.css 的 `.swiss-signal form > label`
     （标签+输入同行的认证页布局：align-items:center + space-between），
     否则这张卡的标签会被居中、输入框收缩成一行内容宽 */
  align-items: stretch;
  justify-content: flex-start;
  gap: 6px;
}

.pwd__label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ink);
}

.pwd__input-wrap {
  display: flex;
  align-items: center;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: #fff;
}

.pwd__input-wrap:focus-within {
  border-color: var(--blue);
}

.pwd__input {
  flex: 1;
  min-width: 0;
  padding: 9px 12px;
  border: 0;
  border-radius: 6px 0 0 6px;
  background: transparent;
  font: inherit;
  font-size: 13px;
  color: var(--ink);
  outline: none;
}

.pwd__toggle {
  flex-shrink: 0;
  padding: 0 12px;
  border: 0;
  background: transparent;
  color: var(--ink-soft);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}

.pwd__toggle:hover {
  color: var(--blue);
}

.pwd__error {
  color: #a33d32;
  font-size: 12px;
}

.pwd__caps {
  color: #8a6512;
  font-size: 12px;
}

/* —— 强度条：三段直角计量块，跟瑞士信号的方块语言一致 —— */
.pwd__strength {
  display: flex;
  gap: 10px;
  align-items: center;
}

.pwd__strength-bars {
  display: flex;
  gap: 4px;
}

.pwd__strength-bars i {
  width: 34px;
  height: 4px;
  background: var(--line);
}

.pwd__strength-bars[data-level="1"] i:nth-child(-n + 1) {
  background: #ba291b;
}

.pwd__strength-bars[data-level="2"] i:nth-child(-n + 2) {
  background: #b98a1d;
}

.pwd__strength-bars[data-level="3"] i {
  background: #2f7d43;
}

.pwd__strength-label {
  color: var(--ink-soft);
  font-size: 11px;
}

/* —— 实时规则清单 —— */
.pwd__rules {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.pwd__rules li {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  color: var(--ink-soft);
}

.pwd__rules li[data-state="ok"] {
  color: #2f7d43;
}

.pwd__rules li[data-state="fail"] {
  color: #a33d32;
}

.pwd__rule-mark {
  width: 14px;
  text-align: center;
  font-weight: 700;
}

.pwd__server-error {
  margin: 0;
  padding: 12px 16px;
  color: #a33d32;
  border-left: 3px solid #a33d32;
  border-radius: 6px;
  background: #fdf3f1;
  font-size: 13px;
}

.pwd__actions {
  display: flex;
  gap: 14px;
  align-items: center;
  justify-content: flex-end;
}

.pwd__success {
  margin: 0;
  color: #2f7d43;
  font-size: 12px;
}

.pwd__btn {
  padding: 9px 22px;
  border: 0;
  border-radius: 6px;
  background: var(--blue);
  color: #fff;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 160ms ease;
}

.pwd__btn:hover:not(:disabled) {
  filter: brightness(1.08);
}

.pwd__btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.pwd__btn--done {
  background: #2f7d43;
  opacity: 1;
}

@media (prefers-reduced-motion: reduce) {
  .pwd__btn {
    transition: none;
  }
}
</style>
