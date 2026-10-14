<script setup lang="ts">
/**
 * 修改密码卡片（issue #85 / FE-01）。
 *
 * 登录态下换密码：旧密码验证 + 新密码 6–20 位 + 确认一致性，全部在
 * auth/form.ts 的纯函数里校验（单独有测试），本组件只接线。
 *
 * 成功后端会撤销该用户**全部**会话（含当前请求的令牌），所以前端必须
 * 清掉本地会话并跳回登录页，带 ?reason=password-changed 让登录页出提示条
 * ——此时再留在设置页只会在下一个请求 401。
 *
 * 失败（旧密码错误 / 新密码与旧相同 / 网络异常）时保留输入可直接重试，
 * 服务端 message 原样展示（后端已给出面向用户的文案）。
 */
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";

import { AuthRequestError, changePassword } from "@/api/auth";
import {
  toChangePasswordPayload,
  validateChangePasswordForm,
  type ChangePasswordForm,
} from "@/auth/form";
import { clearAuthSession, readAuthSession } from "@/auth/session";

const router = useRouter();

const form = reactive<ChangePasswordForm>({
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});
const errors = ref<Record<string, string>>({});
const serverError = ref("");
const saving = ref(false);
const passwordVisible = reactive({
  old: false,
  next: false,
  confirm: false,
});

async function handleSubmit(): Promise<void> {
  if (saving.value) return;
  serverError.value = "";

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
    // 后端已撤销全部会话；本地也清掉，去登录页并带上提示原因
    clearAuthSession();
    await router.replace("/?reason=password-changed");
  } catch (error: unknown) {
    serverError.value =
      error instanceof AuthRequestError
        ? error.message
        : "服务暂时不可用，请稍后重试";
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
          :aria-invalid="Boolean(errors.oldPassword)"
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
      <span v-if="errors.oldPassword" class="pwd__error" role="alert">
        {{ errors.oldPassword }}
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
          :aria-invalid="Boolean(errors.newPassword)"
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
      <span v-if="errors.newPassword" class="pwd__error" role="alert">
        {{ errors.newPassword }}
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
    </label>

    <p v-if="serverError" class="pwd__server-error" role="alert">
      {{ serverError }}
    </p>

    <footer class="pwd__actions">
      <button
        type="submit"
        class="pwd__btn"
        :disabled="saving"
        :aria-busy="saving"
      >
        {{ saving ? "修改中…" : "修改密码" }}
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
  justify-content: flex-end;
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
}

.pwd__btn:hover:not(:disabled) {
  filter: brightness(1.08);
}

.pwd__btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
