/**
 * GSAP Timeline：把 4 个开屏阶段编排成一条时间轴。
 *
 * 时间轴刻度（秒）—— 后续调节奏时对着这三行改即可：
 *
 *   阶段 0  初始状态（页面挂载瞬间，不在时间轴里）
 *           楼栋 scale.y = 0.001 藏在地下；相机在 CONFIG.cameraStart 俯瞰。
 *
 *   阶段 1  PHASE1_START = 0
 *           楼栋错峰升起 + 相机绕 Y 轴缓慢自转。
 *           结束于 PHASE2_START，实际时长 = max(CONFIG.phase1MinDuration,
 *           最后一栋楼的结束时刻)——因为「楼越高时长越长」叠加错峰延迟后
 *           天然会超过名义的 3 秒。
 *
 *   阶段 2  PHASE2_START = max(CONFIG.phase1MinDuration, 升起结束时刻)
 *           相机平滑推到 CONFIG.cameraStudio，注视点同步转到
 *           CONFIG.cameraStudioTarget，落点与工作室楼栋平视。
 *           持续 CONFIG.phase2Duration。
 *
 *   阶段 3  PHASE3_START = PHASE2_START + CONFIG.phase2Duration
 *           相机沿 +X 右移 CONFIG.cameraShiftRight；登录面板从画面左侧滑入。
 *           结束后触发 onComplete，场景转入空闲漂浮。
 *
 * 想让阶段 2 严格从 3.0s 开始：调大 CONFIG.phase1MinDuration，或调小
 * buildingRiseDelay / buildingRiseDuration.max。
 */
import gsap from "gsap";
import * as THREE from "three";

import { CONFIG } from "../config";
import type { StudioScene } from "./types";

export interface TimelineHooks {
  /** 时间轴播放完毕（= 场景进入空闲漂浮）时回调 */
  onComplete?: () => void;
}

/** 每栋楼的升起安排：时长与错峰延迟算完之后才知道阶段 1 何时真正结束 */
interface RisePlan {
  duration: number;
  delay: number;
  /** 该楼栋升起的结束时刻（相对时间轴起点） */
  end: number;
}

/** 把楼高映射到 0~1，用于「楼越高动画越长」 */
function heightRatio(height: number, min: number, max: number): number {
  if (max <= min) {
    return 0;
  }
  return Math.min(1, Math.max(0, (height - min) / (max - min)));
}

/** 先算清每栋楼的 (delay, duration)，才能推出阶段 1 的真实结束时刻 */
function planRises(target: StudioScene): RisePlan[] {
  const heights = target.buildings.map((building) => building.height);
  const minHeight = Math.min(...heights);
  const maxHeight = Math.max(...heights);

  return target.buildings.map(({ index, height }) => {
    const duration = gsap.utils.mapRange(
      0,
      1,
      CONFIG.buildingRiseDuration.min,
      CONFIG.buildingRiseDuration.max,
      heightRatio(height, minHeight, maxHeight),
    );
    const delay =
      index * CONFIG.buildingRiseDelay +
      Math.random() * CONFIG.buildingRiseRandomJitter;

    return { duration, delay, end: delay + duration };
  });
}

/**
 * 构建并**立即开始播放**整条时间轴。
 *
 * @param target  已建好的场景（相机、楼栋、注视点都要已就位）
 * @param panelEl 登录面板元素；GSAP 需要它此刻已在 DOM 中
 */
