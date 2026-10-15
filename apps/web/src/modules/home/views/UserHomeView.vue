<script setup lang="ts">
/**
 * 用户侧个人首页 `/user/home`（issue #89：接入真实招新数据）。
 *
 * 数据全部来自后端（需登录，路由守卫已把关；未登录访问会被带到登录页
 * 并带上 ?redirect= 原目标）：
 *   - GET  /api/v1/recruitment/directions        方向与开放状态
 *   - POST /api/v1/recruitment/applications      报名（归属由令牌决定）
 *   - GET  /api/v1/recruitment/applications      本人申请（进度/统计）
 *   - GET  /api/v1/recruitment/applications/:id  详情（时间线）
 * 卡片标语等纯视觉元数据在 directionPresentation.ts，不属于接口契约。
 *
 * 三块数据彼此独立加载：方向挂了不该拖垮进度区，反之亦然。
 */
import { computed, inject, nextTick, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { AuthRequestError } from "@/api/auth";
import { fetchMyProfile } from "@/api/profile";
import {
  beginProfileLoad,
  currentProfile as user,
  profileDisplayName,
} from "@/modules/user-settings/profileIdentity";
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
import { presentationOf } from "../directionPresentation";
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
  void transition(() => router.push("/user/settings"));
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
const openDirectionCount = computed(
  () => directions.value.filter((direction) => direction.open).length,
);
const appliedCodes = computed(
  () =>
    new Set(
      applications.value.map((application) => application.direction_code),
    ),
);

function describeError(error: unknown, fallback: string): string {
  return error instanceof AuthRequestError ? error.message : fallback;
}

async function loadProfile(): Promise<void> {
  const publish = beginProfileLoad();
  loading.value = true;
  userError.value = "";
  try {
    publish(await fetchMyProfile());
  } catch (error) {
    publish(null);
    userError.value = describeError(error, "暂时无法加载用户信息，请稍后重试");
  } finally {
    loading.value = false;
  }
}

async function loadDirections(): Promise<void> {
  directionsLoading.value = true;
  directionsError.value = "";
  try {
    directions.value = (await fetchDirections()).items;
  } catch (error) {
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
  applicationsLoading.value = true;
  applicationsError.value = "";
  try {
    applications.value = (await fetchMyApplications()).items;
    if (selectedRecordIndex.value >= applications.value.length) {
      selectedRecordIndex.value = 0;
    }
  } catch (error) {
    applications.value = [];
    applicationsError.value = describeError(
      error,
      "暂时无法加载报名记录，请稍后重试",
    );
  } finally {
    applicationsLoading.value = false;
  }
}

onMounted(() => {
  void loadProfile();
  void loadDirections();
  void loadApplications();
  void scrollToHashSection();
});

/**
 * 外链 hash 滚动（个人设置「去选择方向」→ /user/home#directions）。
 *
 * Vue Router 不会为 SPA 内跳转自动滚到锚点；且方向/进度区的骨架在接口
 * 数据回来前就有高度，先滚再撑高会把版块顶下去（review 实测差了 200px）。
 * 所以先等三块数据加载完再滚，目标找不到时轮询最多 1.5s 兜底。
 * 系统开了「减少动态效果」时退回瞬时跳转，与 scrollToSection 口径一致。
 */
async function scrollToHashSection(): Promise<void> {
  const id = route.hash.replace(/^#/, "");
  if (!id) return;
  await nextTick();
  // 等数据加载完：骨架→内容的切换会改变版块高度，早滚会偏
  const dataDeadline = Date.now() + 3000;
  while (
    (directionsLoading.value || applicationsLoading.value) &&
    Date.now() < dataDeadline
  ) {
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  const deadline = Date.now() + 1500;
  let target = document.getElementById(id);
  while (!target && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 100));
    target = document.getElementById(id);
  }
  if (!target) return;
  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
  target.scrollIntoView({
    behavior: reduced ? "auto" : "smooth",
    block: "start",
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
  selectedDirection.value = direction;
  submitError.value = "";
  submitNotice.value = "";
}

/**
 * 真实报名（issue #89）。成功后刷新本人申请——记录以服务端为准，
 * 不在前端乐观插入；失败原地提示，可安全重试（后端唯一约束兜底重复）。
 */
async function handleApply(): Promise<void> {
  const direction = selectedDirection.value;
  if (direction === null || submitting.value) return;
  submitError.value = "";
  submitNotice.value = "";
  submitting.value = true;
  try {
    await submitApplication(direction.code);
    submitNotice.value = "报名成功，可在「我的进度」查看审核进展。";
    await loadApplications();
  } catch (error) {
    submitError.value = describeError(error, "报名失败，请稍后重试");
  } finally {
    submitting.value = false;
  }
}

/** 打开详情弹窗并拉取时间线 */
async function openDetail(application: RecruitmentApplication): Promise<void> {
  selectedApplication.value = application;
  detailTimeline.value = [];
  detailError.value = "";
  detailLoading.value = true;
  try {
    const detail = await fetchMyApplication(application.id);
    detailTimeline.value = detail.timeline;
  } catch (error) {
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
        ><a href="#journey" @click="scrollToSection($event, 'journey')"
          >我的进度</a
        ><RouterLink to="/user/invitations">邀请与结果</RouterLink
        ><RouterLink to="/user/about" @click="goAbout($event)"
          >关于我们</RouterLink
        >
      </div>
    </nav>
    <section class="home-poster">
      <div class="mono home-poster-top">
        <span v-if="displayName">HELLO, {{ displayName }}.</span
        ><span v-else>{{ loading ? "正在加载你的资料…" : "个人空间" }}</span
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
          ><span :data-count="openDirectionCount">{{
            String(openDirectionCount).padStart(2, "0")
          }}</span
          ><small>个开放方向</small></strong
        >
      </div>
      <div>
        <span class="mono">MY APPLICATIONS</span
        ><strong
          ><span :data-count="applications.length">{{
            String(applications.length).padStart(2, "0")
          }}</span
          ><small>条报名记录</small></strong
        >
      </div>
      <div class="stat-statement profile-readout" aria-label="本人资料">
        <span class="mono">PROFILE / YOUR IDENTITY</span>
        <p v-if="loading" role="status">正在加载用户信息…</p>
        <div v-else-if="userError" role="alert">
          {{ userError }}
          <button class="plain-link" @click="loadProfile">重试 ↗</button>
        </div>
        <strong v-else-if="user"
          ><span class="profile-summary"
            ><span>{{ displayName }}</span
            ><br /><span>{{ user.detail?.class || "未填写" }}</span></span
          ><RouterLink
            class="profile-jump"
            to="/user/settings"
            aria-label="完善我的资料"
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
              :disabled="!direction.open || appliedCodes.has(direction.code)"
              @click="openDirection(direction)"
            >
              {{
                !direction.open
                  ? "暂未开放"
                  : appliedCodes.has(direction.code)
                    ? "已报名"
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
      <p v-if="applicationsLoading" role="status">正在加载报名记录…</p>
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
      <RouterLink
        class="panel-button"
        to="/user/settings"
        @click="goSettings($event)"
        >完善我的资料 <span>↗</span></RouterLink
      >
    </section>
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
            class="plain-link"
            to="/user/settings"
            @click="goSettings($event)"
            >完善个人资料 ↗</RouterLink
          >
        </div>
      </div>
      <p class="dialog-note">
        报名将提交真实申请，可在首页「我的进度」查看审核进展。
      </p>
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
              : "确认报名"
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
