<script setup lang="ts">
/**
 * 修改密码弹窗（issue #85 重构：液态玻璃 + 邮箱验证）。
 *
 * 提交流程复用「忘记密码」的后端契约：
 *   POST /api/v1/send-reset-code  向绑定邮箱发验证码（60s 冷却）
 *   POST /api/v1/reset-password   后端真实校验验证码（一次性、错 5 次作废），
 *                                 成功后撤销该用户全部会话
 * 因此弹窗不收旧密码——验证码顶替了它的位置；邮箱只读回显 /me 的绑定邮箱。
 *
 * 形态：居中 modal。卡外区域整层模糊（overlay backdrop-filter），
 * 卡片本体液态玻璃（半透明渐变 + 饱和放大 + 内高光边）。
 * 成功后清会话跳 /?reason=password-changed（登录页出黄条提示）。
 */
import {
  computed,
  nextTick,
  onBeforeUnmount,
  reactive,
  ref,
  watch,
} from "vue";
import { useRouter } from "vue-router";

import { AuthRequestError, resetPassword, sendResetCode } from "@/api/auth";
import {
  toResetPayload,
  validateResetForm,
  type ResetForm,
} from "@/auth/form";
import { clearAuthSession } from "@/auth/session";
import { CONFIG } from "@/modules/opener/config";
import { mapResetError, passwordStrength } from "../passwordPolicy";

const props = defineProps<{
  /** 打开状态（由入口组件控制） */
  open: boolean;
  /** 账号绑定邮箱（/me detail.email）；空串视为未绑定 */
  email: string;
}>();

const emit = defineEmits<{
  (event: "close"): void;
}>();

const router = useRouter();

const form = reactive<ResetForm>({
  email: "",
  code: "",
  password: "",
  confirmPassword: "",
});
const errors = ref<Record<string, string>>({});
const codeFieldError = ref("");
const serverError = ref("");
const sendingCode = ref(false);
const codeCooldown = ref(0);
const verifyNotice = ref("");
const submitting = ref(false);
const succeeded = ref(false);
const passwordVisible = reactive({ next: false, confirm: false });

const card = ref<HTMLElement | null>(null);
const codeInput = ref<HTMLInputElement | null>(null);

let cooldownTimer: ReturnType<typeof setInterval> | undefined;

const hasEmail = computed(() => props.email.trim() !== "");
const strength = computed(() => passwordStrength(form.password));

const codeButtonLabel = computed(() => {
  if (codeCooldown.value > 0) return `${codeCooldown.value}s 后重发`;
  return sendingCode.value ? "发送中…" : "获取验证码";
});

/** 打开时重置为干净表单；聚焦验证码框省一次点击 */
watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    form.email = props.email.trim();
    form.code = "";
    form.password = "";
    form.confirmPassword = "";
    errors.value = {};
    codeFieldError.value = "";
    serverError.value = "";
    verifyNotice.value = "";
    succeeded.value = false;
    await nextTick();
    if (hasEmail.value) codeInput.value?.focus();
  },
);

/** 打开期间锁 body 滚动；Esc 关闭（提交中不给关，避免状态撕裂） */
watch(
  () => props.open,
  (open) => {
    document.body.style.overflow = open ? "hidden" : "";
    if (open) {
      document.addEventListener("keydown", onKeydown);
    } else {
      document.removeEventListener("keydown", onKeydown);
    }
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKeydown);
  document.body.style.overflow = "";
  if (cooldownTimer !== undefined) {
    clearInterval(cooldownTimer);
    cooldownTimer = undefined;
  }
});

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape" && !submitting.value) {
    requestClose();
  }
}

function requestClose(): void {
  if (submitting.value || succeeded.value) return;
  emit("close");
}

async function sendCode(): Promise<void> {
  if (sendingCode.value || codeCooldown.value > 0 || submitting.value) return;
  serverError.value = "";
  verifyNotice.value = "";
  codeFieldError.value = "";

  sendingCode.value = true;
  try {
    await sendResetCode(form.email.trim());
    verifyNotice.value = "验证码已发送，请到邮箱查收";
    codeCooldown.value = CONFIG.verifyCodeCooldownSeconds;
    cooldownTimer = setInterval(() => {
      codeCooldown.value -= 1;
      if (codeCooldown.value <= 0 && cooldownTimer !== undefined) {
        clearInterval(cooldownTimer);
        cooldownTimer = undefined;
      }
    }, 1000);
  } catch (error: unknown) {
    serverError.value =
      error instanceof AuthRequestError
        ? error.message
        : "服务暂时不可用，请稍后重试";
  } finally {
    sendingCode.value = false;
  }
}

