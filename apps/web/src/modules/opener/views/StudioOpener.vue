<script setup lang="ts">
/**
 * StudioOpener —— 3D 开屏视图（组装层）。
 *
 * 自己几乎不含逻辑，只做三件事：
 *   1. 把 canvas 宿主与面板元素交给 useStudioScene 驱动 3D 与动画
 *   2. 把 useAuthPanel 的状态接到 AuthPanel 上
 *   3. 在这里声明设计令牌（CSS 变量），供子组件用 var() 消费
 *
 * 拆分后的分工：
 *   config.ts / buildings.ts      → 所有可调参数与楼栋布局数据
 *   scene/*                       → 纯 Three.js / GSAP（不依赖 Vue）
 *   composables/useStudioScene.ts → 3D 与 Vue 生命周期的对接
 *   composables/useAuthPanel.ts   → 登录注册状态机
 *   components/*                  → 面板与字段的展示
 */
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";

import AuthPanel from "../components/AuthPanel.vue";
import { useAuthPanel } from "../composables/useAuthPanel";
import { useStudioScene } from "../composables/useStudioScene";
import { CONFIG } from "../config";

const route = useRoute();
const router = useRouter();

/**
 * 守卫挡下访客时会带上 ?redirect=<原路径>，登录成功后跳回去。
 * 取值交给 submitAuth 校验（只接受站内绝对路径），这里不做判断。
 */
const redirect = computed(() => {
  const raw = route.query.redirect;
  return typeof raw === "string" ? raw : undefined;
});

/** canvas 宿主；WebGLRenderer 的 canvas 会被 append 进来 */
const canvasHostRef = ref<HTMLElement | null>(null);
/** 面板外壳。GSAP 直接补间这个元素做滑入，所以它必须是稳定的 DOM 元素本身，
 *  而不是某个子组件的实例 */
const panelRef = ref<HTMLElement | null>(null);

const { panelVisible, webglFailed, animationSkipped, replayAnimation } =
  useStudioScene({
    canvasHostRef,
    panelRef,
    /**
     * 3D 渲染不出来时转投二维后备登录页——此时它是唯一可用的登录入口。
     * redirect 必须一起带过去，否则用户登录后会丢掉原来的目标。
     */
    onWebglFailed: () =>
      router.replace({ path: "/login", query: route.query }),
  });

// 顶层解构，模板里就能直接写 :mode="mode"（<script setup> 只对顶层 ref 自动解包）
const {
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
  submit,
  sendCode,
} = useAuthPanel(redirect);

/** 面板宽度交给 CSS 变量，这样窄屏媒体查询还能再压窄它 */
const panelStyle = computed<Record<string, string>>(() => ({
  "--panel-width": `${CONFIG.loginPanelWidth}px`,
}));
</script>

<template>
  <!--
    整屏容器：position: fixed 让它脱离 #app 的尺寸约束，天然不产生滚动条。
    3D canvas 铺满容器，登录面板绝对定位浮在它上面。
  -->
  <div class="studio-opener">
    <!--
      canvas 宿主。aria-hidden：纯装饰，读屏器只读面板表单。
      这里刻意不绑任何 pointer 事件 —— 场景本身不需要交互，也就无所谓
      「点击穿透到 3D」；面板上再加 .stop 是双保险。
    -->
    <div
      ref="canvasHostRef"
      class="studio-opener__canvas"
      aria-hidden="true"
    ></div>

    <!--
      WebGL 不可用时的提示。此时已经转投二维后备登录页，这句只在跳转
      完成前短暂可见——留着是为了万一导航失败还有反馈，不至于白屏。
    -->
    <p v-if="webglFailed" class="studio-opener__fallback" role="status">
      当前浏览器未能启用 WebGL，正在切换到备用登录页…
    </p>

    <!--
      跳过了开屏动画时的补救入口。
      系统开了「减少动态效果」时我们默认不播（正确的无障碍行为），但必须让人
      知道原因并且能一键播放，否则整个开屏动画在这台机器上等于消失。
    -->
    <button
      v-if="animationSkipped"
      type="button"
      class="studio-opener__replay"
      @pointerdown.stop
      @click.stop="replayAnimation"
    >
      ▶ 播放开屏动画
    </button>

    <!--
      面板外壳：定位 + 动画层。
      v-show 由 useStudioScene 打开（时间轴需要一个真实存在的 tween target），
      具体「什么时候看得见」由时间轴阶段 3 的 autoAlpha 决定。
      @pointerdown.stop / @click.stop / @wheel.stop：阻止事件冒泡到 3D 场景。
    -->
    <div
      v-show="panelVisible"
      ref="panelRef"
      class="studio-opener__panel"
      :style="panelStyle"
      @pointerdown.stop
      @click.stop
      @wheel.stop
    >
      <AuthPanel
        :mode="mode"
        :login-form="loginForm"
        :register-form="registerForm"
        :errors="errors"
        :notice="notice"
        :submit-error="submitError"
        :submitting="submitting"
        :sending-code="sendingCode"
        :code-cooldown="codeCooldown"
        :submit-label="submitLabel"
        :code-button-label="codeButtonLabel"
        @update:mode="setMode"
        @submit="submit"
        @send-code="sendCode"
      />
    </div>
  </div>
