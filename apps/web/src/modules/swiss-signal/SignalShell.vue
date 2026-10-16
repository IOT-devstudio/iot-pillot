<script setup lang="ts">
import { computed, nextTick, onUnmounted, provide, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import UserMenu from "@/components/common/UserMenu.vue";
import SignInCta from "@/components/common/SignInCta.vue";
import { currentSession, syncAuthSession } from "@/auth/session";
import type { AuthMode } from "@/auth/submit";
import { currentProfile } from "@/modules/user-settings/profileIdentity";
import { SIGNAL_BUSY, SIGNAL_MODE, SIGNAL_TRANSITION } from "./context";
import { useSignalMotion } from "./motion";
import "./signal.css";

defineProps<{ transitionOnly?: boolean }>();
const emit = defineEmits<{ "transition-active": [active: boolean] }>();
const route = useRoute();
const router = useRouter();
syncAuthSession();
/**
 * masthead 的导航（issue #94）：
 *   - 认证页（/、/login）：不放导航——表单自带登录/注册切换，顶栏再来一份
 *     完全重复
 *   - 用户侧（/user/*）：用户名菜单（UserMenu）常驻；「返回」按钮只出现在
 *     非首页的用户页面，目标按角色分流（管理员→控制台，成员→个人首页）；
 *     个人设置页不放用户名菜单——页面本身就是「个人设置」，入口重复（issue #94 review）
 *   - 品牌区（左上角）：静态展示，**不是链接**——它原是指向 "/" 的返回入口，
 *     在首页点它会跳回登录页，与「首页」语义冲突。左侧徽标为工作室圆形图标，
 *     右侧字标保持原样；返回入口只由上面的 view-nav 承担
 */
const isUserPage = () => route.path.startsWith("/user/") || route.meta.signalShell === true;
const isUserHome = () => route.path === "/user/home";
const isUserSettings = () => route.path === "/user/settings";
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

/**
 * 返回按钮按角色分流（#94 原始裁决的回归修复）：
 *   管理员 → 控制台；普通成员 → 个人首页。
 * 角色来自 profileIdentity（UserMenu 挂载时就会加载 /me）；加载完成前
 * 按成员默认展示——/user/home 管理员也进得去，不会把人引进死路。
 */
const backTarget = computed(() =>
  currentProfile.value?.role === "admin" ? "/dashboard" : "/user/home",
);
const backLabel = computed(() =>
  currentProfile.value?.role === "admin" ? "返回控制台" : "返回用户首页",
);

/** 「返回」：与登录转场同款的擦除动画 */
function goBack(): void {
  void withWipe(() => router.push(backTarget.value));
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
    :data-view="isUserPage() ? 'home' : mode"
  >
    <header class="masthead">
      <div class="wordmark">
        <img class="logo" src="/studio-logo.png" alt="" aria-hidden="true" />
        <span>IoT-Studio</span>
      </div>
      <span class="masthead-note">A PLACE FOR CURIOUS MINDS.</span>
      <nav
        v-if="isUserPage() && !isUserHome()"
        class="view-nav"
        aria-label="页面导航"
      >
        <button aria-pressed="true" :disabled="busy" @click="goBack">
          {{ backLabel }} <span>↗</span>
        </button>
      </nav>
      <span v-if="isUserPage() && !isUserSettings()" class="masthead__user">
        <UserMenu v-if="currentSession" />
        <SignInCta v-else />
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
      <!-- 关于我们的入口收敛到首页顶栏（review：页脚重复就删） -->
      <span class="mono">IoT-Studio / SWISS SIGNAL / 2026</span
      ><span>保持好奇，把想法做成作品。</span
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
