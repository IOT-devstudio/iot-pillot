/**
 * 场景运行时：持有渲染循环、尺寸响应、时间轴与释放逻辑。
 *
 * 这一层完全不依赖 Vue —— 由 composables/useStudioScene.ts 在组件的
 * onMounted / onUnmounted 里调用它的 start() / dispose()。
 */
import gsap from "gsap";
import * as THREE from "three";

import { CONFIG } from "../config";
import { createBuildings } from "./createBuildings";
import { createStudioScene } from "./createStudioScene";
import { createStudioTimeline } from "./createStudioTimeline";
import type { StudioScene } from "./types";

export interface StudioRuntime {
  /** 播放开屏动画（含面板初始隐藏位置）*/
  play(): void;
  /** 启动渲染循环 */
  start(): void;
  /** 窗口尺寸变化：同步相机 aspect 与渲染器尺寸 */
  resize(): void;
  /** 直接落到动画终态并显示面板（prefers-reduced-motion / WebGL 失败时用）*/
  showFinalState(): void;
  /** 停帧、杀时间轴、把面板恢复为可见、释放 GPU 资源 */
  dispose(): void;
}

/**
 * 建场景 + 建楼栋，返回一个可操控的运行时。
 *
 * @param host    canvas 的宿主元素
 * @param panelEl 登录面板元素（GSAP 的滑入目标）
 * @throws WebGLRenderer 构造失败时抛出，由调用方降级
 */
export function createStudioRuntime(
  host: HTMLElement,
  panelEl: HTMLElement | null,
): StudioRuntime {
  const target: StudioScene = createStudioScene(host);
  createBuildings(target);

  let frameId = 0;
  let timeline: gsap.core.Timeline | null = null;
  let disposed = false;

  // 空闲漂浮：只在时间轴播完后生效，始终以「静止位 + 正弦偏移」重算，
  // 不做增量累加，避免误差累积导致相机漂走
  const restPosition = new THREE.Vector3();
  let idleActive = false;
  let idleStartAt = 0;

  function renderFrame(): void {
    if (disposed) {
      return;
    }
    frameId = requestAnimationFrame(renderFrame);

    if (idleActive) {
      const t = (target.clock.getElapsedTime() - idleStartAt) * CONFIG.idleFloatSpeed;
      const amplitude = CONFIG.idleFloatAmplitude;
      target.camera.position.set(
        restPosition.x + Math.sin(t) * amplitude,
        restPosition.y + Math.sin(t * 0.7) * amplitude * 0.35,
        restPosition.z + Math.cos(t * 0.85) * amplitude,
      );
    }

    target.camera.lookAt(target.lookAt.x, target.lookAt.y, target.lookAt.z);
    target.renderer.render(target.scene, target.camera);
  }

  return {
    play(): void {
      if (panelEl) {
        // 面板初始位置：整个藏到画面左侧之外（等价 transform: translateX(-100%)）
        gsap.set(panelEl, { xPercent: -100, autoAlpha: 0 });
      }

      timeline = createStudioTimeline(target, panelEl, {
        onComplete: () => {
          restPosition.copy(target.camera.position);
          idleStartAt = target.clock.getElapsedTime();
          idleActive = true;
        },
      });
    },

    start(): void {
      renderFrame();
    },

    resize(): void {
      const width = target.host.clientWidth;
      const height = target.host.clientHeight;
      if (width === 0 || height === 0) {
        return;
      }

      target.camera.aspect = width / height;
      target.camera.updateProjectionMatrix();
      target.renderer.setPixelRatio(
        Math.min(window.devicePixelRatio, CONFIG.maxPixelRatio),
      );
      target.renderer.setSize(width, height, false);
    },

    showFinalState(): void {
      target.camera.position.set(
        CONFIG.cameraStudio.x + CONFIG.cameraShiftRight,
        CONFIG.cameraStudio.y,
        CONFIG.cameraStudio.z,
      );
      target.lookAt.x = CONFIG.cameraStudioTarget.x;
      target.lookAt.y = CONFIG.cameraStudioTarget.y;
      target.lookAt.z = CONFIG.cameraStudioTarget.z;

      for (const { mesh } of target.buildings) {
        mesh.scale.y = 1;
      }

      if (panelEl) {
        // 不播动画也要让面板可见。autoAlpha:1 会把 visibility 还原成可见，
        // 不需要额外 clearProps。
        gsap.set(panelEl, { xPercent: 0, autoAlpha: 1 });
      }
    },

    dispose(): void {
      disposed = true;

      if (frameId !== 0) {
        cancelAnimationFrame(frameId);
        frameId = 0;
      }

      // 杀时间轴，避免它对已销毁的 mesh 继续补间
      timeline?.kill();
      timeline = null;
      idleActive = false;

      for (const item of target.disposables) {
        item.dispose();
      }
      target.disposables.length = 0;
      target.buildings.length = 0;

      target.scene.clear();
      target.renderer.dispose();
      target.renderer.forceContextLoss();
      target.renderer.domElement.remove();
    },
  };
}
