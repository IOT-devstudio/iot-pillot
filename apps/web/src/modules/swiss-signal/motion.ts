import { nextTick, onMounted, onUnmounted, ref, type Ref } from "vue";

/** Each shell owns its animations, timers, observers and listeners. */
export function useSignalMotion(root: Ref<HTMLElement | null>) {
  const paused = ref(false);
  const suspended = ref(false);
  const reduced = ref(false);
  const active = new Map<Animation, ReturnType<typeof setTimeout>>();
  const frames = new Set<number>();
  const cleanups: Array<() => void> = [];
  let observer: IntersectionObserver | undefined;
  let disposed = false;
  let scrollFrame = 0;
  let pointerFrame = 0;
  let x = 0,
    y = 0,
    sx = 0,
    sy = 0;
  const enabled = () =>
    !disposed && !paused.value && !suspended.value && !reduced.value;
  const ease = "cubic-bezier(.22,1,.36,1)";

  function frame(callback: (time: number) => void) {
    const id = requestAnimationFrame((time) => {
      frames.delete(id);
      if (!disposed) callback(time);
    });
    frames.add(id);
    return id;
  }
  function animate(
    element: Element | null,
    keyframes: Keyframe[],
    options: KeyframeAnimationOptions = {},
  ) {
    if (!element || !enabled() || !element.animate) return Promise.resolve();
    const animation = element.animate(keyframes, {
      duration: 650,
      easing: ease,
      fill: "both",
      ...options,
    });
    const timer = setTimeout(
      () => {
        try {
          animation.finish();
        } catch {
          animation.cancel();
        }
      },
      Number(options.duration ?? 650) + Number(options.delay ?? 0) + 350,
    );
    active.set(animation, timer);
    return animation.finished
      .catch(() => {})
      .then(() => {
        clearTimeout(timer);
        active.delete(animation);
        animation.cancel();
      });
  }
  function settle(preserveWipe = false) {
    active.forEach((timer, animation) => {
      const target = (animation.effect as KeyframeEffect | null)?.target;
      if (
        preserveWipe &&
        target instanceof Element &&
        target.closest(".page-wipe")
      )
        return;
      clearTimeout(timer);
      try {
        animation.finish();
      } catch {
        animation.cancel();
      }
    });
    frames.forEach(cancelAnimationFrame);
    frames.clear();
    scrollFrame = pointerFrame = 0;
    x = y = sx = sy = 0;
    root.value?.style.setProperty("--drift-x", "0px");
    root.value?.style.setProperty("--drift-y", "0px");
    root.value
      ?.querySelectorAll<HTMLElement>(
        "[data-magnetic], .direction-panel, .home-poster",
      )
      .forEach((el) => {
        el.style.removeProperty("translate");
        for (const name of [
          "--chapter-drift",
          "--symbol-drift",
          "--scroll-drift",
        ])
          el.style.removeProperty(name);
      });
  }
  function stagger(elements: Iterable<Element>, distance = 24, delay = 0) {
    [...elements].forEach(
      (el, i) =>
        void animate(
          el,
          [
            { opacity: 0, transform: `translateY(${distance}px)` },
            { opacity: 1, transform: "translateY(0)" },
          ],
          { delay: delay + i * 45 },
        ),
    );
  }
  async function enter() {
    await nextTick();
    if (!root.value || disposed) return;
    // The shell persists across these routes: settle the previous page's work,
    // while the authentication wipe remains owned by the awaiting transition.
    settle(true);
    stagger(root.value.querySelectorAll(".title-line"), 100, 55);
    stagger(
      root.value.querySelectorAll(
        ".poster-top, .poster-copy, .poster-foot, .home-poster-top, .home-poster-bottom",
      ),
      18,
      160,
    );
    root.value.querySelectorAll(".signal-mark").forEach(
      (el) =>
        void animate(
          el,
          [
            { opacity: 0, scale: ".6" },
            { opacity: getComputedStyle(el).opacity, scale: "1" },
          ],
          { duration: 1100, delay: 90 },
        ),
    );
    form();
    observer?.disconnect();
    root.value
      .querySelectorAll(
        ".direction-section, .journey-sheet, .closing-note, .space-stats",
      )
      .forEach((el) => observer?.observe(el));
  }
  function form() {
    active.forEach((_timer, animation) => {
      const target = (animation.effect as KeyframeEffect | null)?.target;
      if (target instanceof Element && target.closest(".auth-pane")) {
        try {
          animation.finish();
        } catch {
          animation.cancel();
        }
      }
    });
    const pane = root.value?.querySelector('.auth-pane[aria-hidden="false"]');
    if (pane)
      stagger(
        pane.querySelectorAll("h2, .form-intro, .field-group, .action-button"),
        18,
        100,
      );
  }
  function scroll() {
    if (!enabled() || scrollFrame) return;
    scrollFrame = frame(() => {
      scrollFrame = 0;
      const hero = root.value?.querySelector<HTMLElement>(".home-poster");
      if (hero)
        hero.style.setProperty(
          "--scroll-drift",
          `${Math.max(-25, Math.min(35, -hero.getBoundingClientRect().top * 0.09))}px`,
        );
      const rail = root.value?.querySelector<HTMLElement>(".direction-rail");
      if (!rail || !rail.clientWidth) return;
      const rect = rail.getBoundingClientRect();
      if (rect.bottom < 0 || rect.top > innerHeight) return;
      rail
        .querySelectorAll<HTMLElement>(".direction-panel")
        .forEach((panel) => {
          const r = panel.getBoundingClientRect();
          const delta =
            (r.left + r.width / 2 - rect.left - rail.clientWidth / 2) /
            rail.clientWidth;
          panel.style.setProperty(
            "--chapter-drift",
            `${Math.max(-30, Math.min(30, -delta * 32))}px`,
          );
          panel.style.setProperty(
            "--symbol-drift",
            `${Math.max(-16, Math.min(16, delta * 22))}px`,
          );
        });
    });
  }
  function follow() {
    if (!enabled()) return;
    sx += (x - sx) * 0.075;
    sy += (y - sy) * 0.075;
    root.value?.style.setProperty("--drift-x", `${sx.toFixed(2)}px`);
    root.value?.style.setProperty("--drift-y", `${sy.toFixed(2)}px`);
    pointerFrame =
      Math.abs(x - sx) + Math.abs(y - sy) > 0.05 ? frame(follow) : 0;
  }
  function listen(
    target: EventTarget,
    name: string,
    callback: EventListener,
    options?: AddEventListenerOptions,
  ) {
    target.addEventListener(name, callback, options);
    cleanups.push(() => target.removeEventListener(name, callback, options));
  }
  function sync() {
    if (!enabled()) settle();
    else scroll();
  }
  function toggle() {
    paused.value = !paused.value;
    sync();
  }
  onMounted(() => {
    if (!root.value?.querySelectorAll || typeof matchMedia === "undefined")
      return;
    const media = matchMedia("(prefers-reduced-motion: reduce)");
    const fine = matchMedia("(hover: hover) and (pointer: fine)");
    reduced.value = media.matches;
    suspended.value = document.hidden;
    listen(media, "change", () => {
      reduced.value = media.matches;
      sync();
    });
    listen(document, "visibilitychange", () => {
      suspended.value = document.hidden;
      sync();
    });
    listen(window, "resize", scroll);
    listen(document, "scroll", scroll, { passive: true, capture: true });
    listen(
      root.value,
      "pointermove",
      (event) => {
        const e = event as PointerEvent;
        if (!enabled() || !fine.matches || e.pointerType === "touch") return;
        x = (e.clientX / innerWidth - 0.5) * 22;
        y = (e.clientY / innerHeight - 0.5) * 14;
        if (!pointerFrame) pointerFrame = frame(follow);
        const button = (e.target as Element).closest<HTMLButtonElement>(
          ".action-button, .panel-button, .rail-controls button",
        );
        if (button && !button.disabled) {
          const r = button.getBoundingClientRect();
          button.dataset.magnetic = "true";
          button.style.translate = `${Math.max(-5, Math.min(5, (e.clientX - r.left - r.width / 2) * 0.07))}px ${Math.max(-4, Math.min(4, (e.clientY - r.top - r.height / 2) * 0.1))}px`;
        }
      },
      { passive: true },
    );
    listen(root.value, "pointerout", (e) => {
      const el = (e.target as Element).closest<HTMLElement>("[data-magnetic]");
      if (el && !el.contains((e as PointerEvent).relatedTarget as Node | null))
        el.style.removeProperty("translate");
    });
    listen(root.value, "click", (e) => {
      const button = (e.target as Element).closest<HTMLButtonElement>(
        ".action-button, .panel-button, .rail-controls button",
      );
      if (button && !button.disabled)
        void animate(
          button,
          [{ scale: "1" }, { scale: ".975", offset: 0.35 }, { scale: "1" }],
          { duration: 300 },
        );
    });
    observer = new IntersectionObserver(
      (entries) =>
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          observer?.unobserve(entry.target);
          stagger([entry.target], 28);
          stagger(entry.target.querySelectorAll(".journey-steps li"), 18, 120);
          // 统计数字由 Vue 渲染；异步数据更新时不能用动画覆盖文本。
        }),
      { threshold: 0.15 },
    );
    void enter();
  });
  onUnmounted(() => {
    disposed = true;
    settle();
    observer?.disconnect();
    cleanups.forEach((fn) => fn());
  });
  return {
    paused,
    suspended,
    reduced,
    animate,
    stagger,
    enter,
    form,
    toggle,
    enabled,
    settle,
  };
}
