<script setup lang="ts">
/**
 * 关于我们 `/user/about`。
 *
 * 纯展示页：没有交互、不发请求、不持状态。
 *
 * 分工：
 *   fixtures.ts                   静态文案与假数据
 *   components/StudioIntroCard    工作室简介 + 数字
 *   components/FocusList          方向 / 定位
 *   components/WorksList          成果
 *   components/MemberList         成员
 *   本组件                         布局 + 接线
 *
 * ⚠️ 页面内容**全部是占位示例**（见 fixtures.ts 头部说明），所以底部有一行
 * 标注，免得被当成工作室的真实资料。
 *
 * 布局与视觉沿用 home 模块（`/user/home`）：`__inner` 是唯一的宽度容器，
 * `__body` 只负责卡片间距，因此四块卡片天然等宽、整页水平居中。
 *
 * 设计令牌在这里又定义了一份（`--about-*`，值与 `--home-*` 相同）。
 * 之所以不共用：`src/assets/` 全局样式目录还没建（CLAUDE.md 规划了，
 * 但「只有真实需要时才创建」），而本步骤只允许改 about 模块下的文件。
 * M3 做设计基线时应当把这两套抽到全局。
 */
import {
  STUDIO_FOCUS,
  STUDIO_INTRO,
  STUDIO_MEMBERS,
  STUDIO_WORKS,
} from "../fixtures";
import FocusList from "../components/FocusList.vue";
import MemberList from "../components/MemberList.vue";
import StudioIntroCard from "../components/StudioIntroCard.vue";
import WorksList from "../components/WorksList.vue";

const profile = STUDIO_INTRO;
const focuses = STUDIO_FOCUS;
const works = STUDIO_WORKS;
const members = STUDIO_MEMBERS;
</script>

<template>
  <div class="about">
    <!-- 唯一的宽度容器：标题区和四块卡片都跟着它走，宽度天然一致 -->
    <div class="about__inner">
      <header class="about__header">
        <h1 class="about__title">关于我们</h1>
        <p class="about__subtitle">{{ profile.name }} · 招新项目手册</p>
      </header>

      <div class="about__body">
        <StudioIntroCard :profile="profile" />

        <FocusList :focuses="focuses" />

        <WorksList :works="works" />

        <MemberList :members="members" />
      </div>

      <p class="about__disclaimer">
        以上为占位示例内容，待工作室提供正式资料。
      </p>
    </div>
  </div>
</template>

<style scoped>
/* 设计令牌：与 home（/user/home）用同一套值，避免两个用户侧页面观感割裂 */
.about {
  --about-ink: #102b4e;
  --about-muted: #5c6b7a;
  --about-accent: #164d80;
  --about-border: #d8dee4;
  /* 内容栏宽度：标题与四块卡片共用这一个值，改宽度只改这里 */
  --about-content-width: 720px;

  min-height: 100vh;
  padding: 32px clamp(16px, 4vw, 48px) 48px;
  color: var(--about-ink);
  background: #f4f6f8;
  font-family: "Avenir Next", "PingFang SC", "Microsoft YaHei", sans-serif;
}

/*
 * 内容整体水平居中。
 * 窗口比内容栏窄时 max-width 不生效，由 .about 的左右 padding 兜住，
 * 所以窄屏也不会贴边。
 */
.about__inner {
  max-width: var(--about-content-width);
  margin: 0 auto;
}

.about__header {
  margin-bottom: 20px;
}

.about__title {
  margin: 0;
  font-size: 22px;
}

.about__subtitle {
  margin: 6px 0 0;
  color: var(--about-muted);
  font-size: 13px;
}

/* 只负责卡片间距；宽度交给 .about__inner，四块卡片因此天然等宽 */
.about__body {
  display: grid;
  gap: 16px;
}

.about__disclaimer {
  margin: 16px 0 0;
  color: var(--about-muted);
  font-size: 11px;
  line-height: 1.6;
}
</style>
