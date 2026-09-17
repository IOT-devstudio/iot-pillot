<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { AuthRequestError, login, register } from "@/api/auth";
import type { LoginForm, RegisterForm } from "@/auth/form";
import { saveAuthSession } from "@/auth/session";
import { submitAuth, type AuthMode } from "@/auth/submit";

const router = useRouter();
const route = useRoute();

/**
 * 从 3D 开屏跳过来时带的标记。
 *
 * 3D 渲染失败会把用户转投到这个后备页（见 modules/opener/views/StudioOpener.vue），
 * 但转投本身是静默的——用户只看到「3D 闪一下就没了」。所以由 3D 侧带上
 * ?fallback=webgl，这里读出来解释原因，并给一个回开屏的出口。
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

const mode = ref<AuthMode>("login");
const loading = ref(false);
const errors = ref<Record<string, string>>({});
const submitError = ref("");

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
  errors.value = {};
  submitError.value = "";
}

async function handleSubmit(): Promise<void> {
  errors.value = {};
  submitError.value = "";
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
        navigate: (path) => router.push(path),
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
</script>

<template>
  <main class="auth-page">
    <aside class="brand-panel" aria-label="iot-pillot 全栈工作室招新">
      <div class="brand-panel__grid" aria-hidden="true"></div>

      <header class="brand-header">
        <a class="wordmark" href="/" aria-label="iot-pillot 首页">
          <span class="wordmark__mark" aria-hidden="true">i/o</span>
          <span>iot-pillot</span>
        </a>
        <span class="edition">2026 春季招新</span>
      </header>

      <section class="brand-copy">
        <p class="eyebrow">全栈招新 · 01</p>
        <h1>从界面到服务，<br />把想法做成作品。</h1>
        <p class="brand-copy__description">
          iot 全栈工作室以前端与后端开发为核心，围绕真实问题做项目：拆需求、写代码、联调服务，再把结果交付给真正的用户。
        </p>
      </section>

      <div class="schematic" aria-hidden="true">
        <svg viewBox="0 0 620 390" role="presentation">
          <g class="schematic__fine-lines">
            <path d="M14 313H600M39 340H360M91 48v284M521 48v284" />
            <path d="M91 91h104M91 246h104M425 91h96M425 246h96" />
            <circle cx="91" cy="91" r="5" />
            <circle cx="91" cy="246" r="5" />
            <circle cx="521" cy="91" r="5" />
            <circle cx="521" cy="246" r="5" />
          </g>
          <g class="schematic__main-lines">
            <rect x="115" y="116" width="118" height="124" rx="2" />
            <rect x="251" y="80" width="118" height="196" rx="2" />
            <rect x="387" y="116" width="118" height="124" rx="2" />
            <path d="M233 178h18m118 0h18" />
            <path d="M142 145h64m-64 31h38m-38 31h64m-64 31h42" />
            <path d="M278 116h64m-64 31h38m-38 31h64m-64 31h42m-42 31h64" />
            <path d="M414 145h64m-64 31h38m-38 31h64m-64 31h42" />
            <path d="M174 116V91m0 149v25m136-195V51m0 225v25m136-185V91m0 149v25" />
            <circle cx="310" cy="178" r="18" />
            <path d="M301 178h18m-9-9v18" />
          </g>
          <g class="schematic__labels">
            <text x="115" y="103">01 / 界面</text>
            <text x="251" y="67">02 / 服务</text>
            <text x="387" y="103">03 / 数据</text>
            <text x="19" y="329">全栈练习 · 比例 1:2</text>
          </g>
          <g class="schematic__annotations">
            <path d="M431 82c40-24 87-13 112 17" />
            <path d="M535 93l12 8-15 4" />
            <path d="M112 280c71 23 166 28 277 0" />
            <text x="431" y="68">一起完成</text>
          </g>
        </svg>
      </div>

      <footer class="brand-footer">
        <span>IoT 全栈工作室 · 项目手册</span>
        <span>编号 001—026</span>
      </footer>
    </aside>

    <section class="form-panel" aria-labelledby="auth-title">
      <div class="form-shell">
        <div class="mobile-wordmark" aria-hidden="true">
          <span>i/o</span>
          iot-pillot
        </div>

        <div class="section-index" aria-hidden="true">
          <span>招新入口</span>
          <span>01 / 02</span>
        </div>

        <!--
          3D 渲染失败转投过来的说明。用 role="status" 而不是 alert：
          这是「告知」不是「出错」，不该抢读屏器的紧急通道。

          刻意不放「返回开屏」链接：WebGL 不可用是环境特征而非偶发故障，
          点回去会立刻被再次转投，形成死循环。与其给一个必然失败的按钮，
          不如直说这个浏览器看不了开屏——想再试的话刷新页面即可。
        -->
        <p v-if="showCompatNotice" class="compat-notice" role="status">
          <span class="compat-notice__mark" aria-hidden="true">※</span>
          <span>
            当前浏览器未能启用 WebGL，开屏动画无法显示，已切换到兼容登录页。
            下面的表单功能完全可用。
          </span>
        </p>

        <header class="form-heading">
          <p>{{ mode === "login" ? "欢迎回来" : "建立成员档案" }}</p>
          <h2 id="auth-title">
            {{ mode === "login" ? "回到工作台" : "加入 iot 全栈工作室" }}
          </h2>
          <span>
            {{
              mode === "login"
                ? "使用工作室账号继续协作。"
                : "填写资料，加入一群认真做作品的人。"
            }}
          </span>
        </header>

        <div class="mode-switch" role="group" aria-label="认证方式">
          <button
            id="login-tab"
            type="button"
            :aria-current="mode === 'login' ? 'page' : undefined"
            :disabled="loading"
            @click="setMode('login')"
          >
            登录
          </button>
          <button
            id="register-tab"
            type="button"
            :aria-current="mode === 'register' ? 'page' : undefined"
            :disabled="loading"
            @click="setMode('register')"
          >
            注册
          </button>
        </div>

        <form
          class="auth-form"
          :aria-labelledby="mode === 'login' ? 'login-tab' : 'register-tab'"
          novalidate
          @submit.prevent="handleSubmit"
        >
          <template v-if="mode === 'login'">
            <div class="field-group">
              <div class="field-meta">
                <label for="username">用户名</label>
                <span>用户名</span>
              </div>
              <input
                id="username"
                v-model="loginForm.username"
                name="username"
                type="text"
                autocomplete="username"
                placeholder="输入用户名"
                :aria-invalid="Boolean(errors.username)"
                :aria-describedby="errors.username ? 'username-error' : undefined"
              />
              <p v-if="errors.username" id="username-error" class="field-error" aria-live="polite">
                {{ errors.username }}
              </p>
            </div>

            <div class="field-group">
              <div class="field-meta">
                <label for="login-password">密码</label>
                <span>密码</span>
              </div>
              <input
                id="login-password"
                v-model="loginForm.password"
                name="password"
                type="password"
                autocomplete="current-password"
                placeholder="输入密码"
                :aria-invalid="Boolean(errors.password)"
                :aria-describedby="errors.password ? 'login-password-error' : undefined"
              />
              <p
                v-if="errors.password"
                id="login-password-error"
                class="field-error"
                aria-live="polite"
              >
                {{ errors.password }}
              </p>
            </div>
          </template>

          <template v-else>
            <div class="field-group">
              <div class="field-meta">
                <label for="name">姓名</label>
                <span>姓名</span>
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
              <p v-if="errors.name" id="name-error" class="field-error" aria-live="polite">
                {{ errors.name }}
              </p>
            </div>

            <div class="field-group">
              <div class="field-meta">
                <label for="email">邮箱</label>
                <span>邮箱</span>
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
              <p v-if="errors.email" id="email-error" class="field-error" aria-live="polite">
                {{ errors.email }}
              </p>
            </div>

            <div class="field-row">
              <div class="field-group">
                <div class="field-meta">
                  <label for="register-password">密码</label>
                  <span>密码</span>
                </div>
                <input
                  id="register-password"
                  v-model="registerForm.password"
                  name="password"
                  type="password"
                  autocomplete="new-password"
                  placeholder="6–20 位密码"
                  :aria-invalid="Boolean(errors.password)"
                  :aria-describedby="errors.password ? 'register-password-error' : undefined"
                />
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
                  <span>确认密码</span>
                </div>
                <input
                  id="confirm-password"
                  v-model="registerForm.confirmPassword"
                  name="confirmPassword"
                  type="password"
                  autocomplete="new-password"
                  placeholder="再次输入密码"
                  :aria-invalid="Boolean(errors.confirmPassword)"
                  :aria-describedby="errors.confirmPassword ? 'confirm-password-error' : undefined"
                />
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

            <div class="field-group">
              <div class="field-meta">
                <label for="code">邮箱验证码</label>
                <span>验证码</span>
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
                  :aria-describedby="errors.code ? 'code-error code-status' : 'code-status'"
                />
                <button id="code-status" type="button" disabled>
                  获取验证码 · 暂未开放
                </button>
              </div>
              <p v-if="errors.code" id="code-error" class="field-error" aria-live="polite">
                {{ errors.code }}
              </p>
            </div>
          </template>

          <div
            v-if="submitError"
            class="submit-error"
            role="alert"
            aria-live="assertive"
          >
            <span aria-hidden="true">!</span>
            {{ submitError }}
          </div>

          <button class="submit-button" type="submit" :disabled="loading" :aria-busy="loading">
            <span>{{ loading ? "正在校验…" : mode === "login" ? "进入工作台" : "创建账户" }}</span>
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M5 12h13m-5-5 5 5-5 5" />
            </svg>
          </button>
        </form>

        <p class="form-note">
          <span aria-hidden="true">※</span>
          {{
            mode === "login"
              ? "登录即代表你同意遵守工作室协作规范。"
              : "验证码服务暂未开放，注册资料会在后续招新流程中使用。"
          }}
        </p>
      </div>
    </section>
  </main>
</template>

<style scoped>
:global(*) {
  box-sizing: border-box;
}

:global(html),
:global(body),
:global(#app) {
  min-width: 320px;
  min-height: 100%;
  margin: 0;
}

:global(body) {
  color: #102b4e;
  background: #f3efe4;
  font-family: "Avenir Next", "PingFang SC", "Microsoft YaHei", sans-serif;
}

button,
input {
  font: inherit;
}

.auth-page {
  --ink: #102b4e;
  --ink-soft: #52657b;
  --blue: #164d80;
  --blue-bright: #1e659f;
  --paper: #f3efe4;
  --paper-deep: #e8e0ce;
  --white: #fffdf7;
  --red: #b84235;
  display: grid;
  grid-template-columns: minmax(430px, 46%) minmax(390px, 54%);
  grid-template-rows: minmax(0, 1fr);
  height: 100dvh;
  min-height: 100vh;
  overflow: hidden;
  background: var(--paper);
}

.brand-panel {
  position: relative;
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  padding: clamp(30px, 4.2vw, 68px);
  overflow: hidden;
  color: #f6f0df;
  background: var(--blue);
  isolation: isolate;
}

.brand-panel::before {
  position: absolute;
  inset: 0;
  z-index: -1;
  background:
    radial-gradient(circle at 76% 18%, rgb(112 171 211 / 18%), transparent 27%),
    linear-gradient(145deg, rgb(255 255 255 / 3%), transparent 52%);
  content: "";
}

.brand-panel__grid {
  position: absolute;
  inset: 0;
  z-index: -1;
  background-image:
    linear-gradient(rgb(211 232 244 / 9%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(211 232 244 / 9%) 1px, transparent 1px),
    linear-gradient(rgb(211 232 244 / 5%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(211 232 244 / 5%) 1px, transparent 1px);
  background-position: -1px -1px;
  background-size: 80px 80px, 80px 80px, 16px 16px, 16px 16px;
  mask-image: linear-gradient(to bottom, black 0%, rgb(0 0 0 / 76%) 82%, transparent 100%);
}

.brand-header,
.brand-footer,
.section-index,
.field-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand-header {
  animation: reveal 700ms ease-out both;
}

.wordmark {
  display: inline-flex;
  gap: 12px;
  align-items: center;
  color: inherit;
  font-family: "Iowan Old Style", "Songti SC", STSong, serif;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: -0.02em;
  text-decoration: none;
}

.wordmark__mark,
.mobile-wordmark span {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border: 1px solid currentcolor;
  border-radius: 50%;
  font-family: "Avenir Next", sans-serif;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.edition,
.section-index,
.field-meta span,
.brand-footer,
.eyebrow {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.17em;
  text-transform: uppercase;
}

.edition {
  padding-bottom: 5px;
  border-bottom: 1px solid rgb(246 240 223 / 55%);
}

.brand-copy {
  position: relative;
  z-index: 1;
  max-width: 610px;
  margin-top: clamp(60px, 10vh, 118px);
  animation: reveal 700ms 100ms ease-out both;
}

.eyebrow {
  display: flex;
  gap: 12px;
  align-items: center;
  margin: 0 0 20px;
  color: #c3d8e8;
}

.eyebrow::before {
  width: 34px;
  height: 1px;
  background: var(--red);
  content: "";
}

.brand-copy h1 {
  margin: 0;
  font-family: "Iowan Old Style", "Songti SC", STSong, serif;
  font-size: clamp(44px, 5.3vw, 78px);
  font-weight: 600;
  line-height: 1.13;
  letter-spacing: -0.045em;
}

.brand-copy__description {
  max-width: 450px;
  margin: 28px 0 0;
  color: rgb(246 240 223 / 73%);
  font-size: 14px;
  line-height: 1.85;
}

.schematic {
  position: absolute;
  right: -7%;
  bottom: 7%;
  width: min(84%, 690px);
  opacity: 0.78;
  animation: reveal 900ms 220ms ease-out both;
}

.schematic svg {
  display: block;
  width: 100%;
}

.schematic__fine-lines,
.schematic__main-lines,
.schematic__annotations {
  fill: none;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.schematic__fine-lines {
  stroke: rgb(219 235 244 / 32%);
  stroke-width: 1;
}

.schematic__main-lines {
  stroke: rgb(231 241 246 / 65%);
  stroke-width: 1.4;
}

.schematic__labels {
  fill: rgb(231 241 246 / 56%);
  font-family: monospace;
  font-size: 9px;
  letter-spacing: 0.12em;
}

.schematic__annotations {
  stroke: #e77464;
  stroke-width: 2;
}

.schematic__annotations text {
  fill: #eaa196;
  font-family: "Kaiti SC", KaiTi, serif;
  font-size: 14px;
  stroke: none;
}

.brand-footer {
  z-index: 1;
  margin-top: auto;
  padding-top: 24px;
  color: rgb(246 240 223 / 62%);
  border-top: 1px solid rgb(246 240 223 / 18%);
}

.form-panel {
  position: relative;
  display: grid;
  height: 100%;
  min-height: 0;
  align-content: safe center;
  place-items: center;
  padding: clamp(36px, 7vw, 96px);
  overflow-y: auto;
  scrollbar-gutter: stable;
  background-color: var(--white);
  background-image:
    linear-gradient(rgb(22 77 128 / 3.5%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(22 77 128 / 3.5%) 1px, transparent 1px);
  background-size: 32px 32px;
}

.form-panel::before {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 18px;
  width: 1px;
  background: rgb(184 66 53 / 35%);
  content: "";
}

.form-shell {
  width: min(100%, 540px);
  margin: auto 0;
  animation: form-enter 620ms 80ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.mobile-wordmark {
  display: none;
}

.section-index {
  margin-bottom: clamp(54px, 8vh, 84px);
  padding-bottom: 11px;
  color: var(--ink-soft);
  border-bottom: 1px solid rgb(16 43 78 / 22%);
}

.section-index span:last-child {
  color: var(--red);
}

.form-heading p {
  margin: 0 0 11px;
  color: var(--red);
  font-family: "Kaiti SC", KaiTi, serif;
  font-size: 16px;
  transform: rotate(-1.5deg);
  transform-origin: left center;
}

.form-heading h2 {
  margin: 0;
  color: var(--ink);
  font-family: "Iowan Old Style", "Songti SC", STSong, serif;
  font-size: clamp(34px, 4vw, 50px);
  font-weight: 600;
  line-height: 1.12;
  letter-spacing: -0.035em;
}

.form-heading > span {
  display: block;
  margin-top: 13px;
  color: var(--ink-soft);
  font-size: 14px;
  line-height: 1.7;
}

.mode-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  margin: 38px 0 32px;
  border-bottom: 1px solid rgb(16 43 78 / 24%);
}

.mode-switch button {
  position: relative;
  padding: 13px 10px;
  color: #758394;
  border: 0;
  background: transparent;
  cursor: pointer;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.mode-switch button::after {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: var(--blue);
  content: "";
  opacity: 0;
  transform: scaleX(0.35);
  transition: opacity 180ms ease, transform 180ms ease;
}

.mode-switch button[aria-current="page"] {
  color: var(--blue);
}

.mode-switch button[aria-current="page"]::after {
  opacity: 1;
  transform: scaleX(1);
}

.mode-switch button:disabled {
  cursor: wait;
}

.auth-form {
  display: grid;
  gap: 24px;
}

.field-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}

.field-group {
  min-width: 0;
}

.field-meta {
  margin-bottom: 8px;
}

.field-meta label {
  color: var(--ink);
  font-size: 13px;
  font-weight: 700;
}

.field-meta span {
  color: #8b96a3;
  font-size: 8px;
}

.field-group input {
  width: 100%;
  height: 50px;
  padding: 0 14px;
  color: var(--ink);
  border: 1px solid #bfc6c6;
  border-radius: 2px;
  outline: 0;
  background: rgb(255 253 247 / 82%);
  font-size: 14px;
  transition: border-color 160ms ease, box-shadow 160ms ease, background 160ms ease;
}

.field-group input::placeholder {
  color: #9aa2aa;
}

.field-group input:hover {
  border-color: #8195a8;
}

.field-group input:focus-visible,
.mode-switch button:focus-visible,
.submit-button:focus-visible,
.wordmark:focus-visible {
  outline: 3px solid rgb(30 101 159 / 28%);
  outline-offset: 3px;
}

.field-group input:focus-visible {
  border-color: var(--blue-bright);
  outline: 0;
  background: #fffefb;
  box-shadow: 0 0 0 3px rgb(30 101 159 / 15%);
}

.field-group input[aria-invalid="true"] {
  border-color: var(--red);
  background: #fffaf5;
}

.field-error {
  margin: 7px 0 0;
  color: var(--red);
  font-size: 12px;
  line-height: 1.4;
}

/* 3D 渲染失败时的说明条。用蓝色系而非红色：这是「换了个方式」不是「出错了」。
   尺寸收紧到刚好不把表单挤出滚动条——右栏本就独立滚动，但为一行提示
   冒出一条滚动条会显得像故障。 */
