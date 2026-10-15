/**
 * 液态玻璃流光的鼠标跟随（issue #85 玻璃系统增强，review 反馈）。
 *
 * 行为（review 裁决）：
 *   - 悬停玻璃卡时，两束高光实时指向鼠标所在方向；
 *     离开卡片后恢复 conic-gradient 的自动循环
 *   - 作用于所有 .liquid-glass 卡（事件委托，一处安装全局生效）
 *   - 触屏/无精确指针设备保持自动循环，不跟随
 *   - prefers-reduced-motion 不跟随（停在静态高光）
 *
 * 实现要点：
 *   - CSS 动画在级联上压过 inline 自定义属性，所以跟随时必须给卡片挂
 *     .glass-beam-follow（animation: none），inline --beam-angle 才能生效；
 *     离开时摘类，动画从头自动循环
 *   - mousemove 全局监听但只在有悬停卡时 rAF 节流计算，避免空转
 */

export function installGlassBeam(): void {
  if (typeof document === "undefined") return;

  const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)");
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

  let active: HTMLElement | null = null;
  let frame = 0;
  let pointerX = 0;
  let pointerY = 0;

  /** 渐变里白光亮斑位于 40deg：把 40deg 转到鼠标方向，光就指向鼠标 */
  function applyBeamAngle(): void {
    frame = 0;
    if (active === null) return;
    const rect = active.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    const mouseAngle =
      (Math.atan2(pointerY - centerY, pointerX - centerX) * 180) / Math.PI;
    active.style.setProperty("--beam-angle", `${mouseAngle - 40}deg`);
  }

  function scheduleApply(): void {
    if (active !== null && frame === 0) {
      frame = requestAnimationFrame(applyBeamAngle);
    }
  }

  document.addEventListener(
    "mousemove",
    (event) => {
      if (!finePointer.matches || reducedMotion.matches) return;
      pointerX = event.clientX;
      pointerY = event.clientY;
      scheduleApply();
    },
    { passive: true },
  );

  document.addEventListener(
    "mouseover",
    (event) => {
      if (!finePointer.matches || reducedMotion.matches) return;
      const card = (event.target as Element | null)?.closest?.<HTMLElement>(
        ".liquid-glass",
      );
      if (card === null || card === undefined || card === active) return;
      active?.classList.remove("glass-beam-follow");
      active = card;
      card.classList.add("glass-beam-follow");
      pointerX = event.clientX;
      pointerY = event.clientY;
      scheduleApply();
    },
  );

  document.addEventListener(
    "mouseout",
    (event) => {
      if (active === null) return;
      const related = event.relatedTarget as Node | null;
      // 在卡片内部子元素间移动不释放
      if (related !== null && active.contains(related)) return;
      active.classList.remove("glass-beam-follow");
      active.style.removeProperty("--beam-angle");
      active = null;
    },
  );
}
