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
}

export interface UseStudioSceneResult {
  /** 面板是否参与渲染；可见性本身由时间轴的 autoAlpha 决定 */
  panelVisible: Ref<boolean>;
  /** WebGL 初始化失败的降级标记 */
  webglFailed: Ref<boolean>;
}

/** 系统是否开启了「减少动态效果」*/
function prefersReducedMotion(): boolean {
  return (
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

export function useStudioScene({
  canvasHostRef,
  panelRef,
}: UseStudioSceneOptions): UseStudioSceneResult {
  const panelVisible = ref(false);
  const webglFailed = ref(false);

  // 运行时本身不是响应式的：它内部持有 WebGLRenderer / Scene / Timeline，
  // 塞进 ref 会被 Vue 套 Proxy，反而破坏 Three.js 的内部身份比较。
  let runtime: StudioRuntime | null = null;
  let disposed = false;

  function handleResize(): void {
    runtime?.resize();
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
    //    此时面板已经可见且从未被 GSAP 隐藏，直接就是可用的登录页。
    try {
      runtime = createStudioRuntime(host, panelRef.value);
    } catch (error: unknown) {
      runtime?.dispose();
      runtime = null;
      webglFailed.value = true;
      console.warn("[StudioOpener] WebGL 初始化失败，已降级为静态画面：", error);
      return;
    }

    // 3) 布局自检（坐标重叠 / name 重复 / 平视高度不匹配）
    reportSceneIssues();

    window.addEventListener("resize", handleResize);
    runtime.resize();
    runtime.start();

    // 4) 尊重系统设置：不播动画，直接给终态
    if (prefersReducedMotion()) {
      runtime.showFinalState();
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

  return { panelVisible, webglFailed };
}
