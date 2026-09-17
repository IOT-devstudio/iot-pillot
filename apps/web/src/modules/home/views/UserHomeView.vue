<script setup lang="ts">
/**
 * 用户侧首页 `/user/home`。
 *
 * 分工：
 *   fixtures.ts                  招新方向与报名记录的假数据（接口未就绪）
 *   format.ts                    展示用的日期格式化（纯函数）
 *   components/UserProfileCard   当前用户信息区（纯展示）
 *   components/ApplicationList   我的招新情况 / 报名记录（纯展示）
 *   components/DirectionList     招新方向列表（纯展示）
 *   本组件                       布局 + 接线，并持有「刚点了哪条方向」这一个状态
 *
 * 当前用户信息从后端 `/api/v1/me` 获取，路由守卫负责进入前的认证与角色校验；
 * 招新方向和报名记录仍是等待后端接口交付的本地 fixture。
 *
 * 视觉沿用 opener-editor 的设计令牌（品牌蓝 + 深墨色），与开屏、编辑器
 * 保持同一套观感；不引 Element Plus 组件，与本模块手写 UI 的既有做法一致。
 */
import { onMounted, ref } from "vue";

import { AuthRequestError, type CurrentUser } from "@/api/auth";
import { fetchCurrentUserWithRefresh } from "@/auth/current-user";
import { readAuthSession } from "@/auth/session";
import SignOutButton from "@/components/SignOutButton.vue";

import type { RecruitmentDirection } from "../fixtures";
import {
  MY_APPLICATIONS,
  RECRUITMENT_DIRECTIONS,
} from "../fixtures";
import ApplicationList from "../components/ApplicationList.vue";
import DirectionList from "../components/DirectionList.vue";
import UserProfileCard from "../components/UserProfileCard.vue";

const user = ref<CurrentUser | null>(null);
const userError = ref("");
const applications = MY_APPLICATIONS;
const directions = RECRUITMENT_DIRECTIONS;

/** 最近一次点击报名的方向；null 表示还没点过 */
const appliedDirection = ref<RecruitmentDirection | null>(null);

function messageOf(error: unknown): string {
  return error instanceof AuthRequestError
    ? error.message
    : "暂时无法加载用户信息，请稍后重试";
}

onMounted(async () => {
  const session = readAuthSession();
  if (session === null) {
    userError.value = "登录状态已失效，请重新登录";
    return;
  }

  try {
    user.value = await fetchCurrentUserWithRefresh(session.access_token);
  } catch (error: unknown) {
    userError.value = messageOf(error);
  }
});

/**
 * 报名入口的占位实现——**不请求任何接口**。
 *
 * 招新接口就绪前这里只负责「点了有反馈」。提示文案里如实写明是演示数据，
 * 避免演示时被当成真的报名成功了。
 */
function handleApply(direction: RecruitmentDirection): void {
  appliedDirection.value = direction;
  console.info(
    "[UserHomeView] 报名意向（演示数据，未提交到后端）：",
    direction.title,
  );
}
</script>

<template>
  <div class="user-home">
    <!-- 唯一的宽度容器：标题区和三块卡片都跟着它走，宽度天然一致 -->
    <div class="user-home__inner">
      <header class="user-home__header">
        <div>
          <h1 class="user-home__title">用户首页</h1>
          <p class="user-home__subtitle">
            查看你的信息与当前开放的招新方向。
          </p>
        </div>
        <SignOutButton />
      </header>

      <div class="user-home__body">
        <p v-if="userError" class="user-home__error" role="alert">{{ userError }}</p>
        <p v-else-if="user === null" class="user-home__loading" role="status">
          正在加载用户信息…
        </p>
        <UserProfileCard v-else :user="user" />

        <ApplicationList :applications="applications" />

        <DirectionList :directions="directions" @apply="handleApply" />

        <p v-if="appliedDirection" class="user-home__notice" role="status">
          已记录报名意向：{{ appliedDirection.title }}（演示数据，尚未提交到后端）
        </p>
      </div>

      <!-- 底部入口：纯声明式跳转，用 RouterLink 而不是 router.push -->
      <p class="user-home__footer">
        <RouterLink class="user-home__link" to="/user/about">关于我们</RouterLink>
      </p>
    </div>
  </div>
</template>

<style scoped>
/* 设计令牌：与开屏 / 编辑器同一套品牌色，避免三个页面观感割裂 */
.user-home {
  --home-ink: #102b4e;
  --home-muted: #5c6b7a;
  --home-accent: #164d80;
  --home-border: #d8dee4;
  /* 内容栏宽度：标题与三块卡片共用这一个值，改宽度只改这里 */
  --home-content-width: 720px;

  min-height: 100vh;
  padding: 32px clamp(16px, 4vw, 48px) 48px;
  color: var(--home-ink);
  background: #f4f6f8;
  font-family: "Avenir Next", "PingFang SC", "Microsoft YaHei", sans-serif;
}

/*
 * 内容整体水平居中。
 * 窗口比内容栏窄时 max-width 不生效，由 .user-home 的左右 padding 兜住，
 * 所以窄屏也不会贴边。
 */
.user-home__inner {
  max-width: var(--home-content-width);
  margin: 0 auto;
}

.user-home__header {
  margin-bottom: 20px;
}

.user-home__title {
  margin: 0;
  font-size: 22px;
}

.user-home__subtitle {
  margin: 6px 0 0;
  color: var(--home-muted);
  font-size: 13px;
}

/* 只负责卡片间距；宽度交给 .user-home__inner，三块卡片因此天然等宽 */
.user-home__body {
  display: grid;
  gap: 16px;
}

.user-home__notice {
  margin: 0;
  padding: 10px 14px;
  color: #1c6b45;
  border-left: 3px solid #2e8b57;
  border-radius: 8px;
  background: #e9f6ef;
  font-size: 12px;
  line-height: 1.6;
}

.user-home__loading,
.user-home__error {
  margin: 0;
  padding: 18px 20px;
  border: 1px solid var(--home-border);
  border-radius: 10px;
  background: #fff;
  font-size: 13px;
}

.user-home__loading {
  color: var(--home-muted);
}

.user-home__error {
  color: #a33d32;
}

/* 底部入口。放在 __body 之外、__inner 之内，跟 about 页底部的处理一致 */
.user-home__footer {
  margin: 20px 0 0;
  padding-top: 16px;
  border-top: 1px solid var(--home-border);
  font-size: 13px;
}

.user-home__link {
  color: var(--home-accent);
  text-decoration: none;
}

.user-home__link:hover {
  text-decoration: underline;
}
</style>