async function handleSubmit(): Promise<void> {
  if (submitting.value || succeeded.value) return;
  errors.value = {};
  codeFieldError.value = "";
  serverError.value = "";

  const validation = validateResetForm(form);
  if (Object.keys(validation).length > 0) {
    errors.value = validation;
    return;
  }

  submitting.value = true;
  try {
    await resetPassword(toResetPayload(form));
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
    const mapped = mapResetError(message);
    if (mapped?.field === "code") {
      codeFieldError.value = mapped.message;
    } else if (mapped?.field === "email") {
      errors.value = { email: mapped.message };
    } else {
      serverError.value = message;
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="pwd-modal">
      <div v-if="open" class="pwd-modal">
        <!-- 卡外区域：整层模糊 + 压暗。点击空白处关闭 -->
        <div class="pwd-modal__overlay" @click="requestClose"></div>

        <div
          ref="card"
          class="pwd-modal__card liquid-glass"
          role="dialog"
          aria-modal="true"
          aria-labelledby="pwd-modal-title"
        >
          <header class="pwd-modal__head">
            <h2 id="pwd-modal-title">修改密码</h2>
            <button
              type="button"
              class="pwd-modal__close"
              aria-label="关闭"
              :disabled="submitting"
              @click="requestClose"
            >
              ×
            </button>
          </header>

          <!-- 未绑定邮箱：无法走验证码流程，指路登录页忘记密码 -->
          <div v-if="!hasEmail" class="pwd-modal__notice" role="alert">
            账号尚未绑定邮箱，无法接收验证码。请在登录页使用「忘记密码」。
          </div>

          <template v-else>
            <p class="pwd-modal__intro">
              验证码将发送到你的绑定邮箱；修改成功后所有设备需重新登录。
            </p>

            <label class="pwd-modal__field">
              <span class="pwd-modal__label">绑定邮箱</span>
              <span class="pwd-modal__input-wrap pwd-modal__input-wrap--locked">
                <input
                  class="pwd-modal__input"
                  type="email"
                  :value="form.email"
                  readonly
                  tabindex="-1"
                  aria-readonly="true"
                />
              </span>
            </label>

            <label class="pwd-modal__field">
              <span class="pwd-modal__label">邮箱验证码</span>
              <span class="pwd-modal__code-row">
                <span class="pwd-modal__input-wrap">
                  <input
                    ref="codeInput"
                    v-model="form.code"
                    class="pwd-modal__input"
                    type="text"
                    autocomplete="one-time-code"
                    inputmode="numeric"
                    maxlength="6"
                    placeholder="6 位数字"
                    :aria-invalid="Boolean(errors.code || codeFieldError)"
                  />
                </span>
                <button
                  type="button"
                  class="pwd-modal__send"
                  :disabled="sendingCode || codeCooldown > 0 || submitting"
                  @click="sendCode"
                >
                  {{ codeButtonLabel }}
                </button>
              </span>
              <span v-if="errors.code || codeFieldError" class="pwd-modal__error" role="alert">
                {{ errors.code || codeFieldError }}
              </span>
              <span v-else-if="verifyNotice" class="pwd-modal__ok" role="status">
                {{ verifyNotice }}
              </span>
            </label>

            <label class="pwd-modal__field">
              <span class="pwd-modal__label">新密码</span>
              <span class="pwd-modal__input-wrap">
                <input
                  v-model="form.password"
                  class="pwd-modal__input"
                  :type="passwordVisible.next ? 'text' : 'password'"
                  autocomplete="new-password"
                  placeholder="6–20 位新密码"
                  :aria-invalid="Boolean(errors.password)"
                />
                <button
                  type="button"
                  class="pwd-modal__eye"
                  :aria-label="passwordVisible.next ? '隐藏密码' : '显示密码'"
                  :aria-pressed="passwordVisible.next"
                  @click="passwordVisible.next = !passwordVisible.next"
                >
                  <svg
                    v-if="!passwordVisible.next"
                    viewBox="0 0 24 24"
                    width="16"
                    height="16"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    aria-hidden="true"
                  >
                    <path d="M2 12s3.6-6.2 10-6.2S22 12 22 12s-3.6 6.2-10 6.2S2 12 2 12z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                  <svg
                    v-else
                    viewBox="0 0 24 24"
                    width="16"
                    height="16"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    aria-hidden="true"
                  >
                    <path d="M2 12s3.6-6.2 10-6.2S22 12 22 12s-3.6 6.2-10 6.2S2 12 2 12z" />
                    <circle cx="12" cy="12" r="3" />
                    <line x1="4" y1="20" x2="20" y2="4" />
                  </svg>
                </button>
              </span>
              <span v-if="strength" class="pwd-modal__strength" role="status">
                <span class="pwd-modal__strength-bars" :data-level="strength.level">
                  <i></i><i></i><i></i>
                </span>
                <span>强度{{ strength.label }}</span>
              </span>
              <span v-if="errors.password" class="pwd-modal__error" role="alert">
                {{ errors.password }}
              </span>
            </label>

            <label class="pwd-modal__field">
              <span class="pwd-modal__label">确认新密码</span>
              <span class="pwd-modal__input-wrap">
                <input
                  v-model="form.confirmPassword"
                  class="pwd-modal__input"
                  :type="passwordVisible.confirm ? 'text' : 'password'"
                  autocomplete="new-password"
                  placeholder="再次输入新密码"
                  :aria-invalid="Boolean(errors.confirmPassword)"
                />
                <button
                  type="button"
                  class="pwd-modal__eye"
                  :aria-label="passwordVisible.confirm ? '隐藏密码' : '显示密码'"
                  :aria-pressed="passwordVisible.confirm"
                  @click="passwordVisible.confirm = !passwordVisible.confirm"
                >
                  <svg
                    v-if="!passwordVisible.confirm"
                    viewBox="0 0 24 24"
                    width="16"
                    height="16"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    aria-hidden="true"
                  >
                    <path d="M2 12s3.6-6.2 10-6.2S22 12 22 12s-3.6 6.2-10 6.2S2 12 2 12z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                  <svg
                    v-else
                    viewBox="0 0 24 24"
                    width="16"
                    height="16"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    aria-hidden="true"
                  >
                    <path d="M2 12s3.6-6.2 10-6.2S22 12 22 12s-3.6 6.2-10 6.2S2 12 2 12z" />
                    <circle cx="12" cy="12" r="3" />
                    <line x1="4" y1="20" x2="20" y2="4" />
                  </svg>
                </button>
              </span>
              <span v-if="errors.confirmPassword" class="pwd-modal__error" role="alert">
                {{ errors.confirmPassword }}
              </span>
            </label>

            <p v-if="serverError" class="pwd-modal__server-error" role="alert">
              {{ serverError }}
            </p>

            <footer class="pwd-modal__actions">
              <button
                type="button"
                class="pwd-modal__ghost"
                :disabled="submitting"
                @click="requestClose"
              >
                取消
              </button>
              <button
                type="button"
                class="pwd-modal__submit"
                :class="{ 'pwd-modal__submit--done': succeeded }"
                :disabled="submitting || succeeded"
                :aria-busy="submitting"
                @click="handleSubmit"
              >
                {{
                  succeeded
                    ? "✓ 密码已修改"
                    : submitting
                      ? "提交中…"
                      : "确认修改"
                }}
              </button>
            </footer>
            <p v-if="succeeded" class="pwd-modal__success" role="status">
              所有设备已退出，正在前往登录页…
            </p>
          </template>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* —— 遮罩：卡外整层模糊 + 压暗（用户要求的「卡片之外区域模糊」） —— */
.pwd-modal {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: grid;
  place-items: center;
  padding: 20px;
}

.pwd-modal__overlay {
  position: absolute;
  inset: 0;
  background: rgba(16, 22, 44, 0.38);
  backdrop-filter: blur(16px) saturate(1.1);
  -webkit-backdrop-filter: blur(16px) saturate(1.1);
}

/* 卡片玻璃质感来自全局 .liquid-glass（通透底 + 流光边缘）；
   这里只保留弹窗专属的尺寸、内边距与更圆的角 */
.pwd-modal__card {
  width: min(420px, 100%);
  max-height: min(86vh, 720px);
  overflow-y: auto;
  padding: 24px 26px 22px;
  border-radius: 24px;
}

.pwd-modal__head {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.pwd-modal__head h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.pwd-modal__close {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  padding: 0;
  border: 1px solid rgba(20, 26, 50, 0.14);
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.4);
  color: #141a32;
  font-size: 17px;
  line-height: 1;
  cursor: pointer;
  transition: background 140ms ease;
}

.pwd-modal__close:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.8);
}

.pwd-modal__close:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pwd-modal__intro {
  position: relative;
  margin: 0 0 18px;
  color: #4b5670;
  font-size: 12px;
  line-height: 1.7;
}

.pwd-modal__notice {
  position: relative;
  margin-top: 10px;
  padding: 14px 16px;
  border: 1px solid rgba(20, 26, 50, 0.14);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.45);
  color: #4b5670;
  font-size: 13px;
  line-height: 1.7;
}