</template>

<style scoped>
/* ── 设计令牌 ──────────────────────────────────────────────────────
   定义在模块根元素上，靠 CSS 自定义属性的继承性传给 AuthPanel /
   AuthField，子组件因此不需要各自引一份样式变量。 */
.studio-opener {
  --opener-font: "Avenir Next", "PingFang SC", "Microsoft YaHei", sans-serif;
  --opener-serif: "Iowan Old Style", "Songti SC", STSong, serif;
  --opener-ink: #102b4e;
  --opener-soft: #52657b;
  --opener-muted: #9aa2aa;
  --opener-blue: #164d80;
  --opener-blue-deep: #0c3c69;
  --opener-blue-bright: #1e659f;
  --opener-danger: #b84235;
  --opener-success: #2e8b57;
  --opener-success-bg: rgb(46 139 87 / 10%);
  --opener-danger-bg: rgb(184 66 53 / 10%);
  --opener-border: #bfc6c6;
  --opener-divider: rgb(16 43 78 / 18%);
  --opener-tab: #758394;
  --opener-focus-ring: rgb(30 101 159 / 16%);
  --opener-input-bg: rgb(255 255 255 / 86%);
  --opener-card-bg: rgb(255 255 255 / 72%);
  --opener-card-border: rgb(16 43 78 / 12%);
  --opener-card-shadow:
    0 18px 48px rgb(16 43 78 / 16%),
    0 2px 8px rgb(16 43 78 / 8%);
  --opener-code-border: rgb(22 77 128 / 45%);
  --opener-code-bg: rgb(22 77 128 / 7%);
  --opener-code-border-disabled: rgb(139 150 163 / 45%);
  --opener-code-bg-disabled: rgb(139 150 163 / 10%);

  position: fixed;
  inset: 0;
  overflow: hidden;
  background: #f2f4f7;
}

/* 整屏组件自己吃掉了默认外边距，这里只需清掉浏览器给 body 的 8px */
:global(body) {
  margin: 0;
}

.studio-opener__canvas {
  position: absolute;
  inset: 0;
}

/* canvas 的 CSS 尺寸在这里统一控制：WebGLRenderer.setSize 传了
   updateStyle=false，不写这两行 canvas 会按物理像素尺寸溢出 */
.studio-opener__canvas :deep(canvas) {
  display: block;
  width: 100%;
  height: 100%;
}

.studio-opener__fallback {
  position: absolute;
  top: 18px;
  left: 50%;
  z-index: 2;
  margin: 0;
  padding: 8px 14px;
  color: #8e2f27;
  border-left: 2px solid var(--opener-danger);
  background: rgb(248 235 228 / 92%);
  font-size: 12px;
  transform: translateX(-50%);
}

/* 跳过动画时的补救入口：右上角一枚轻量胶囊按钮，不抢 3D 画面也不压住面板 */
.studio-opener__replay {
  position: absolute;
  top: 18px;
  right: 20px;
  z-index: 4;
  padding: 9px 16px;
  color: var(--opener-blue);
  border: 1px solid var(--opener-code-border);
  border-radius: 999px;
  background: var(--opener-card-bg);
  box-shadow: var(--opener-card-shadow);
  backdrop-filter: blur(12px);
  cursor: pointer;
  font-family: var(--opener-font);
  font-size: 13px;
  font-weight: 700;
  transition: background 180ms ease, transform 180ms ease;
}

.studio-opener__replay:hover {
  background: #fff;
  transform: translateY(-1px);
}

.studio-opener__replay:active {
  transform: translateY(0);
}

/* 面板外壳：垂直居中刻意用 top/bottom + margin:auto，不用
   transform: translateY(-50%) —— transform 要留给 GSAP 的 xPercent 做滑入，
   两者写在同一个属性上会互相覆盖。 */
.studio-opener__panel {
  position: absolute;
  top: 0;
  bottom: 0;
  left: clamp(20px, 5vw, 88px);
  z-index: 3;
  width: var(--panel-width, 360px);
  height: fit-content;
  max-height: calc(100vh - 40px);
  margin: auto 0;
  overflow-y: auto;
}

/* 窄屏：面板改为近满宽。
   水平居中同样用 left/right + width:auto，绝不用 transform。 */
@media (max-width: 820px) {
  .studio-opener__panel {
    right: 16px;
    left: 16px;
    width: auto;
  }
}
</style>
