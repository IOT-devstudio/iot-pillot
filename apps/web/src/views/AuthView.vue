<script setup lang="ts">
import { computed, inject, onUnmounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  AuthRequestError,
  login,
  register,
  resetPassword,
  sendResetCode,
  sendVerifyCode,
} from "@/api/auth";
import {
  type LoginForm,
  type RegisterForm,
  type ResetForm,
  toResetPayload,
  validateRegisterForm,
  validateResetForm,
} from "@/auth/form";
import { saveAuthSession } from "@/auth/session";
import { submitAuth, type AuthMode } from "@/auth/submit";
import { CONFIG } from "@/modules/opener/config";

import {
  SIGNAL_BUSY,
  SIGNAL_MODE,
  SIGNAL_TRANSITION,
} from "@/modules/swiss-signal/context";
const router = useRouter();
const route = useRoute();

/**
 * 保留旧开屏的 fallback 查询参数，历史兼容链接仍可使用真实认证。
 */
const showCompatNotice = computed(() => route.query.fallback === "webgl");

/**
 * 守卫挡下访客时会带上 ?redirect=<原路径>，登录成功后跳回去。
 * 取值交给 submitAuth 校验（只接受站内绝对路径），这里不做判断。
 */
const redirect = computed(() => {
  const raw = route.query.redirect;
  return typeof raw === "string" ? raw : undefined;
});

const mode = inject(SIGNAL_MODE, ref<AuthMode>("login"));
const transition = inject(SIGNAL_TRANSITION, async (change: () => unknown) => {
  await change();
});
const passwordVisible = reactive({
  login: false,
  register: false,
  confirm: false,
  resetNew: false,
  resetConfirm: false,
});
const loading = inject(SIGNAL_BUSY, ref(false));
const sendingCode = ref(false);
const codeCooldown = ref(0);
const verifyNotice = ref("");
const errors = ref<Record<string, string>>({});
const submitError = ref("");
/** 重置成功后显示在登录面板的确认语 */
const loginNotice = ref("");

let cooldownTimer: ReturnType<typeof setInterval> | undefined;

const loginForm = reactive<LoginForm>({
  username: "",
  password: "",
});

const registerForm = reactive<RegisterForm>({
  name: "",
  email: "",
  password: "",
  confirmPassword: "",
  code: "",
});

const resetForm = reactive<ResetForm>({
  email: "",
  password: "",
  confirmPassword: "",
  code: "",
});

/**
 * 会话失效提示（issue #84/#85）。
 * 守卫与会话请求器把失效原因放在 ?reason= 里带到登录页：
 *   invalidated      —— 4011：被顶替 / 改密 / 登出，会话被服务端撤销
 *   expired          —— 4010：纯过期且刷新失败
 *   password-changed —— 本端主动改密成功后的引导
 * 网络异常不会带 reason（不会被误报为「其他位置登录」）。
 */
const REASON_NOTICES: Record<string, string> = {
  invalidated:
    "登录状态已失效（账号可能已在其他位置登录或密码已修改），请重新登录。",
  expired: "登录状态已过期，请重新登录。",
  "password-changed": "密码已修改，请使用新密码登录。",
};
const sessionNotice = computed(() => {
  const raw = route.query.reason;
  const reason = typeof raw === "string" ? raw : "";
  return REASON_NOTICES[reason] ?? "";
});

/** 三面板滑窗：login 0 → register -33.3% → reset -66.7%（每格 1/3 宽） */
const sliderTransform = computed(() => {
  const offset =
    mode.value === "register" ? 1 : mode.value === "reset" ? 2 : 0;
  return `translateX(-${(offset * 100) / 3}%)`;
});

const accessLabel = computed(() => {
  if (mode.value === "register") return "ACCESS / 02";
  if (mode.value === "reset") return "ACCESS / 03";
  return "ACCESS / 01";
});

function setMode(nextMode: AuthMode): void {
  if (loading.value || mode.value === nextMode) {
    return;
  }

  mode.value = nextMode;
}