.pwd-modal__field {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.pwd-modal__label {
  font-size: 12px;
  font-weight: 600;
  color: #2a3350;
}

.pwd-modal__input-wrap {
  display: flex;
  align-items: center;
  border: 1px solid rgba(20, 26, 50, 0.16);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.5);
  transition: border-color 140ms ease, background 140ms ease;
}

.pwd-modal__input-wrap:focus-within {
  border-color: #1739f5;
  background: rgba(255, 255, 255, 0.72);
}

.pwd-modal__input-wrap--locked {
  background: rgba(20, 26, 50, 0.05);
}

.pwd-modal__input {
  flex: 1;
  min-width: 0;
  padding: 10px 12px;
  border: 0;
  border-radius: 10px 0 0 10px;
  background: transparent;
  font: inherit;
  font-size: 13px;
  color: #141a32;
  outline: none;
}

.pwd-modal__input[readonly] {
  color: #4b5670;
  cursor: default;
}

/* 小眼睛：输入框右侧图标按钮 */
.pwd-modal__eye {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 38px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #4b5670;
  cursor: pointer;
}

.pwd-modal__eye:hover {
  color: #1739f5;
}

.pwd-modal__code-row {
  display: flex;
  gap: 8px;
  align-items: stretch;
}

.pwd-modal__code-row .pwd-modal__input-wrap {
  flex: 1;
  min-width: 0;
}

