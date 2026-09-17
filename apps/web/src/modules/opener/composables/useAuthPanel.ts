/**
 * 登录 / 注册面板的全部状态与行为。
 *
 * 刻意**不重写**请求与校验逻辑，而是复用 M2 已经落地的那套：
 *   - @/auth/form.ts      表单字段类型 + 校验规则
 *   - @/auth/submit.ts    校验 → 提交 → 存会话 → 跳转的编排
 *   - @/auth/session.ts   会话持久化（iot-pillot.auth）
 *   - @/api/auth.ts       HTTP 封装与错误映射
 * 这样 3D 开屏与 /login 后备页共用同一份契约，不会出现两套校验漂移。
 */
import { computed, onUnmounted, ref, type ComputedRef, type Ref } from "vue";
import { useRouter } from "vue-router";
import { AuthRequestError, login, register, sendVerifyCode } from "@/api/auth";
import {
  type LoginForm,
  type RegisterForm,
  validateRegisterForm,
} from "@/auth/form";
import { saveAuthSession } from "@/auth/session";
import { submitAuth, type AuthMode } from "@/auth/submit";

import { CONFIG } from "../config";

export interface UseAuthPanelResult {
  mode: Ref<AuthMode>;
  loginForm: Ref<LoginForm>;
  registerForm: Ref<RegisterForm>;
  errors: Ref<Record<string, string>>;
  notice: Ref<string>;
  submitError: Ref<string>;
  submitting: Ref<boolean>;
  sendingCode: Ref<boolean>;
  codeCooldown: Ref<number>;
  submitLabel: ComputedRef<string>;
  codeButtonLabel: ComputedRef<string>;
  setMode: (mode: AuthMode) => void;
  clearFieldError: (field: string) => void;
  submit: () => Promise<void>;
  sendCode: () => Promise<void>;
}

const UNAVAILABLE_MESSAGE = "服务暂时不可用，请稍后重试";

export function useAuthPanel(
  /** 守卫挡下访客时带在 URL 上的原目标（?redirect=），登录成功后跳回去 */
  redirect?: Ref<string | undefined>,
): UseAuthPanelResult {
  const router = useRouter();

  const mode = ref<AuthMode>("login");
  const submitting = ref(false);
  const sendingCode = ref(false);
  const errors = ref<Record<string, string>>({});
  const notice = ref("");
  const submitError = ref("");
  const codeCooldown = ref(0);

  const loginForm = ref<LoginForm>({ username: "", password: "" });
  const registerForm = ref<RegisterForm>({
    name: "",
    email: "",
    password: "",
    confirmPassword: "",
    code: "",
  });

  let cooldownTimer: ReturnType<typeof setInterval> | undefined;

  const submitLabel = computed(() => {
    if (submitting.value) {
      return "正在校验…";
    }
    return mode.value === "login" ? "进入工作台" : "创建账户";
  });

  const codeButtonLabel = computed(() => {
    if (codeCooldown.value > 0) {
      return `${codeCooldown.value}s 后重发`;
    }
    return sendingCode.value ? "发送中…" : "获取验证码";
  });

  function setMode(next: AuthMode): void {
    if (submitting.value || mode.value === next) {
      return;
    }
    mode.value = next;
    errors.value = {};
    submitError.value = "";
    notice.value = "";
  }

  function clearFieldError(field: string): void {
    const next = { ...errors.value };
    delete next[field];
    errors.value = next;
  }

  function toMessage(error: unknown): string {
    return error instanceof AuthRequestError ? error.message : UNAVAILABLE_MESSAGE;
  }

  async function submit(): Promise<void> {
    errors.value = {};
    submitError.value = "";
    notice.value = "";
    submitting.value = true;

    try {
      const form = mode.value === "login" ? loginForm.value : registerForm.value;
      const result = await submitAuth(
        mode.value,
        form,
        {
          login,
          register,
          saveSession: saveAuthSession,
          navigate: (path) => router.push(path),
        },
        redirect?.value,
      );

      if (!result.ok) {
        errors.value = result.errors;
      }
    } catch (error: unknown) {
      submitError.value = toMessage(error);
    } finally {
      submitting.value = false;
    }
  }

  /** 发送邮箱验证码（注册必须先拿到它才能提交）*/
  async function sendCode(): Promise<void> {
    if (sendingCode.value || codeCooldown.value > 0) {
      return;
    }

    submitError.value = "";
    notice.value = "";

    // 复用注册表单的邮箱规则，避免在这里再写一份正则造成两处漂移
    const emailError = validateRegisterForm(registerForm.value).email;
    if (emailError) {
      errors.value = { ...errors.value, email: emailError };
      return;
    }
    clearFieldError("email");

    sendingCode.value = true;
    try {
      await sendVerifyCode(registerForm.value.email.trim());
      notice.value = "验证码已发送，请到邮箱查收";

      codeCooldown.value = CONFIG.verifyCodeCooldownSeconds;
      if (cooldownTimer !== undefined) {
        clearInterval(cooldownTimer);
      }
      cooldownTimer = setInterval(() => {
        codeCooldown.value -= 1;
        if (codeCooldown.value <= 0 && cooldownTimer !== undefined) {
          clearInterval(cooldownTimer);
          cooldownTimer = undefined;
        }
      }, 1000);
    } catch (error: unknown) {
      submitError.value = toMessage(error);
    } finally {
      sendingCode.value = false;
    }
  }

  onUnmounted(() => {
    if (cooldownTimer !== undefined) {
      clearInterval(cooldownTimer);
      cooldownTimer = undefined;
    }
  });

  return {
    mode,
    loginForm,
    registerForm,
    errors,
    notice,
    submitError,
    submitting,
    sendingCode,
    codeCooldown,
    submitLabel,
    codeButtonLabel,
    setMode,
    clearFieldError,
    submit,
    sendCode,
  };
}
