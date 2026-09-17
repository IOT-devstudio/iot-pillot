/**
 * 把 3D 运行时接到 Vue 生命周期上。
 *
 * 这里只做四件事，任何 Three.js/GSAP 细节都留在 scene/ 里：
 *   1. 挂载后建运行时、挂 resize 监听、启动渲染循环
 *   2. 控制登录面板的 v-show 时机（面板必须先于时间轴进入 DOM）
 *   3. 处理 WebGL 不可用与 prefers-reduced-motion 两种降级
 *   4. 卸载时摘监听、dispose 全部 GPU 资源
 */
import { nextTick, onMounted, onUnmounted, ref, type Ref } from "vue";

import { CONFIG } from "../config";
import { reportSceneIssues } from "../scene/createBuildings";
import {
  createStudioRuntime,
  type StudioRuntime,
} from "../scene/studioRuntime";

export interface UseStudioSceneOptions {
  /** canvas 宿主元素 */
  canvasHostRef: Ref<HTMLElement | null>;
  /** 登录面板元素（GSAP 的滑入目标）*/
  panelRef: Ref<HTMLElement | null>;
  /**
   * WebGL 初始化失败时的处理，由调用方决定跳哪。
   *
   * 抽成回调而不是在这里直接跳转：这个模块刻意不依赖 vue-router，
   * 只做「Three.js 与 Vue 生命周期」的对接，导航属于调用方的职责。
   *
   * 典型实现是转到二维后备登录页——3D 渲染不出来时，那个面板是唯一
   * 还能用的登录入口。
   */
  onWebglFailed?: () => void;
}

export interface UseStudioSceneResult {
  /** 面板是否参与渲染；可见性本身由时间轴的 autoAlpha 决定 */
  panelVisible: Ref<boolean>;
  /** WebGL 初始化失败的降级标记 */
  webglFailed: Ref<boolean>;
  /**
   * 本次是否跳过了开屏动画（系统减少动态效果 / ?motion=off）。
   * 界面据此给出「播放开屏动画」入口——没有这个入口，
   * 开了该设置的机器上整个开屏动画等于不存在，且无从发现原因。
   */
  animationSkipped: Ref<boolean>;
  /** 手动重播开屏动画（会把场景复位后重新播放）*/
  replayAnimation: () => void;
}

/**
 * 是否跳过整条开屏动画。
 *
 * 默认尊重系统的 prefers-reduced-motion，但**必须**留出覆盖入口：
 * 这个设置一旦为真就会把整个产品卖点（开屏动画）静默抹掉，
 * 不给出路的话，开了「动画效果」的机器上根本无法演示或验收。
 *
 * 优先级：URL ?motion= 显式指定 > CONFIG.respectReducedMotion > 系统设置
 */
export type MotionOverride = "force" | "off" | null;

/** 读取 ?motion=force|off。force = 强制播放，off = 强制静态 */
export function readMotionOverride(): MotionOverride {
  if (typeof window === "undefined" || !window.location) {
    return null;
  }
  const value = new URLSearchParams(window.location.search).get("motion");
  return value === "force" || value === "off" ? value : null;
}

/** 系统是否开启了「减少动态效果」*/
export function prefersReducedMotion(): boolean {
  return (
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/** 跳过动画的原因，用于日志与提示文案 */
export function skipReason(): "url-motion-off" | "reduced-motion" | null {
  const override = readMotionOverride();
  if (override === "off") {
    return "url-motion-off";
  }
  if (override === "force" || !CONFIG.respectReducedMotion) {
    return null;
  }
  return prefersReducedMotion() ? "reduced-motion" : null;
}

export function useStudioScene({
  canvasHostRef,
  panelRef,
  onWebglFailed,
}: UseStudioSceneOptions): UseStudioSceneResult {
  const panelVisible = ref(false);
  const webglFailed = ref(false);
  const animationSkipped = ref(false);

  // 运行时本身不是响应式的：它内部持有 WebGLRenderer / Scene / Timeline，
  // 塞进 ref 会被 Vue 套 Proxy，反而破坏 Three.js 的内部身份比较。
  let runtime: StudioRuntime | null = null;
  let disposed = false;

  function handleResize(): void {
    runtime?.resize();
  }

  /** 手动播放/重播开屏动画 */
  function replayAnimation(): void {
    if (!runtime || disposed) {
      return;
    }
    animationSkipped.value = false;
    runtime.play();
  }

  onMounted(async () => {
    const host = canvasHostRef.value;
    if (!host) {
      return;
    }

    // 1) 面板先进入 DOM。GSAP 的 tween target 必须在建时间轴时就存在，
    //    否则会得到 "GSAP target not found" 的空补间；
    //    「什么时候看得见」随后完全交给时间轴的 autoAlpha。
    panelVisible.value = true;
    await nextTick();

    // onMounted 里 await 过，组件可能已经卸载了，必须防这个竞态
    if (disposed) {
      return;
    }

    // 2) 建场景。WebGL 不可用（老显卡 / 被禁用 / 无头环境）时降级：
    //    先把面板留作可见（它从未被 GSAP 隐藏，直接就是可用的登录页），
    //    再交给调用方决定要不要转到别的登录入口。
    try {
      runtime = createStudioRuntime(host, panelRef.value);
    } catch (error: unknown) {
      runtime?.dispose();
      runtime = null;
      webglFailed.value = true;
      console.warn("[StudioOpener] WebGL 初始化失败，已降级：", error);

      // await 过一次，组件可能已经卸载；此时再导航会指向一个已消失的视图
      if (!disposed) {
        onWebglFailed?.();
      }
      return;
    }

    // 3) 布局自检（坐标重叠 / name 重复 / 平视高度不匹配）
    reportSceneIssues();

    window.addEventListener("resize", handleResize);
    runtime.resize();
    runtime.start();

    // 4) 决定播不播动画。
    //    这里刻意把「为什么跳过」打出来：一旦跳过，整条时间轴一行都不跑，
    //    画面上只看到最终状态，不留日志的话根本无从判断是设置生效还是代码坏了。
    const reason = skipReason();
    if (reason !== null) {
      runtime.showFinalState();
      animationSkipped.value = true;
      console.info(
        "[StudioOpener] 已跳过开屏动画，直接展示终态。" +
          (reason === "url-motion-off"
            ? "原因：URL 上带了 ?motion=off。"
            : "原因：系统/浏览器开启了「减少动态效果」(prefers-reduced-motion: reduce)。" +
              "画面上提供了「播放开屏动画」按钮可手动播放，也可以加 ?motion=force，" +
              "或把 CONFIG.respectReducedMotion 设为 false。"),
      );
      return;
    }

    // 5) 播开屏动画
    runtime.play();
  });

  onUnmounted(() => {
    disposed = true;
    window.removeEventListener("resize", handleResize);
    runtime?.dispose();
    runtime = null;
  });

  return { panelVisible, webglFailed, animationSkipped, replayAnimation };
}