watch(mode, () => {
  errors.value = {};
  submitError.value = "";
  verifyNotice.value = "";
  if (mode.value !== "login") {
    loginNotice.value = "";
  }
});

const codeButtonLabel = computed(() => {
  if (codeCooldown.value > 0) {
    return `${codeCooldown.value}s 后重发`;
  }
  return sendingCode.value ? "发送中…" : "获取验证码";
});

async function sendCode(): Promise<void> {
  if (sendingCode.value || codeCooldown.value > 0 || loading.value) {
    return;
  }

  submitError.value = "";
  verifyNotice.value = "";
  // 注册码与找回密码码走不同的后端接口（purpose 不同、互不覆盖）
  const emailError =
    mode.value === "reset"
      ? validateResetForm(resetForm).email
      : validateRegisterForm(registerForm).email;
  if (emailError) {
    errors.value = { ...errors.value, email: emailError };
    return;
  }
  const nextErrors = { ...errors.value };
  delete nextErrors.email;
  errors.value = nextErrors;

  sendingCode.value = true;
  try {
    if (mode.value === "reset") {
      await sendResetCode(resetForm.email.trim());
    } else {
      await sendVerifyCode(registerForm.email.trim());
    }
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
    submitError.value =
      error instanceof AuthRequestError
        ? error.message
        : "服务暂时不可用，请稍后重试";
  } finally {
    sendingCode.value = false;
  }
}

/**
 * 找回密码提交（issue #85）。
 * 成功后后端已撤销该用户全部会话；前端回登录面板并提示用新密码登录。
 */
async function handleResetSubmit(): Promise<void> {
  const validation = validateResetForm(resetForm);
  if (Object.keys(validation).length > 0) {
    errors.value = validation;
    return;
  }

  loading.value = true;
  try {
    await resetPassword(toResetPayload(resetForm));
    setMode("login");
    loginNotice.value = "密码已重置，请使用新密码登录。";
  } catch (error: unknown) {
    submitError.value =
      error instanceof AuthRequestError
        ? error.message
        : "服务暂时不可用，请稍后重试";
  } finally {
    loading.value = false;
  }
}

async function handleSubmit(): Promise<void> {
  if (loading.value) return;

  if (mode.value === "reset") {
    await handleResetSubmit();
    return;
  }

  errors.value = {};
  submitError.value = "";
  verifyNotice.value = "";
  loading.value = true;

  try {
    const form = mode.value === "login" ? loginForm : registerForm;
    const result = await submitAuth(
      mode.value,
      form,
      {
        login,
        register,
        saveSession: saveAuthSession,
        navigate: (path) => transition(() => router.push(path)),
      },
      redirect.value,
    );

    if (!result.ok) {
      errors.value = result.errors;
    }
  } catch (error: unknown) {
    submitError.value =
      error instanceof AuthRequestError
        ? error.message
        : "服务暂时不可用，请稍后重试";
  } finally {
    loading.value = false;
  }
}