.pwd-modal__send {
  flex-shrink: 0;
  padding: 0 14px;
  border: 1px solid rgba(20, 26, 50, 0.2);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.55);
  color: #1739f5;
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
}

.pwd-modal__send:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.85);
}

.pwd-modal__send:disabled {
  color: #4b5670;
  opacity: 0.6;
  cursor: not-allowed;
}

.pwd-modal__error {
  color: #a33d32;
  font-size: 12px;
}

.pwd-modal__ok {
  color: #2f7d43;
  font-size: 12px;
}

/* 强度条：三段直角计量块 */
.pwd-modal__strength {
  display: flex;
  gap: 10px;
  align-items: center;
  color: #4b5670;
  font-size: 11px;
}

.pwd-modal__strength-bars {
  display: flex;
  gap: 4px;
}

.pwd-modal__strength-bars i {
  width: 30px;
  height: 4px;
  background: rgba(20, 26, 50, 0.14);
}

.pwd-modal__strength-bars[data-level="1"] i:nth-child(-n + 1) {
  background: #ba291b;
}

.pwd-modal__strength-bars[data-level="2"] i:nth-child(-n + 2) {
  background: #b98a1d;
}

.pwd-modal__strength-bars[data-level="3"] i {
  background: #2f7d43;
}

.pwd-modal__server-error {
  position: relative;
  margin: 0 0 14px;
  padding: 11px 14px;
  border: 1px solid rgba(163, 61, 50, 0.3);
  border-radius: 10px;
  background: rgba(253, 241, 238, 0.75);
  color: #a33d32;
  font-size: 12px;
  line-height: 1.6;
}

.pwd-modal__actions {
  position: relative;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 4px;
}

.pwd-modal__ghost {
  padding: 9px 18px;
  border: 1px solid rgba(20, 26, 50, 0.2);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.4);
  color: #2a3350;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.pwd-modal__ghost:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.75);
}

.pwd-modal__ghost:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pwd-modal__submit {
  padding: 9px 22px;
  border: 0;
  border-radius: 10px;
  background: #1739f5;
  color: #fff;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 160ms ease;
}

.pwd-modal__submit:hover:not(:disabled) {
  background: #0f2ed6;
}

.pwd-modal__submit:disabled {
  cursor: not-allowed;
  opacity: 0.65;
}

.pwd-modal__submit--done {
  background: #2f7d43;
  opacity: 1;
}

.pwd-modal__success {
  position: relative;
  margin: 12px 0 0;
  color: #2f7d43;
  font-size: 12px;
  text-align: right;
}

/* —— 进出场：遮罩淡入、卡片缩放浮出（ease-out-quint） —— */
.pwd-modal-enter-active,
.pwd-modal-leave-active {
  transition: opacity 0.24s ease;
}

.pwd-modal-enter-active .pwd-modal__card,
.pwd-modal-leave-active .pwd-modal__card {
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.24s ease;
}

.pwd-modal-enter-from,
.pwd-modal-leave-to {
  opacity: 0;
}

.pwd-modal-enter-from .pwd-modal__card,
.pwd-modal-leave-to .pwd-modal__card {
  opacity: 0;
  transform: translateY(14px) scale(0.96);
}

@media (prefers-reduced-motion: reduce) {
  .pwd-modal-enter-active,
  .pwd-modal-leave-active,
  .pwd-modal-enter-active .pwd-modal__card,
  .pwd-modal-leave-active .pwd-modal__card {
    transition: none;
  }

  /* 流光停止流动，留静态高光边 */
  .pwd-modal__card::after {
    animation: none;
  }
}
</style>