.compat-notice {
  display: flex;
  gap: 7px;
  align-items: flex-start;
  margin: 0 0 14px;
  padding: 8px 12px;
  border: 1px solid color-mix(in srgb, var(--blue) 22%, transparent);
  background: color-mix(in srgb, var(--blue) 6%, transparent);
  color: var(--ink-soft);
  font-size: 12px;
  line-height: 1.45;
}

.compat-notice__mark {
  flex: none;
  color: var(--blue);
}

.field-error::before {
  margin-right: 6px;
  content: "↳";
}

.code-field {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 9px;
}

.code-field button {
  min-width: 168px;
  padding: 0 14px;
  color: #77828b;
  border: 1px dashed #aeb5b7;
  border-radius: 2px;
  background: #eeeae0;
  font-size: 12px;
  cursor: not-allowed;
}

.submit-error {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 12px 14px;
  color: #8e2f27;
  border-left: 2px solid var(--red);
  background: #f8ebe4;
  font-size: 13px;
  line-height: 1.5;
}

.submit-error span {
  display: grid;
  width: 20px;
  height: 20px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid currentcolor;
  border-radius: 50%;
  font-size: 11px;
  font-weight: 800;
}

.submit-button {
  display: flex;
  width: 100%;
  height: 54px;
  align-items: center;
  justify-content: space-between;
  margin-top: 4px;
  padding: 0 20px;
  overflow: hidden;
  color: #fffdf7;
  border: 1px solid var(--blue);
  border-radius: 2px;
  background: var(--blue);
  cursor: pointer;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.09em;
  transition: background 180ms ease, transform 180ms ease;
}