onUnmounted(() => {
  if (cooldownTimer !== undefined) {
    clearInterval(cooldownTimer);
    cooldownTimer = undefined;
  }
});
</script>
<template>
  <main>
    <section class="auth-view" aria-label="账号入口">
      <div class="brand-panel">
        <div class="auth-poster" :data-mode="mode">
          <div class="poster-top mono">
            <span><i class="status-dot"></i> SIGNAL IS ON.</span
            ><span>YA’AN / CN</span>
          </div>
          <h1>
            <span class="title-mask"
              ><span class="title-line">把未知</span></span
            ><span class="title-mask"
              ><span class="title-line">做成<span>现实。</span></span></span
            >
          </h1>
          <div class="poster-lower">
            <div class="poster-copy">
              <span class="mono">THINK → BUILD → CONNECT</span>
              <p>
                一行代码，一个想法。<br />在这里，让好奇心<br />有一个真实的落点。
              </p>
            </div>
            <div class="signal-mark" aria-hidden="true">
              <i style="--r: 0deg"></i><i style="--r: 45deg"></i
              ><i style="--r: 90deg"></i><i style="--r: 135deg"></i><b></b>
            </div>
          </div>
          <div class="poster-foot mono">
            <span>NOT JUST AN IDEA. A START.</span><span>↓</span>
          </div>
          <div class="signal-scanner" aria-hidden="true"></div>
          <div class="signal-halo" aria-hidden="true"></div>
        </div>
      </div>
      <div class="auth-space">
        <div class="auth-space-top mono">
          <span>{{ accessLabel }}</span
          ><span class="small-cross" aria-hidden="true">+</span>
        </div>
        <div class="form-tabs" role="group" aria-label="登录与注册切换">
          <button
            id="login-tab"
            :aria-pressed="mode === 'login'"
            :disabled="loading"
            @click="setMode('login')"
          >
            登录</button
          ><button
            id="register-tab"
            :aria-pressed="mode === 'register'"
            :disabled="loading"
            @click="setMode('register')"
          >
            注册</button
          ><span class="tab-line"></span>
        </div>
        <p v-if="showCompatNotice" class="compat-notice" role="status">
          这是旧版开屏的兼容登录页，当前已统一使用二维认证界面。登录、注册功能正常。
        </p>
        <div class="form-window">
          <div class="form-slider" :style="{ transform: sliderTransform }">
            <section
              class="auth-pane"
              id="login-pane"
              :inert="mode !== 'login'"
              :aria-hidden="mode !== 'login'"
            >
              <h2>欢迎回来。</h2>
              <p class="form-intro">你的下一个想法，从这里继续。</p>
              <p
                v-if="sessionNotice"
                class="session-notice"
                role="status"
              >
                {{ sessionNotice }}
              </p>
              <p v-if="loginNotice" class="session-notice" role="status">
                {{ loginNotice }}
              </p>
              <form
                class="auth-form"
                :aria-labelledby="'login-tab'"
                novalidate
                @submit.prevent="mode === 'login' && handleSubmit()"
              >
                <div class="field-group">
                  <div class="field-meta">
                    <label for="username">用户名</label>
                    <span>USERNAME</span>
                  </div>
                  <input
                    id="username"
                    v-model="loginForm.username"
                    name="username"
                    type="text"
                    autocomplete="username"
                    placeholder="输入用户名"
                    :aria-invalid="Boolean(errors.username)"
                    :aria-describedby="
                      errors.username ? 'username-error' : undefined
                    "
                  />
                  <p
                    v-if="errors.username"
                    id="username-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.username }}
                  </p>
                </div>

                <div class="field-group">
                  <div class="field-meta">
                    <label for="login-password">密码</label>
                    <span>PASSWORD</span>
                  </div>
                  <div class="password-field">
                    <input
                      id="login-password"
                      v-model="loginForm.password"
                      name="password"
                      :type="passwordVisible.login ? 'text' : 'password'"
                      autocomplete="current-password"
                      placeholder="输入密码"
                      :aria-invalid="Boolean(errors.password)"
                      :aria-describedby="
                        errors.password ? 'login-password-error' : undefined
                      "
                    /><button
                      type="button"
                      :aria-label="
                        passwordVisible.login ? '隐藏密码' : '显示密码'
                      "
                      :aria-pressed="passwordVisible.login"
                      @click="passwordVisible.login = !passwordVisible.login"
                    >
                      {{ passwordVisible.login ? "隐藏" : "显示" }}
                    </button>
                  </div>
                  <p
                    v-if="errors.password"
                    id="login-password-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.password }}
                  </p>
                </div>
                <div class="form-meta">
                  <span>用户名或注册邮箱均可登录</span>
                  <button
                    type="button"
                    class="plain-link"
                    :disabled="loading"
                    @click="setMode('reset')"
                  >
                    忘记密码？
                  </button>
                </div>

                <div
                  v-if="submitError && mode === 'login'"
                  class="submit-error"
                  role="alert"
                >
                  {{ submitError }}
                </div>
                <button
                  class="action-button"
                  type="submit"
                  :disabled="loading"
                  :aria-busy="loading"
                >
                  <span>{{ loading ? "正在校验…" : "进入工作台" }}</span
                  ><b aria-hidden="true">↗</b>
                </button>
              </form>
              <div class="form-divider"><span>或者，开启你的新章节</span></div>
              <button
                type="button"
                class="outline-button"
                :disabled="loading"
                @click="setMode('register')"
              >
                创建工作室账号 <span>→</span>
              </button>
              <p class="form-bottom">
                还没有账号？<button
                  class="plain-link"
                  :disabled="loading"
                  @click="setMode('register')"
                >
                  开启新章节 ↗
                </button>
              </p>
            </section>
            <section
              class="auth-pane"
              id="register-pane"
              :inert="mode !== 'register'"
              :aria-hidden="mode !== 'register'"
            >
              <h2>你好，新同伴。</h2>
              <p class="form-intro">带上好奇心，剩下的我们一起探索。</p>
              <form
                class="auth-form"
                :aria-labelledby="'register-tab'"
                novalidate
                @submit.prevent="mode === 'register' && handleSubmit()"
              >
                <div class="field-group">
                  <div class="field-meta">
                    <label for="name">姓名</label>
                    <span>NAME</span>
                  </div>
                  <input
                    id="name"
                    v-model="registerForm.name"
                    name="name"
                    type="text"
                    autocomplete="name"
                    placeholder="输入 3–20 个字符"
                    :aria-invalid="Boolean(errors.name)"
                    :aria-describedby="errors.name ? 'name-error' : undefined"
                  />
                  <p
                    v-if="errors.name"
                    id="name-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.name }}
                  </p>
                </div>

                <div class="field-group">
                  <div class="field-meta">
                    <label for="email">邮箱</label>
                    <span>EMAIL</span>
                  </div>
                  <input
                    id="email"
                    v-model="registerForm.email"
                    name="email"
                    type="email"
                    autocomplete="email"
                    placeholder="name@example.com"
                    :aria-invalid="Boolean(errors.email)"
                    :aria-describedby="errors.email ? 'email-error' : undefined"
                  />
                  <p
                    v-if="errors.email"
                    id="email-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.email }}
                  </p>
                </div>

                <div>
                  <div class="field-group">
                    <div class="field-meta">
                      <label for="register-password">密码</label>
                      <span>PASSWORD</span>
                    </div>
                    <div class="password-field">
                      <input
                        id="register-password"
                        v-model="registerForm.password"
                        name="password"
                        :type="passwordVisible.register ? 'text' : 'password'"
                        autocomplete="new-password"
                        placeholder="6–20 位密码"
                        :aria-invalid="Boolean(errors.password)"
                        :aria-describedby="
                          errors.password
                            ? 'register-password-error'
                            : undefined
                        "
                      /><button
                        type="button"
                        :aria-label="
                          passwordVisible.register ? '隐藏密码' : '显示密码'
                        "
                        :aria-pressed="passwordVisible.register"
                        @click="
                          passwordVisible.register = !passwordVisible.register
                        "
                      >
                        {{ passwordVisible.register ? "隐藏" : "显示" }}
                      </button>
                    </div>
                    <p
                      v-if="errors.password"
                      id="register-password-error"
                      class="field-error"
                      aria-live="polite"
                    >
                      {{ errors.password }}
                    </p>
                  </div>

                  <div class="field-group">
                    <div class="field-meta">
                      <label for="confirm-password">确认密码</label>
                      <span>CONFIRM</span>
                    </div>
                    <div class="password-field">
                      <input
                        id="confirm-password"
                        v-model="registerForm.confirmPassword"
                        name="confirmPassword"
                        :type="passwordVisible.confirm ? 'text' : 'password'"
                        autocomplete="new-password"
                        placeholder="再次输入密码"
                        :aria-invalid="Boolean(errors.confirmPassword)"
                        :aria-describedby="
                          errors.confirmPassword
                            ? 'confirm-password-error'
                            : undefined
                        "
                      /><button
                        type="button"
                        :aria-label="
                          passwordVisible.confirm ? '隐藏密码' : '显示密码'
                        "
                        :aria-pressed="passwordVisible.confirm"
                        @click="
                          passwordVisible.confirm = !passwordVisible.confirm
                        "
                      >
                        {{ passwordVisible.confirm ? "隐藏" : "显示" }}
                      </button>
                    </div>
                    <p
                      v-if="errors.confirmPassword"
                      id="confirm-password-error"
                      class="field-error"
                      aria-live="polite"
                    >
                      {{ errors.confirmPassword }}
                    </p>
                  </div>
                </div>

                <div class="field-group verification-row">
                  <div class="field-meta">
                    <label for="code">邮箱验证码</label>
                    <span>CODE</span>
                  </div>
                  <div class="code-field">
                    <input
                      id="code"
                      v-model="registerForm.code"
                      name="code"
                      type="text"
                      autocomplete="one-time-code"
                      inputmode="numeric"
                      maxlength="6"
                      placeholder="6 位数字"
                      :aria-invalid="Boolean(errors.code)"
                      :aria-describedby="
                        errors.code ? 'code-error code-status' : 'code-status'
                      "
                    />
                    <button
                      id="code-status"
                      type="button"
                      :disabled="sendingCode || codeCooldown > 0 || loading"
                      @click="sendCode"
                    >
                      {{ codeButtonLabel }}
                    </button>
                  </div>
                  <p
                    v-if="errors.code"
                    id="code-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.code }}
                  </p>
                  <p v-if="verifyNotice" class="code-notice" role="status">
                    {{ verifyNotice }}
                  </p>
                </div>

                <div
                  v-if="submitError && mode === 'register'"
                  class="submit-error"
                  role="alert"
                >
                  {{ submitError }}
                </div>
                <button
                  class="action-button"
                  type="submit"
                  :disabled="loading"
                  :aria-busy="loading"
                >
                  <span>{{ loading ? "正在校验…" : "创建账户" }}</span
                  ><b aria-hidden="true">↗</b>
                </button>
              </form>
              <p class="form-bottom">
                已经有账号？<button
                  class="plain-link"
                  :disabled="loading"
                  @click="setMode('login')"
                >
                  返回登录 ↗
                </button>
              </p>
            </section>
            <section
              class="auth-pane"
              id="reset-pane"
              :inert="mode !== 'reset'"
              :aria-hidden="mode !== 'reset'"
            >
              <h2>找回密码。</h2>
              <p class="form-intro">
                验证码会发送到你的绑定邮箱；邮箱不可更换。
              </p>
              <form
                class="auth-form"
                novalidate
                @submit.prevent="mode === 'reset' && handleSubmit()"
              >
                <div class="field-group">
                  <div class="field-meta">
                    <label for="reset-email">绑定邮箱</label>
                    <span>EMAIL</span>
                  </div>
                  <input
                    id="reset-email"
                    v-model="resetForm.email"
                    name="resetEmail"
                    type="email"
                    autocomplete="email"
                    placeholder="name@example.com"
                    :aria-invalid="Boolean(errors.email)"
                    :aria-describedby="
                      errors.email ? 'reset-email-error' : undefined
                    "
                  />
                  <p
                    v-if="errors.email"
                    id="reset-email-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.email }}
                  </p>
                </div>

                <div class="field-group verification-row">
                  <div class="field-meta">
                    <label for="reset-code">邮箱验证码</label>
                    <span>CODE</span>
                  </div>
                  <div class="code-field">
                    <input
                      id="reset-code"
                      v-model="resetForm.code"
                      name="resetCode"
                      type="text"
                      autocomplete="one-time-code"
                      inputmode="numeric"
                      maxlength="6"
                      placeholder="6 位数字"
                      :aria-invalid="Boolean(errors.code)"
                      :aria-describedby="
                        errors.code
                          ? 'reset-code-error reset-code-status'
                          : 'reset-code-status'
                      "
                    />
                    <button
                      id="reset-code-status"
                      type="button"
                      :disabled="sendingCode || codeCooldown > 0 || loading"
                      @click="sendCode"
                    >
                      {{ codeButtonLabel }}
                    </button>
                  </div>
                  <p
                    v-if="errors.code"
                    id="reset-code-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.code }}
                  </p>
                  <p v-if="verifyNotice" class="code-notice" role="status">
                    {{ verifyNotice }}
                  </p>
                </div>

                <div class="field-group">
                  <div class="field-meta">
                    <label for="reset-password">新密码</label>
                    <span>PASSWORD</span>
                  </div>
                  <div class="password-field">
                    <input
                      id="reset-password"
                      v-model="resetForm.password"
                      name="resetPassword"
                      :type="passwordVisible.resetNew ? 'text' : 'password'"
                      autocomplete="new-password"
                      placeholder="6–20 位新密码"
                      :aria-invalid="Boolean(errors.password)"
                      :aria-describedby="
                        errors.password ? 'reset-password-error' : undefined
                      "
                    /><button
                      type="button"
                      :aria-label="
                        passwordVisible.resetNew ? '隐藏密码' : '显示密码'
                      "
                      :aria-pressed="passwordVisible.resetNew"
                      @click="
                        passwordVisible.resetNew = !passwordVisible.resetNew
                      "
                    >
                      {{ passwordVisible.resetNew ? "隐藏" : "显示" }}
                    </button>
                  </div>
                  <p
                    v-if="errors.password"
                    id="reset-password-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.password }}
                  </p>
                </div>

                <div class="field-group">
                  <div class="field-meta">
                    <label for="reset-confirm-password">确认新密码</label>
                    <span>CONFIRM</span>
                  </div>
                  <div class="password-field">
                    <input
                      id="reset-confirm-password"
                      v-model="resetForm.confirmPassword"
                      name="resetConfirmPassword"
                      :type="
                        passwordVisible.resetConfirm ? 'text' : 'password'
                      "
                      autocomplete="new-password"
                      placeholder="再次输入新密码"
                      :aria-invalid="Boolean(errors.confirmPassword)"
                      :aria-describedby="
                        errors.confirmPassword
                          ? 'reset-confirm-password-error'
                          : undefined
                      "
                    /><button
                      type="button"
                      :aria-label="
                        passwordVisible.resetConfirm ? '隐藏密码' : '显示密码'
                      "
                      :aria-pressed="passwordVisible.resetConfirm"
                      @click="
                        passwordVisible.resetConfirm =
                          !passwordVisible.resetConfirm
                      "
                    >
                      {{ passwordVisible.resetConfirm ? "隐藏" : "显示" }}
                    </button>
                  </div>
                  <p
                    v-if="errors.confirmPassword"
                    id="reset-confirm-password-error"
                    class="field-error"
                    aria-live="polite"
                  >
                    {{ errors.confirmPassword }}
                  </p>
                </div>

                <div
                  v-if="submitError && mode === 'reset'"
                  class="submit-error"
                  role="alert"
                >
                  {{ submitError }}
                </div>
                <button
                  class="action-button"
                  type="submit"
                  :disabled="loading"
                  :aria-busy="loading"
                >
                  <span>{{ loading ? "正在重置…" : "重置密码" }}</span
                  ><b aria-hidden="true">↗</b>
                </button>
              </form>
              <p class="form-bottom">
                想起密码了？<button
                  class="plain-link"
                  :disabled="loading"
                  @click="setMode('login')"
                >
                  返回登录 ↗
                </button>
              </p>
            </section>
          </div>
        </div>
        <span class="auth-caption mono">SMALL STARTS. REAL POSSIBILITIES.</span>
      </div>
    </section>
  </main>
</template>
<style scoped>
.brand-panel {
  display: flex;
  min-width: 0;
}
.auth-poster {
  width: 100%;
}
</style>
