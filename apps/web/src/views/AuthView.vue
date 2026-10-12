<script setup lang="ts">
import { computed, inject, onUnmounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { AuthRequestError, login, register, sendVerifyCode } from "@/api/auth";
import {
  type LoginForm,
  type RegisterForm,
  validateRegisterForm,
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
});
const loading = inject(SIGNAL_BUSY, ref(false));
const sendingCode = ref(false);
const codeCooldown = ref(0);
const verifyNotice = ref("");
const errors = ref<Record<string, string>>({});
const submitError = ref("");

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
  const emailError = validateRegisterForm(registerForm).email;
  if (emailError) {
    errors.value = { ...errors.value, email: emailError };
    return;
  }
  const nextErrors = { ...errors.value };
  delete nextErrors.email;
  errors.value = nextErrors;

  sendingCode.value = true;
  try {
    await sendVerifyCode(registerForm.email.trim());
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

async function handleSubmit(): Promise<void> {
  if (loading.value) return;
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
          <span>{{ mode === "login" ? "ACCESS / 01" : "ACCESS / 02" }}</span
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
          <div
            class="form-slider"
            :style="{
              transform:
                mode === 'register' ? 'translateX(-50%)' : 'translateX(0)',
            }"
          >
            <section
              class="auth-pane"
              id="login-pane"
              :inert="mode !== 'login'"
              :aria-hidden="mode !== 'login'"
            >
              <h2>欢迎回来。</h2>
              <p class="form-intro">你的下一个想法，从这里继续。</p>
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
                  <span>用户名或注册邮箱均可登录</span><span>密码 6–20 位</span>
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
