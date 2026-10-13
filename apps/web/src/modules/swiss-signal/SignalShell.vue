<script setup lang="ts">
import { nextTick, onUnmounted, provide, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import type { AuthMode } from "@/auth/submit";
import { SIGNAL_BUSY, SIGNAL_MODE, SIGNAL_TRANSITION } from "./context";
import { useSignalMotion } from "./motion";
import "./signal.css";

defineProps<{ transitionOnly?: boolean }>();
const emit = defineEmits<{ "transition-active": [active: boolean] }>();
const route = useRoute();
const router = useRouter();
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
async function openAuth(next: AuthMode) {
  if (busy.value) return;
  if (!["/", "/login"].includes(route.path)) await router.push("/");
  mode.value = next;
}
provide(SIGNAL_TRANSITION, async (change) => {
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
});
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
      <nav class="view-nav" aria-label="页面导航">
        <button
          :aria-pressed="!route.path.startsWith('/user/') && mode === 'login'"
          :disabled="busy"
          @click="openAuth('login')"
        >
          01 登录
        </button>
        <button
          :aria-pressed="
            !route.path.startsWith('/user/') && mode === 'register'
          "
          :disabled="busy"
          @click="openAuth('register')"
        >
          02 注册
        </button>
        <button
          :aria-pressed="route.path.startsWith('/user/')"
          :disabled="busy"
          @click="router.push('/user/home')"
        >
          03 用户首页 <span>↗</span>
        </button>
      </nav>
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
