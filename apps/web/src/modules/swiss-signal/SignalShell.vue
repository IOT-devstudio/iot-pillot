<script setup lang="ts">
import { nextTick, onUnmounted, provide, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import UserMenu from "@/components/common/UserMenu.vue";
import type { AuthMode } from "@/auth/submit";
import { SIGNAL_BUSY, SIGNAL_MODE, SIGNAL_TRANSITION } from "./context";
import { useSignalMotion } from "./motion";
import "./signal.css";

defineProps<{ transitionOnly?: boolean }>();
const emit = defineEmits<{ "transition-active": [active: boolean] }>();
const route = useRoute();
const router = useRouter();
/**
 * masthead 的导航（issue #94）：
 *   - 认证页（/、/login）：不放导航——表单自带登录/注册切换，顶栏再来一份
 *     完全重复
 *   - 用户侧（/user/*）：用户名菜单（UserMenu）常驻；「返回用户首页」只出现
 *     在非首页的用户页面（首页本身就是落点，再放一个自己链自己没有意义）
 */
const isUserPage = () => route.path.startsWith("/user/");
const isUserHome = () => route.path === "/user/home";
const root = ref<HTMLElement | null>(null);
const wipe = ref<HTMLElement | null>(null);
const wiping = ref(false);
const mode = ref<AuthMode>("login");
const busy = ref(false);
const motion = useSignalMotion(root);
const { paused, suspended, reduced } = motion;
let disposed = false;
onUnmounted(() => {
  disposed = true;
});
provide(SIGNAL_MODE, mode);
provide(SIGNAL_BUSY, busy);
/**
 * 页面切换的三板擦除过渡（黄块带着 CONNECTING IDEAS 划过）。
 * 认证页的登录/注册切换、用户侧「返回用户首页」与进入个人设置都走它——
 * change 只管真正执行路由跳转，动画的启停、中断与 reduced-motion 降级
 * 全部收敛在这里。
 */
async function withWipe(change: () => unknown) {
  if (wiping.value || disposed) return;
  if (!motion.enabled()) {
    await change();
    return;
  }
  wiping.value = true;
  emit("transition-active", true);
  await nextTick();
  try {
    const panels = [...(wipe.value?.querySelectorAll<HTMLElement>(".wipe-panel") ?? [])];
    // Persist each endpoint before animate() cancels its fill, including
    // when motion is interrupted. The label travels inside the yellow panel.
    await Promise.all(panels.map((panel, index) => {
      const origin = index === 1 ? "-100%" : "100%";
      panel.style.transform = "translateY(0)";
      return motion.animate(
        panel,
        [{ transform: `translateY(${origin})` }, { transform: "translateY(0)" }],
        { duration: 450 },
      );
    }));
    if (disposed) return;
    await change();
    await nextTick();
    if (disposed) return;
    await Promise.all(panels.map((panel, index) => {
      const destination = index === 1 ? "-100%" : "100%";
      panel.style.transform = `translateY(${destination})`;
      return motion.animate(
        panel,
        [{ transform: "translateY(0)" }, { transform: `translateY(${destination})` }],
        { duration: 650 },
      );
    }));
  } finally {
    wiping.value = false;
    emit("transition-active", false);
  }
}
provide(SIGNAL_TRANSITION, withWipe);

/** 「返回用户首页」：与登录转场同款的擦除动画 */
function goHome(): void {
  void withWipe(() => router.push("/user/home"));
}

watch(mode, async () => {
  await nextTick();
  motion.form();
});
watch(
  () => route.path,
  () => {
    mode.value = "login";
    void motion.enter();
  },
);
</script>

<template>
  <div
    ref="root"
    class="swiss-signal"
    :class="{
      'motion-paused': paused,
      'motion-suspended': suspended || reduced,
      'transition-only': transitionOnly,
    }"
    :data-view="route.path.startsWith('/user/') ? 'home' : mode"
  >
    <header class="masthead">
      <RouterLink class="wordmark" to="/" aria-label="IoT Studio 首页"
        ><span class="logo">⌁</span
        ><span>IoT STUDIO<small>物联网工作室</small></span></RouterLink
      >
      <span class="masthead-note">A PLACE FOR CURIOUS MINDS.</span>
      <nav
        v-if="isUserPage() && !isUserHome()"
        class="view-nav"
        aria-label="页面导航"
      >
        <button aria-pressed="true" :disabled="busy" @click="goHome">
          返回用户首页 <span>↗</span>
        </button>
      </nav>
      <span v-if="isUserPage()" class="masthead__user">
        <UserMenu />
      </span>
    </header>
    <slot />
    <div class="signal-tape" aria-hidden="true">
      <div class="tape-track">
        <span v-for="n in 4" :key="n"
          >IDEA → SYSTEM → IMPACT <i>✳</i> STAY CURIOUS. KEEP BUILDING.
          <i>✳</i></span
        >
      </div>
    </div>
    <footer class="footer">
      <span class="mono">IoT STUDIO / SWISS SIGNAL / 2026</span
      ><RouterLink
        v-if="route.path.startsWith('/user/')"
        class="plain-link"
        to="/user/about"
        >关于我们 ↗</RouterLink
      ><span v-else>保持好奇，把想法做成作品。</span
      ><button class="plain-link" :aria-pressed="paused" @click="motion.toggle">
        {{ paused ? "恢复动效 ▶" : "暂停动效 ⏸" }}
      </button>
    </footer>
    <div v-if="wiping" ref="wipe" class="page-wipe" aria-hidden="true">
      <div class="wipe-panel"></div>
      <div class="wipe-panel">
        <span class="wipe-word">CONNECTING<br />IDEAS<span>↗</span></span>
      </div>
      <div class="wipe-panel"></div>
    </div>
  </div>
</template>