export function createStudioTimeline(
  target: StudioScene,
  panelEl: HTMLElement | null,
  hooks: TimelineHooks = {},
): gsap.core.Timeline {
  const { camera, lookAt, buildings } = target;
  const risePlan = planRises(target);

  const PHASE1_START = 0;
  const PHASE2_START = Math.max(
    CONFIG.phase1MinDuration,
    ...risePlan.map((plan) => plan.end),
  );
  const PHASE3_START = PHASE2_START + CONFIG.phase2Duration;

  const timeline = gsap.timeline({
    defaults: { ease: CONFIG.buildingRiseEase },
    onComplete: () => hooks.onComplete?.(),
  });

  // ── 阶段 1（PHASE1_START → PHASE2_START）：楼栋错峰升起 ──
  buildings.forEach(({ mesh }, i) => {
    const plan = risePlan[i];
    if (!plan) {
      return;
    }
    timeline.to(
      mesh.scale,
      {
        y: 1,
        duration: plan.duration,
        delay: plan.delay,
        ease: CONFIG.buildingRiseEase,
      },
      PHASE1_START,
    );
  });

  // ── 阶段 1 附加：相机绕 Y 轴缓慢自转（cameraSpinDegrees = 0 则跳过）──
  // 在半径不变的圆周上转一个角度，高度保持 cameraStart.y。补间的是一个
  // [0 → 角度] 的普通对象，真正写相机位置在 onUpdate 里做。
  if (CONFIG.cameraSpinDegrees > 0) {
    const start = CONFIG.cameraStart;
    const radius = Math.hypot(start.x, start.z);
    const baseAngle = Math.atan2(start.x, start.z);
    const spin = { angle: 0 };

    timeline.to(
      spin,
      {
        angle: THREE.MathUtils.degToRad(CONFIG.cameraSpinDegrees),
        duration: PHASE2_START - PHASE1_START,
        ease: "sine.inOut",
        onUpdate: () => {
          const angle = baseAngle + spin.angle;
          camera.position.set(
            Math.sin(angle) * radius,
            start.y,
            Math.cos(angle) * radius,
          );
        },
      },
      PHASE1_START,
    );
  }

  // ── 阶段 2（PHASE2_START → PHASE3_START）：推进到工作室并转为平视 ──
  // 位置与注视点同时补间；注视点 y=10 对应工作室 h=20 的中部 → 平视。
  timeline.to(
    camera.position,
    {
      x: CONFIG.cameraStudio.x,
      y: CONFIG.cameraStudio.y,
      z: CONFIG.cameraStudio.z,
      duration: CONFIG.phase2Duration,
      ease: CONFIG.focusEase,
    },
    PHASE2_START,
  );
  timeline.to(
    lookAt,
    {
      x: CONFIG.cameraStudioTarget.x,
      y: CONFIG.cameraStudioTarget.y,
      z: CONFIG.cameraStudioTarget.z,
      duration: CONFIG.phase2Duration,
      ease: CONFIG.focusEase,
    },
    PHASE2_START,
  );

  // ── 阶段 3（PHASE3_START 起）：相机右移 + 面板滑入 ──
  // X 落点写成「阶段 2 落点 + cameraShiftRight」，而不是相对 "+="，
  // 这样时间轴无论正放、倒放还是被 seek，落点都是确定的。
  timeline.to(
    camera.position,
    {
      x: CONFIG.cameraStudio.x + CONFIG.cameraShiftRight,
      duration: CONFIG.phase3Duration,
      ease: CONFIG.focusEase,
    },
    PHASE3_START,
  );

  // 面板：xPercent -100 = transform: translateX(-100%)，整个面板藏在画面左侧外
  // → xPercent 0；autoAlpha 同步 0 → 1。
  // autoAlpha 只动 opacity + visibility，不碰 display，所以与 v-show 不冲突；
  // 隐藏时 visibility:hidden 天然收不到指针事件，点击不会穿透到 canvas。
  if (panelEl) {
    timeline.to(
      panelEl,
      {
        xPercent: 0,
        autoAlpha: 1,
        duration: CONFIG.panelSlideDuration,
        ease: CONFIG.panelEase,
      },
      PHASE3_START,
    );
  }

  // 把四个阶段的绝对时刻打出来：调节奏时不用猜，也方便确认时间轴真的在跑
  console.info(
    `[StudioOpener] 时间轴已启动：阶段1 ${PHASE1_START.toFixed(2)}s → ${PHASE2_START.toFixed(2)}s（楼栋错峰升起）` +
      ` → 阶段2 → ${PHASE3_START.toFixed(2)}s（聚焦工作室）` +
      ` → 阶段3 → ${timeline.duration().toFixed(2)}s（右移 + 面板滑入）`,
  );

  return timeline;
}