.submit-button svg {
  width: 21px;
  fill: none;
  stroke: currentcolor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.5;
  transition: transform 180ms ease;
}

.submit-button:hover:not(:disabled) {
  background: #0c3c69;
}

.submit-button:hover:not(:disabled) svg {
  transform: translateX(4px);
}

.submit-button:active:not(:disabled) {
  transform: translateY(1px);
}

.submit-button:disabled {
  cursor: wait;
  opacity: 0.67;
}

.form-note {
  display: flex;
  gap: 9px;
  margin: 22px 0 0;
  color: #74808d;
  font-size: 11px;
  line-height: 1.6;
}

.form-note span {
  color: var(--red);
}

@keyframes reveal {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes form-enter {
  from {
    opacity: 0;
    transform: translateX(16px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@media (max-width: 1060px) {
  .auth-page {
    grid-template-columns: minmax(390px, 42%) minmax(430px, 58%);
  }

  .brand-copy h1 {
    font-size: 49px;
  }

  .form-panel {
    padding: 48px;
  }

  .field-row {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 819px) {
  .auth-page {
    display: block;
    height: auto;
    min-height: 100vh;
    min-height: 100dvh;
    overflow: visible;
  }

  .brand-panel {
    display: none;
  }

  .form-panel {
    height: auto;
    min-height: 100vh;
    min-height: 100dvh;
    align-content: start;
    padding: 28px clamp(24px, 8vw, 62px) 44px;
    overflow: visible;
    scrollbar-gutter: auto;
  }

  .form-shell {
    margin: 0;
  }

  .form-panel::before {
    left: 12px;
  }

  .mobile-wordmark {
    display: flex;
    gap: 10px;
    align-items: center;
    margin-bottom: 38px;
    font-family: "Iowan Old Style", "Songti SC", STSong, serif;
    font-size: 18px;
    font-weight: 700;
  }

  .mobile-wordmark span {
    width: 32px;
    height: 32px;
    font-size: 8px;
  }

  .section-index {
    margin-bottom: 42px;
  }
}

@media (max-width: 480px) {
  .form-panel {
    place-items: start center;
    padding-right: 22px;
    padding-left: 30px;
  }

  .code-field {
    grid-template-columns: 1fr;
  }

  .code-field button {
    height: 43px;
  }

  .auth-form {
    gap: 20px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .brand-header,
  .brand-copy,
  .schematic,
  .form-shell {
    animation: none;
  }

  .mode-switch button::after,
  .field-group input,
  .submit-button,
  .submit-button svg {
    transition: none;
  }
}
</style>
