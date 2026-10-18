<script setup lang="ts">
/**
 * 用户侧个人首页 `/user/home`（issue #89：接入真实招新数据）。
 *
 * 方向列表公开；个人资料与报名进度仅在有本地会话时读取（issue #156）。
 * 公开页上会话失效清空个人数据并降为游客，继续浏览方向与介绍：
 *   - GET  /api/v1/recruitment/directions        方向与开放状态
 *   - POST /api/v1/recruitment/applications      报名（归属由令牌决定）
 *   - GET  /api/v1/recruitment/applications      本人申请（进度/统计）
 *   - GET  /api/v1/recruitment/applications/:id  详情（时间线）
 * 卡片标语等纯视觉元数据在 directionPresentation.ts，不属于接口契约。
 *
 * 三块数据彼此独立加载：方向挂了不该拖垮进度区，反之亦然。
 */
import { computed, inject, nextTick, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { AuthRequestError } from "@/api/auth";
import { clearAuthSession, currentSession, syncAuthSession } from "@/auth/session";
import { useSignInGate } from "@/auth/useSignInGate";
import { requestForPublicPage } from "@/api/public-page-request";
import SignInPrompt from "@/components/common/SignInPrompt.vue";
import { fetchMyProfile } from "@/api/profile";
import { FOUNDING_SETTINGS_BLOCKED } from "@/router/guards";
import {
  beginProfileLoad,
  currentProfile as user,
  profileDisplayName,
} from "@/modules/user-settings/profileIdentity";
import {
  PROFILE_SOURCE_APPLY,
  PROFILE_SOURCE_PARAM,
} from "@/modules/user-settings/profileCompletion";
import { profileEntryState } from "../profileEntry";
import { SIGNAL_TRANSITION } from "@/modules/swiss-signal/context";
import {
  fetchDirections,
  fetchMyApplication,
  fetchMyApplications,
  submitApplication,
  type ApplicationStatus,
  type RecruitmentApplication,
  type RecruitmentDirection,
  type RecruitmentTimelineEntry,
} from "@/api/recruitment";
import SignalDialog from "@/modules/swiss-signal/SignalDialog.vue";
import {
  interviewBlockReason as interviewBlockReasonOf,
  presentationOf,
} from "../directionPresentation";
import {
  directionsSettled,
  documentOffsetTop,
  waitUntil,
} from "../hashScroll";
import { formatISODate } from "../format";

/** 申请状态的中文标签（与后端状态机一致） */
const STATUS_LABELS: Record<ApplicationStatus, string> = {
  applied: "待审核",
  reviewing: "审核中",
  accepted: "已通过",
  rejected: "未通过",
};

const router = useRouter();
const route = useRoute();
const { signedIn, action: signInAction, requireSignIn, dismiss, signIn } = useSignInGate();
/**
 * 去关于我们：走壳层的擦除过渡（与「返回用户首页」「进入个人设置」
 * 同款动画），裸 RouterLink 会没有转场直接硬切。
 */
const transition = inject(SIGNAL_TRANSITION, async (change: () => unknown) => {
  await change();
});

function goAbout(event: MouseEvent): void {
  event.preventDefault();
  void transition(() => router.push("/user/about"));
}

/** 首页所有跨页跳转统一走擦除过渡（review：硬切没质感） */
function goSettings(event: MouseEvent): void {
  event.preventDefault();
  if (!requireSignIn("查看个人设置")) return;
  selectedDirection.value = null;
  void transition(() => router.push("/user/settings"));
}

/**
 * 资料未完整时的报名拦截（issue #152）。
 *
 * 关闭弹窗、带来源标记跳个人设置页，由设置页说明缺了哪几项并直接进编辑态。
 * 拦截发生在写请求之前——不提交申请，也不留半条记录。
 */
function goCompleteProfile(): void {
  selectedDirection.value = null;
  void transition(() =>
    router.push({
      path: "/user/settings",
      query: { [PROFILE_SOURCE_PARAM]: PROFILE_SOURCE_APPLY },
    }),
  );
}

function goJourney(event: MouseEvent): void {
  if (!requireSignIn("查看我的进度")) { event.preventDefault(); return; }
  scrollToSection(event, "journey");
}

function goInvitations(event: MouseEvent): void {
  event.preventDefault();
  if (!requireSignIn("查看邀请与结果")) return;
  void transition(() => router.push("/user/invitations"));
}

const userError = ref("");
const loading = ref(true);

const directions = ref<RecruitmentDirection[]>([]);
const directionsError = ref("");
const directionsLoading = ref(true);

const applications = ref<RecruitmentApplication[]>([]);
const applicationsError = ref("");
const applicationsLoading = ref(true);

const rail = ref<HTMLElement | null>(null);
const railIndex = ref(0);

/** 报名弹窗状态 */
const selectedDirection = ref<RecruitmentDirection | null>(null);
const submitting = ref(false);
const submitError = ref("");
const submitNotice = ref("");

/** 进度区：当前展示第几条申请 + 全部申请弹窗 */
const selectedRecordIndex = ref(0);
const showApplications = ref(false);
const activeApplication = computed(
  () => applications.value[selectedRecordIndex.value] ?? null,
);

/** 详情弹窗（时间线按需拉取） */
const selectedApplication = ref<RecruitmentApplication | null>(null);
const detailTimeline = ref<RecruitmentTimelineEntry[]>([]);
const detailLoading = ref(false);
const detailError = ref("");

const displayName = computed(() => profileDisplayName(user.value));

/**
 * 资料入口状态（issue #152）：弹窗、统计区、收尾区三处共用。
 *
 * 判定读共享的 `currentProfile`（`user`），不另存快照——设置页保存成功后
 * `hydrate` 已把新资料发布进来，返回首页就是新的；`/me` 只负责刷新。
 * 创始管理员（`hidden`）仍然整个隐藏入口，理由同 issue #138：
 * 它没有个人设置页，点进去也会被守卫弹回。
 */
const profileEntry = computed(() => profileEntryState(user.value));
const showProfileEntry = computed(() => profileEntry.value.state !== "hidden");
const openDirectionCount = computed(
  () => directions.value.filter((direction) => direction.open).length,
);
const appliedCodes = computed(
  () =>
    new Set(
      applications.value.map((application) => application.direction_code),
    ),
);

/**
 * 选中方向的本人申请（issue #136）：弹窗按它的状态决定两条路径的置灰。
 * 同方向多条申请取最新一条（服务端已按 id 倒序）。
 */
const selectedApplicationRecord = computed(() => {
  const direction = selectedDirection.value;
  if (direction === null) return null;
  return (
    applications.value.find(
      (application) => application.direction_code === direction.code,
    ) ?? null
  );
});

/**
 * 面试路径的置灰原因（issue #136 §5 边界表）；null = 可点。
 * 判定逻辑抽在 directionPresentation.interviewBlockReason（纯函数、有单测）。
 */
const interviewBlockReason = computed(() =>
  interviewBlockReasonOf(
    selectedDirection.value,
    selectedApplicationRecord.value,
  ),
);

function describeError(error: unknown, fallback: string): string {
  return error instanceof AuthRequestError ? error.message : fallback;
}

async function loadProfile(): Promise<void> {
  const session = currentSession.value;
  if (!session) return;
  const publish = beginProfileLoad();
  loading.value = true;
  userError.value = "";
  try {
    const profile = await fetchMyProfile(requestForPublicPage);
    if (currentSession.value === session) publish(profile);
  } catch (error) {
    if (currentSession.value !== session) return;
    publish(null);
    userError.value = describeError(error, "暂时无法加载用户信息，请稍后重试");
  } finally {
    loading.value = false;
  }
}

async function loadDirections(): Promise<void> {
  const session = currentSession.value;
  directionsLoading.value = true;
  directionsError.value = "";
  try {
    directions.value = (await fetchDirections()).items;
  } catch (error) {
    syncAuthSession();
    if (session && currentSession.value === session && error instanceof AuthRequestError && [401, 403].includes(error.status)) {
      clearAuthSession();
      // 去掉失效会话后重读一次公开方向；不循环重试真正的服务异常。
      await loadDirections();
      return;
    }
    directions.value = [];
    directionsError.value = describeError(
      error,
      "暂时无法加载方向，请稍后重试",
    );
  } finally {
    directionsLoading.value = false;
  }
}

async function loadApplications(): Promise<void> {
  const session = currentSession.value;
  if (!session) return;
  applicationsLoading.value = true;
  applicationsError.value = "";
  try {
    const result = await fetchMyApplications(requestForPublicPage);
    if (currentSession.value !== session) return;
    applications.value = result.items;
    if (selectedRecordIndex.value >= applications.value.length) {
      selectedRecordIndex.value = 0;
    }
  } catch (error) {
    if (currentSession.value !== session) return;
    applications.value = [];
    applicationsError.value = describeError(
      error,
      "暂时无法加载报名记录，请稍后重试",
    );
  } finally {
    applicationsLoading.value = false;
  }
}

watch(currentSession, (session) => {
  userError.value = "";
  applicationsError.value = "";
  applications.value = [];
  selectedRecordIndex.value = 0;
  selectedDirection.value = null;
  selectedApplication.value = null;
  showApplications.value = false;
  submitError.value = "";
  submitNotice.value = "";
  if (session) {
    void loadProfile();
    void loadApplications();
  } else {
    loading.value = false;
    applicationsLoading.value = false;
  }
}, { immediate: true });

onMounted(() => {
  void loadDirections();
  void scrollToHashSection();
  notifyIfRedirectedFromSettings();
});

/**
 * 同页 hash 变化也要重新定位（issue #150）：组件已挂载时再 push 到
 * /user/home#directions 不会触发 onMounted，只靠挂载那次会漏掉。
 * 清空 hash（如创始管理员提示后 replace）不触发滚动。
 */
watch(
  () => route.hash,
  (hash) => {
    if (hash) void scrollToHashSection();
  },
);

/**
 * 创始管理员被守卫从 /user/settings 弹回时的解释（issue #138）。
 *
 * 守卫重定向带 ?notice=founding-settings；这里出一条提示说明原因，
 * 而不是让人以为是自己点错了。提示后清掉 query，避免刷新后重复弹。
 */
function notifyIfRedirectedFromSettings(): void {
  if (route.query.notice !== FOUNDING_SETTINGS_BLOCKED) return;
  ElMessage.info(
    "创始管理员账号由系统配置直接种入，不提供个人设置页面（无需完善资料，也不走自助改密）。",
  );
  void router.replace({ query: {} });
}

/** 新的定位请求会作废还在等待中的旧请求（重复点击/连续跳转不打架）。 */
let hashScrollRun = 0;

/**
 * 外链 hash 滚动（个人设置「去选择方向」→ /user/home#directions，issue #150）。
 *
 * Vue Router 不会为 SPA 内跳转自动滚到锚点；且方向/进度区的骨架在接口
 * 数据回来前就有高度，先滚再撑高会把版块顶下去（review 实测差了 200px）。
 * 所以：
 *   1. 等三块数据请求结束（骨架 → 内容会改变版块高度）；
 *   2. 方向区再等卡片真正渲染进 DOM（接口结束 ≠ 已渲染，早滚会落在空白）；
 *   3. 显式滚文档容器到版块顶部。方向 rail 只是内部横向滚动容器，
 *      桌面和手机共用同一条纵向页面滚动，不交给 scrollIntoView 猜祖先。
 * 落点用 offsetTop 链计算，避开入场动画未结束时的 transform 位移，
 * 否则停稳后会越过版块顶部十来像素。
 * 系统开了「减少动态效果」时退回瞬时跳转，与 scrollToSection 口径一致。
 */
async function scrollToHashSection(): Promise<void> {
  const id = route.hash.replace(/^#/, "");
  if (!id) return;
  const run = ++hashScrollRun;
  await nextTick();
  await waitUntil(
    () => !directionsLoading.value && !applicationsLoading.value,
    { timeoutMs: 3000, intervalMs: 50 },
  );
  if (id === "directions") {
    await waitUntil(
      () =>
        directionsSettled({
          loading: directionsLoading.value,
          error: directionsError.value,
          count: directions.value.length,
          rendered: document.querySelectorAll(".direction-panel").length,
        }),
      { timeoutMs: 1500, intervalMs: 50 },
    );
  }
  if (run !== hashScrollRun) return;
  const target = document.getElementById(id);
  if (!target) return;
  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
  window.scrollTo({
    top: Math.max(0, documentOffsetTop(target)),
    behavior: reduced ? "auto" : "smooth",
  });
}

/**
 * 页内锚点的平滑滚动（issue #94）。
 *
 * 不用原生锚点跳转：`scroll-behavior: smooth` 挂在 .swiss-signal 上，而实际
 * 滚动容器是视口，样式管不到；这里直接 scrollIntoView。系统开了
 * 「减少动态效果」时退回瞬时跳转，和壳层动画的降级口径一致。
 */
function scrollToSection(event: MouseEvent, id: string): void {
  event.preventDefault();
  const target = document.getElementById(id);
  if (!target) return;
  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
  target.scrollIntoView({
    behavior: reduced ? "auto" : "smooth",
    block: "start",
  });
  // 同步地址栏锚点，保持链接可分享
  history.replaceState(null, "", `#${id}`);
}

function updateIndex() {
  if (!rail.value) return;
  const panels = [
    ...rail.value.querySelectorAll<HTMLElement>(".direction-panel"),
  ];
  const left = rail.value.getBoundingClientRect().left;
  railIndex.value = panels.reduce(
    (best, panel, index) =>
      Math.abs(panel.getBoundingClientRect().left - left) <
      Math.abs(panels[best]!.getBoundingClientRect().left - left)
        ? index
        : best,
    0,
  );
}

function move(index: number) {
  if (!rail.value) return;
  const target = Math.max(0, Math.min(directions.value.length - 1, index));
  const panel = rail.value.children[target] as HTMLElement | undefined;
  if (!panel) return;
  const immediate =
    matchMedia("(prefers-reduced-motion: reduce)").matches ||
    !!rail.value.closest(".motion-paused");
  rail.value.scrollTo({
    left: panel.offsetLeft,
    behavior: immediate ? "instant" : "smooth",
  });
  railIndex.value = target;
}

function railKey(event: KeyboardEvent) {
  if (event.target !== rail.value) return;
  const target =
    event.key === "Home"
      ? 0
      : event.key === "End"
        ? directions.value.length - 1
        : event.key === "ArrowRight"
          ? railIndex.value + 1
          : event.key === "ArrowLeft"
            ? railIndex.value - 1
            : null;
  if (target !== null) {
    event.preventDefault();
    move(target);
  }
}

/** 打开报名弹窗：清上一次的提交痕迹 */
function openDirection(direction: RecruitmentDirection): void {
  if (!requireSignIn("了解并报名")) return;
  selectedDirection.value = direction;
  submitError.value = "";
  submitNotice.value = "";
}

/**
 * 面试路径（issue #136）：跳转该方向绑定的面试表单填写页。
 * 沿用壳层擦除过渡（与首页其他跨页跳转一致）；提交成功后由填写页
 * 引导回 /user/home，不在首页内嵌表单。
 */
function handleInterview(): void {
  if (!requireSignIn("填写面试表单")) return;
  // 面试表单提交与直接报名产生同一条申请、同一份资料快照（issue #152），
  // 只拦「报名」会留下旁路，所以两个入口共用同一道门槛。
  if (profileEntry.value.blocksApplication) {
    goCompleteProfile();
    return;
  }
  const form = selectedDirection.value?.form ?? null;
  if (form === null || interviewBlockReason.value !== null) return;
  void transition(() => router.push(`/f/${form.share_token}`));
}

/**
 * 真实报名（issue #89）。成功后刷新本人申请——记录以服务端为准，
 * 不在前端乐观插入；失败原地提示，可安全重试（后端唯一约束兜底重复）。
 */
async function handleApply(): Promise<void> {
  if (!requireSignIn("提交报名申请")) return;
  const session = currentSession.value;
  const direction = selectedDirection.value;
  if (direction === null || submitting.value) return;
  // 资料不完整先去补全（issue #152）：报名快照会带上班级/学号，空着提交
  // 只会把问题留给审核端。
  if (profileEntry.value.blocksApplication) {
    goCompleteProfile();
    return;
  }
  submitError.value = "";
  submitNotice.value = "";
  submitting.value = true;
  try {
    await submitApplication(direction.code, requestForPublicPage, session?.user_id);
    if (currentSession.value !== session) return;
    submitNotice.value = "报名成功，可在「我的进度」查看审核进展。";
    await loadApplications();
  } catch (error) {
    if (currentSession.value !== session) return;
    submitError.value = describeError(error, "报名失败，请稍后重试");
  } finally {
    submitting.value = false;
  }
}

/** 打开详情弹窗并拉取时间线 */
async function openDetail(application: RecruitmentApplication): Promise<void> {
  if (!requireSignIn("查看报名详情")) return;
  const session = currentSession.value;
  selectedApplication.value = application;
  detailTimeline.value = [];
  detailError.value = "";
  detailLoading.value = true;
  try {
    const detail = await fetchMyApplication(application.id, requestForPublicPage);
    if (currentSession.value !== session) return;
    detailTimeline.value = detail.timeline;
  } catch (error) {
    if (currentSession.value !== session) return;
    detailError.value = describeError(error, "暂时无法加载时间线，请稍后重试");
  } finally {
    detailLoading.value = false;
  }
}

/** 首页进度三步：按真实状态机（applied→reviewing→accepted/rejected）推导 */
function steps(application: RecruitmentApplication) {
  const status = application.status;
  return [
    {
      title: "提交申请",
      complete: true,
      current: false,
      note: formatISODate(application.created_at),
    },
    {
      title: "初步审核",
      complete: status === "accepted" || status === "rejected",
      current: status === "applied" || status === "reviewing",
      note:
        status === "applied"
          ? "已提交，等待工作室处理"
          : status === "reviewing"
            ? "工作室正在审核"
            : "审核已完成",
    },
    {
      title: "结果",
      complete: status === "accepted",
      current: status === "rejected",
      note:
        status === "accepted"
          ? "已通过，留意后续通知"
          : status === "rejected"
            ? "本次未通过，感谢你的关注"
            : "结果待公布",
    },
  ];
}
</script>
<template>
  <main class="home-view">
    <!-- 页内锚点导航（issue #94 review 保留）：只留探索方向/我的进度两个
         页内跳转；个人设置走 masthead 的用户名菜单，关于我们走页脚 -->
    <nav class="space-nav" aria-label="页面内导航">
      <span class="mono">YOUR PERSONAL SPACE / 2026</span>
      <div>
        <a href="#directions" @click="scrollToSection($event, 'directions')"
          >探索方向</a
        ><a href="#journey" @click="goJourney"
          >我的进度</a
        ><a href="/user/invitations" @click="goInvitations">邀请与结果</a
        ><RouterLink to="/user/about" @click="goAbout($event)"
          >关于我们</RouterLink
        >
      </div>
    </nav>
    <section class="home-poster">
      <div class="mono home-poster-top">
        <span v-if="displayName">HELLO, {{ displayName }}.</span
        ><span v-else>{{ !signedIn ? "欢迎来到 IoT-Studio" : loading ? "正在加载你的资料…" : "个人空间" }}</span
        ><span>一起，连接新的可能。</span>
      </div>
      <h1>
        <span class="title-mask"><span class="title-line">好奇心，</span></span
        ><span class="title-mask"
          ><span class="title-line"><span>现在有了坐标。</span></span></span
        >
      </h1>
      <div class="home-poster-bottom">
        <p>前端、后端与智能体。<br />选择一个起点，把你的想法做成作品。</p>
        <a
          class="action-button"
          href="#directions"
          @click="scrollToSection($event, 'directions')"
          ><span>找到我的方向</span><b aria-hidden="true">↘</b></a
        >
      </div>
      <div class="signal-mark home-mark" aria-hidden="true">
        <i style="--r: 0deg"></i><i style="--r: 45deg"></i
        ><i style="--r: 90deg"></i><i style="--r: 135deg"></i><b></b>
      </div>
      <div class="signal-scanner" aria-hidden="true"></div>
      <div class="signal-halo" aria-hidden="true"></div>
    </section>
    <div class="space-stats">
      <div>
        <span class="mono">OPEN DIRECTIONS</span
        ><strong
          ><span>{{
            String(openDirectionCount).padStart(2, "0")
          }}</span
          ><small>个开放方向</small></strong
        >
      </div>
      <div>
        <span class="mono">MY APPLICATIONS</span
        ><strong v-if="signedIn"
          ><span>{{
            String(applications.length).padStart(2, "0")
          }}</span
          ><small>条报名记录</small></strong
        >
        <p v-else class="guest-stat">登录后可查看</p>
      </div>
      <div class="stat-statement profile-readout" aria-label="本人资料">
        <span class="mono">PROFILE / YOUR IDENTITY</span>
        <p v-if="!signedIn" class="guest-stat">登录后可查看
          <button class="plain-link" @click="requireSignIn('查看个人资料')">去登录 ↗</button>
        </p>
        <p v-else-if="loading" role="status">正在加载用户信息…</p>
        <div v-else-if="userError" role="alert">
          {{ userError }}
          <button class="plain-link" @click="loadProfile">重试 ↗</button>
        </div>
        <strong v-else-if="user"
          ><span class="profile-summary"
            ><span>{{ displayName }}</span
            ><br /><span>{{ user.detail?.class || "未填写" }}</span></span
          ><RouterLink
            v-if="showProfileEntry"
            class="profile-jump"
            to="/user/settings"
            :aria-label="profileEntry.label"
            @click="goSettings($event)"
            >↗</RouterLink
          ></strong
        >
      </div>
    </div>
    <section id="directions" class="direction-section">
      <div class="section-top">
        <div>
          <span class="mono">01 / FIND YOUR FREQUENCY</span>
          <h2>找到你的频率。</h2>
        </div>
        <div class="rail-controls">
          <span class="mono"
            >{{ String(railIndex + 1).padStart(2, "0") }} /
            {{ String(directions.length).padStart(2, "0") }}</span
          ><button
            aria-label="上一个方向"
            :disabled="railIndex === 0"
            @click="move(railIndex - 1)"
          >
            ←</button
          ><button
            aria-label="下一个方向"
            :disabled="railIndex >= directions.length - 1"
            @click="move(railIndex + 1)"
          >
            →
          </button>
        </div>
      </div>
      <p v-if="directionsLoading" role="status">正在加载方向…</p>
      <div v-else-if="directionsError" role="alert">
        {{ directionsError }}
        <button class="plain-link" @click="loadDirections">重试 ↗</button>
      </div>
      <p v-else-if="!directions.length">本轮暂无可报名的方向。</p>
      <div
        v-else
        ref="rail"
        class="direction-rail"
        tabindex="0"
        aria-label="方向展示，可横向滚动，方向键切换"
        @scroll.passive="updateIndex"
        @keydown="railKey"
      >
        <article
          v-for="(direction, index) in directions"
          :key="direction.id"
          class="direction-panel"
        >
          <span class="direction-number" aria-hidden="true">{{
            String(index + 1).padStart(2, "0")
          }}</span>
          <div class="mono panel-top">
            <span>{{ presentationOf(direction.code).headline }}</span
            ><span>{{ direction.open ? "开放报名" : "暂未开放" }}</span>
          </div>
          <div class="direction-heading">
            <h3>{{ direction.name }}<br />。</h3>
            <span
              class="direction-symbol"
              :class="{ 'backend-symbol': direction.code === 'back-end' }"
              aria-hidden="true"
              >{{ presentationOf(direction.code).symbol }}</span
            >
          </div>
          <div class="panel-bottom">
            <div>
              <p>{{ presentationOf(direction.code).summary }}</p>
              <span class="mono">{{ presentationOf(direction.code).tag }}</span>
            </div>
            <button
              class="panel-button"
              :disabled="!direction.open"
              @click="openDirection(direction)"
            >
              {{
                !direction.open
                  ? "暂未开放"
                  : appliedCodes.has(direction.code)
                    ? "查看"
                    : "了解并报名"
              }}
              <span>↗</span>
            </button>
          </div>
        </article>
      </div>
    </section>
    <section id="journey" class="journey-section">
      <div class="section-top">
        <div>
          <span class="mono">02 / YOUR NEXT STEP</span>
          <h2>每一步，都算数。</h2>
        </div>
        <button
          v-if="applications.length"
          class="plain-link journey-label"
          @click="showApplications = true"
        >
          查看全部 {{ applications.length }} 条报名记录 ↗
        </button>
      </div>
      <div v-if="!signedIn" class="guest-journey">
        <p>登录后查看你的报名记录与审核进度。</p>
        <button type="button" class="panel-button" @click="requireSignIn('查看我的进度')">登录查看我的进度 <span aria-hidden="true">↗</span></button>
      </div>
      <p v-else-if="applicationsLoading" role="status">正在加载报名记录…</p>
      <div v-else-if="applicationsError" role="alert">
        {{ applicationsError }}
        <button class="plain-link" @click="loadApplications">重试 ↗</button>
      </div>
      <p v-else-if="!applications.length">
        还没有报名记录 —— 从上方的方向卡开始你的第一站。
      </p>
      <div v-else class="journey-sheet">
        <div class="journey-summary">
          <div>
            <span class="mono"
              >APPLICATION /
              {{ String(selectedRecordIndex + 1).padStart(3, "0") }}</span
            >
            <h3>
              {{
                directions.find(
                  (d) => d.code === activeApplication?.direction_code,
                )?.name || "方向已下线"
              }}
            </h3>
            <p>{{ formatISODate(activeApplication!.created_at) }} 提交</p>
          </div>
          <button
            class="status-tag"
            aria-label="切换报名记录"
            title="切换报名记录"
            @click="showApplications = true"
          >
            <i class="status-dot"></i
            >{{ STATUS_LABELS[activeApplication!.status] }}</button
          ><button class="plain-link" @click="openDetail(activeApplication!)">
            查看详情 ↗
          </button>
        </div>
        <ol class="journey-steps">
          <li
            v-for="(step, i) in steps(activeApplication!)"
            :key="step.title"
            :class="{ complete: step.complete, current: step.current }"
          >
            <span class="step-num">0{{ i + 1 }}</span>
            <div>
              <h4>{{ step.title }}</h4>
              <p>{{ step.note }}</p>
            </div>
            <span>{{ step.complete ? "✓" : "↗" }}</span>
          </li>
        </ol>
      </div>
    </section>
    <section class="closing-note">
      <span class="mono">ONE MORE THING.</span>
      <p>不必什么都会。<br /><span>愿意动手，就有可能。</span></p>
      <a
        v-if="showProfileEntry"
        class="panel-button"
        href="/user/settings"
        @click="goSettings($event)"
        >{{ profileEntry.label }} <span>↗</span></a
      >
    </section>
    <SignInPrompt :action="signInAction" @close="dismiss" @confirm="signIn" />
    <SignalDialog
      v-if="selectedDirection"
      :title="selectedDirection.name"
      @close="selectedDirection = null"
      ><p class="dialog-description">
        {{ presentationOf(selectedDirection.code).summary }}
      </p>
      <div class="direction-facts">
        <div>
          <span>状态</span
          ><strong>{{
            selectedDirection.open ? "开放报名" : "暂未开放"
          }}</strong>
        </div>
        <div>
          <span>资料</span
          ><RouterLink
            v-if="showProfileEntry"
            class="plain-link"
            to="/user/settings"
            @click="goSettings($event)"
            >{{ profileEntry.label }} ↗</RouterLink
          ><span v-else class="mono">创始账号</span>
        </div>
      </div>
      <p class="dialog-note">
        两条路径共用同一条申请：直接报名无需填表；面试需填写该方向的面试表单。
        提交后可在首页「我的进度」查看审核进展。
      </p>
      <!-- 报名路径（issue #136）：无需填表，已有申请时置灰为「已报名」 -->
      <button
        class="action-button"
        :disabled="
          !selectedDirection.open ||
          submitting ||
          appliedCodes.has(selectedDirection.code)
        "
        :aria-busy="submitting"
        @click="handleApply"
      >
        <span>{{
          submitting
            ? "提交中…"
            : appliedCodes.has(selectedDirection.code)
              ? "已报名"
              : "报名 · 无需填表"
        }}</span
        ><b aria-hidden="true">↗</b>
      </button>
      <!-- 面试路径（issue #136）：跳转该方向绑定的表单填写页，不内嵌 -->
      <button
        class="action-button"
        :disabled="interviewBlockReason !== null"
        @click="handleInterview"
      >
        <span>{{
          interviewBlockReason ?? "面试 · 填写该方向表单"
        }}</span
        ><b aria-hidden="true">↗</b>
      </button>
      <p v-if="submitError" class="dialog-error" role="alert">
        {{ submitError }}
      </p>
      <p v-if="submitNotice" role="status">{{ submitNotice }}</p></SignalDialog
    >
    <SignalDialog
      v-if="selectedApplication"
      :title="
        directions.find((d) => d.code === selectedApplication?.direction_code)
          ?.name || '申请详情'
      "
      @close="selectedApplication = null"
      ><p class="dialog-description">
        {{ formatISODate(selectedApplication.created_at) }} 提交 ·
        {{ STATUS_LABELS[selectedApplication.status] }}
      </p>
      <p v-if="detailLoading" role="status">正在加载时间线…</p>
      <p v-else-if="detailError" class="dialog-error" role="alert">
        {{ detailError }}
      </p>
      <ol v-else class="detail-steps">
        <li v-for="entry in detailTimeline" :key="entry.id">
          <strong>{{
            STATUS_LABELS[entry.to_status] ?? entry.to_status
          }}</strong>
          <p>{{ formatISODate(entry.created_at) }}</p>
        </li>
      </ol></SignalDialog
    >
    <SignalDialog
      v-if="showApplications"
      title="全部报名"
      @close="showApplications = false"
      ><p class="dialog-note">选择一条查看首页进度。</p>
      <div class="application-picker">
        <button
          v-for="(application, index) in applications"
          :key="application.id"
          class="outline-button"
          @click="
            selectedRecordIndex = index;
            showApplications = false;
          "
        >
          <span
            >{{
              directions.find((d) => d.code === application.direction_code)
                ?.name || "方向已下线"
            }}
            · {{ STATUS_LABELS[application.status] }}</span
          ><span>↗</span>
        </button>
      </div></SignalDialog
    >
  </main>
</template>

<style scoped>
.guest-stat { margin: 14px 0 0; font-size: 14px; line-height: 1.6; }
.guest-stat .plain-link { margin-left: 12px; min-height: 44px; }
.guest-journey {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 24px 0;
  border-top: 1px solid var(--line);
}
.guest-journey p { margin: 0; line-height: 1.7; }
</style>
