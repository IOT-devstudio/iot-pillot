<script setup lang="ts">
import { computed, nextTick, provide, ref } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/common/AdminLayout.vue";
import SignalShell from "@/modules/swiss-signal/SignalShell.vue";
import { SIGNAL_TRANSITION } from "@/modules/swiss-signal/context";
import ProfileCompletionNotice from "@/modules/user-settings/components/ProfileCompletionNotice.vue";
import "@/modules/swiss-signal/signal.css";

const route = useRoute();
const signalTransitioning = ref(false);
const usesAdminLayout = computed(() => route.meta.adminShell === true);
const usesSignal = computed(() =>
  route.meta.signalShell === true ||
  [
    "/",
    "/login",
    "/user/home",
    "/user/settings",
    "/user/about",
    "/user/invitations",
  ].includes(route.path),
);

/**
 * 全局擦除转场（issue #59 后续：退出登录用）。
 *
 * SignalShell 内部的页面跳转仍由它自己 provide 的 withWipe 接管
 * （就近提供者优先，动画带壳层 motion 的暂停/挂起联动）；这里只兜住
 * **壳外**场景——目前唯一入口是管理端 UserMenu 的退出登录：
 * 擦除层从角上扫过覆盖管理台 → 覆盖态下清会话并跳「/」→ 揭开露出登录页。
 *
 * 样式复用 signal.css 的 .swiss-signal.transition-only 覆盖层
 * （本意就是「非信号页上方只渲染擦除层」），动画节奏与 SignalShell
 * 的 withWipe 一致（覆盖 450ms / 揭开 650ms 同一款缓动）。
 * prefers-reduced-motion 时直接跳转，与用户侧口径一致。
 */
const globalWiping = ref(false);
const globalWipeEl = ref<HTMLElement | null>(null);
const WIPE_EASE = "cubic-bezier(.22,1,.36,1)";

function animateWipePanel(
  panel: HTMLElement,
  keyframes: Keyframe[],
  duration: number,
): Promise<void> {
  if (!panel.animate) return Promise.resolve();
  return panel
    .animate(keyframes, { duration, easing: WIPE_EASE, fill: "both" })
    .finished.catch(() => {})
    .then(() => {
      try {
        panel.getAnimations().forEach((animation) => animation.cancel());
      } catch {
        // 动画已自然结束时 cancel 抛错可忽略
      }
    });
}

async function globalWithWipe(change: () => unknown): Promise<void> {
  if (globalWiping.value) return;
  if (
    typeof matchMedia !== "undefined" &&
    matchMedia("(prefers-reduced-motion: reduce)").matches
  ) {
    await change();
    return;
  }
  globalWiping.value = true;
  await nextTick();
  try {
    const panels = [
      ...(globalWipeEl.value?.querySelectorAll<HTMLElement>(".wipe-panel") ??
        []),
    ];
    await Promise.all(
      panels.map((panel, index) => {
        const origin = index === 1 ? "-100%" : "100%";
        panel.style.transform = "translateY(0)";
        return animateWipePanel(
          panel,
          [
            { transform: `translateY(${origin})` },
            { transform: "translateY(0)" },
          ],
          450,
        );
      }),
    );
    await change();
    await nextTick();
    await Promise.all(
      panels.map((panel, index) => {
        const destination = index === 1 ? "-100%" : "100%";
        panel.style.transform = `translateY(${destination})`;
        return animateWipePanel(
          panel,
          [
            { transform: "translateY(0)" },
            { transform: `translateY(${destination})` },
          ],
          650,
        );
      }),
    );
  } finally {
    globalWiping.value = false;
  }
}
provide(SIGNAL_TRANSITION, globalWithWipe);
</script>

<template>
  <el-config-provider>
    <ProfileCompletionNotice v-if="Array.isArray(route.meta.allowRoles)" />
    <AdminLayout v-if="usesAdminLayout">
      <router-view />
    </AdminLayout>
    <SignalShell
      v-if="usesSignal || signalTransitioning"
      :transition-only="!usesSignal"
      @transition-active="signalTransitioning = $event"
      ><router-view v-if="usesSignal"
    /></SignalShell>
    <router-view v-if="!usesAdminLayout && !usesSignal" />
    <!-- 全局擦除覆盖层：壳外转场（退出登录）用；signal.css 的
         .transition-only 只显示擦除层本身，盖在管理台之上 -->
    <div
      v-if="globalWiping"
      class="swiss-signal transition-only"
      aria-hidden="true"
    >
      <div ref="globalWipeEl" class="page-wipe">
        <div class="wipe-panel"></div>
        <div class="wipe-panel">
          <span class="wipe-word">CONNECTING<br />IDEAS<span>↗</span></span>
        </div>
        <div class="wipe-panel"></div>
      </div>
    </div>
  </el-config-provider>
</template>
