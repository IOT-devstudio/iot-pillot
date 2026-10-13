<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { AuthRequestError } from "@/api/auth";
import { fetchMyProfile, type MyProfile } from "@/api/profile";
import SignalDialog from "@/modules/swiss-signal/SignalDialog.vue";
import {
  findDirection,
  MY_APPLICATIONS,
  RECRUITMENT_DIRECTIONS,
  type DirectionApplication,
  type RecruitmentDirection,
} from "../fixtures";
import { formatISODate } from "../format";
const user = ref<MyProfile | null>(null);
const userError = ref("");
const loading = ref(true);
const rail = ref<HTMLElement | null>(null);
const railIndex = ref(0);
const selectedDirection = ref<RecruitmentDirection | null>(null);
const selectedApplication = ref<DirectionApplication | null>(null);
const intent = ref<RecruitmentDirection | null>(null);
const applications = MY_APPLICATIONS;
const showApplications = ref(false);
const selectedRecordIndex = ref(0);
const activeApplications = computed(() => {
  const item = applications[selectedRecordIndex.value];
  return item ? [item] : [];
});
const directions = RECRUITMENT_DIRECTIONS;
const displayName = computed(() => user.value?.name || user.value?.username);
const labels = {
  pending: "待处理",
  invited: "已邀请",
  accepted: "已接受",
  rejected: "未通过",
};
const tags = [
  "VUE / TYPESCRIPT / EXPERIENCE",
  "GO / API / POSTGRESQL",
  "LLM / TOOLS / WORKFLOW",
];
const headlines = [
  "INTERFACE / EXPERIENCE",
  "DATA / SYSTEM",
  "MODEL / WORKFLOW",
];
const symbols = ["↗", "{ }", "⌁"];
async function load() {
  loading.value = true;
  userError.value = "";
  try {
    user.value = await fetchMyProfile();
  } catch (error) {
    user.value = null;
    userError.value =
      error instanceof AuthRequestError
        ? error.message
        : "暂时无法加载用户信息，请稍后重试";
  } finally {
    loading.value = false;
  }
}
onMounted(load);
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
  const target = Math.max(0, Math.min(directions.length - 1, index));
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
        ? directions.length - 1
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
function steps(application: DirectionApplication) {
  const stage =
    application.status === "accepted"
      ? 3
      : application.status === "invited"
        ? 2
        : 1;
  return ["提交申请", "初步审核", "邀请交流"].map((title, i) => ({
    title,
    complete: i < stage,
    current: i === stage,
    note:
      i < stage
        ? "已完成 · 演示记录"
        : i === stage
          ? `${labels[application.status]} · 演示状态`
          : "后续阶段 · 演示流程",
  }));
}
</script>
<template>
  <main class="home-view">
    <!-- 页内导航已收敛到壳层 masthead（issue #94）：用户名菜单进顶栏，
         本页不再单挂一条 space-nav；探索方向/我的进度由页内锚点按钮直达 -->
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
        <a class="action-button" href="#directions"
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
          ><span :data-count="directions.filter((d) => d.isOpen).length">{{
            String(directions.filter((d) => d.isOpen).length).padStart(2, "0")
          }}</span
          ><small>个演示方向</small></strong
        >
      </div>
      <div>
        <span class="mono">MY APPLICATIONS</span
        ><strong
          ><span :data-count="applications.length">{{
            String(applications.length).padStart(2, "0")
          }}</span
          ><small>条演示记录</small></strong
        >
      </div>
      <div class="stat-statement profile-readout" aria-label="本人资料">
        <span class="mono">PROFILE / YOUR IDENTITY</span>
        <p v-if="loading" role="status">正在加载用户信息…</p>
        <div v-else-if="userError" role="alert">
          {{ userError }}
          <button class="plain-link" @click="load">重试 ↗</button>
        </div>
        <strong v-else-if="user"
          ><span class="profile-summary"
            ><span>{{ displayName }}</span
            ><br /><span>{{ user.detail?.class || "未填写" }}</span></span
          ><RouterLink
            class="profile-jump"
            to="/user/settings"
            aria-label="完善我的资料"
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
            :disabled="railIndex === directions.length - 1"
            @click="move(railIndex + 1)"
          >
            →
          </button>
        </div>
      </div>
      <div
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
            <span>{{ headlines[index] }}</span
            ><span>探索方向 · 演示</span>
          </div>
          <div class="direction-heading">
            <h3>{{ direction.title.replace("开发", "") }}<br />开发。</h3>
            <span
              class="direction-symbol"
              :class="{ 'backend-symbol': index === 1 }"
              aria-hidden="true"
              >{{ symbols[index] }}</span
            >
          </div>
          <div class="panel-bottom">
            <div>
              <p>{{ direction.summary }}</p>
              <span class="mono">{{ tags[index] }}</span>
            </div>
            <button
              class="panel-button"
              :disabled="!direction.isOpen"
              @click="selectedDirection = direction"
            >
              {{ direction.isOpen ? "了解并体验报名" : "暂未开放" }}
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
          class="plain-link journey-label"
          @click="showApplications = true"
        >
          查看全部 {{ applications.length }} 条演示记录 ↗
        </button>
      </div>
      <p v-if="!applications.length">暂无演示申请记录。</p>
      <div
        v-for="application in activeApplications"
        :key="application.id"
        class="journey-sheet"
      >
        <div class="journey-summary">
          <div>
            <span class="mono"
              >APPLICATION /
              {{ String(selectedRecordIndex + 1).padStart(3, "0") }}</span
            >
            <h3>
              {{
                findDirection(application.directionId)?.title || "方向已下线"
              }}
            </h3>
            <p>{{ formatISODate(application.appliedAt) }} · 演示申请</p>
          </div>
          <button
            class="status-tag"
            aria-label="切换演示申请记录"
            title="切换演示申请记录"
            @click="showApplications = true"
          >
            <i class="status-dot"></i>{{ labels[application.status] }} · 演示</button
          ><button
            class="plain-link"
            @click="selectedApplication = application"
          >
            查看详情 ↗
          </button>
        </div>
        <ol class="journey-steps">
          <li
            v-for="(step, i) in steps(application)"
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
      <p v-if="intent" class="new-intents" role="status">
        本次体验选择：{{ intent.title }}（演示，未提交到后端，刷新后重置）
      </p>
    </section>
    <section class="closing-note">
      <span class="mono">ONE MORE THING.</span>
      <p>不必什么都会。<br /><span>愿意动手，就有可能。</span></p>
      <RouterLink class="panel-button" to="/user/settings"
        >完善我的资料 <span>↗</span></RouterLink
      >
    </section>
    <SignalDialog
      v-if="selectedDirection"
      :title="selectedDirection.title"
      @close="selectedDirection = null"
      ><p class="dialog-description">{{ selectedDirection.summary }}</p>
      <div class="direction-facts">
        <div>
          <span>状态</span
          ><strong>{{
            selectedDirection.isOpen ? "开放体验" : "暂未开放"
          }}</strong>
        </div>
        <div>
          <span>资料</span
          ><RouterLink class="plain-link" to="/user/settings"
            >完善个人资料 ↗</RouterLink
          >
        </div>
      </div>
      <p class="dialog-note">
        这是报名交互演示，不会提交真实申请或保存报名数据。
      </p>
      <button
        class="action-button"
        :disabled="!selectedDirection.isOpen"
        @click="intent = selectedDirection"
      >
        <span>体验报名意向</span><b>↗</b>
      </button>
      <p v-if="intent?.id === selectedDirection.id" role="status">
        已选择本次体验方向，未提交到后端。
      </p></SignalDialog
    >
    <SignalDialog
      v-if="selectedApplication"
      :title="
        findDirection(selectedApplication.directionId)?.title || '申请详情'
      "
      @close="selectedApplication = null"
      ><p class="dialog-description">
        {{ formatISODate(selectedApplication.appliedAt) }} ·
        {{ labels[selectedApplication.status] }}
      </p>
      <p class="dialog-note">
        演示申请记录与示例进度，不代表真实报名或审核结果。
      </p>
      <ol class="detail-steps">
        <li v-for="step in steps(selectedApplication)" :key="step.title">
          <strong>{{ step.title }}</strong>
          <p>{{ step.note }}</p>
        </li>
      </ol></SignalDialog
    >
    <SignalDialog
      v-if="showApplications"
      title="全部演示申请"
      @close="showApplications = false"
      ><p class="dialog-note">
        以下记录均为演示，不代表真实报名或审核结果。选择一条查看首页进度。
      </p>
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
              findDirection(application.directionId)?.title || "方向已下线"
            }}
            · {{ labels[application.status] }}</span
          ><span>↗</span>
        </button>
      </div></SignalDialog
    >
  </main>
</template>
