<script setup lang="ts">
/**
 * 管理台 `/dashboard`（工作室成员的工作区）。
 *
 * 内容 = 原来的内容首页（健康检查 + 快捷入口卡片）+ 注册用户表：
 *   components/AdminUserTable  注册用户名单（消费 /api/v1/admin/users）
 *   本组件                      布局 + 接线，并持有后端健康状态这一个状态
 *
 * 原来的 /home 内容首页已并入这里，旧路径由 router/routes.ts 重定向过来。
 * 页面令牌全部来自全局 styles/theme.css，这里不再自带一套颜色。
 */
import { computed, onMounted, ref } from "vue";

import { describeHealthError, fetchHealth } from "@/api/health";
import AppHeader from "@/components/common/AppHeader.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import SignOutButton from "@/components/SignOutButton.vue";

import AdminUserTable from "../components/AdminUserTable.vue";

const health = ref<string>("检测中…");

const healthy = computed(() => health.value.startsWith("ok"));

onMounted(async () => {
  try {
    const data = await fetchHealth();
    health.value = `${data.status} @ ${data.time}`;
  } catch (e) {
    // 走分类翻译器，不直接拼 (e as Error).message——原始异常（如
    // Unexpected token '<'…）不该出现在界面上，见 issue #56
    health.value = describeHealthError(e);
  }
});

/** 快捷入口：按招新工作的使用频率排序；用户与权限归到设置类，排在系统设置之前 */
const entries = [
  { title: "意向成员", to: "/recruitment/prospects" },
  { title: "表单管理", to: "/forms" },
  { title: "邮件模板", to: "/templates" },
  { title: "用户与权限", to: "/dashboard/admins" },
  { title: "系统设置", to: "/settings" },
];
</script>

<template>
  <div class="page">
    <AppHeader />

    <main class="page__body">
      <PageHeader eyebrow="iot-pillot · 招新管理" title="控制台">
        <template #actions>
          <SignOutButton />
        </template>
      </PageHeader>

      <section class="health" :class="healthy ? 'health--ok' : 'health--down'">
        <span class="health__dot" aria-hidden="true"></span>
        <div class="health__body">
          <span class="health__label">后端服务</span>
          <span class="health__value">{{ health }}</span>
        </div>
      </section>

      <section class="entries" aria-label="快捷入口">
        <router-link v-for="e in entries" :key="e.to" :to="e.to" class="entry">
          <span class="entry__title">{{ e.title }}</span>
          <span class="entry__arrow" aria-hidden="true">→</span>
        </router-link>
      </section>

      <AdminUserTable />
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
}

.page__body {
  display: flex;
  flex-direction: column;
  gap: 28px;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

/* —— 服务状态 —— */
.health {
  display: flex;
  gap: 14px;
  align-items: center;
  padding: 16px 20px;
  border: 1px solid var(--line);
  border-radius: 2px;
  background: var(--surface);
}

.health__dot {
  width: 10px;
  height: 10px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--ink-faint);
}

.health--ok .health__dot {
  background: var(--green);
  box-shadow: 0 0 0 3px rgb(46 139 87 / 16%);
}

.health--down .health__dot {
  background: var(--red);
  box-shadow: 0 0 0 3px rgb(184 66 53 / 16%);
}

.health__body {
  display: flex;
  gap: 12px;
  align-items: baseline;
}

.health__label {
  color: var(--ink-soft);
  font-size: 13px;
  font-weight: 700;
}

.health__value {
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 13px;
}

/* —— 快捷入口 —— */
.entries {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: clamp(16px, 1.4vw, 22px);
}

.entry {
  position: relative;
  display: flex;
  gap: clamp(18px, 1.8vw, 24px);
  align-items: center;
  min-height: clamp(140px, 12vw, 176px);
  padding: clamp(28px, 3vw, 40px) clamp(28px, 3.2vw, 44px);
  overflow: hidden;
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 2px;
  background: var(--surface);
  text-decoration: none;
  transition: border-color 180ms ease, box-shadow 180ms ease, transform 180ms ease;
}

.entry::before {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 3px;
  background: var(--blue);
  content: "";
  opacity: 0.34;
  transition: opacity 180ms ease;
}

.entry:hover {
  border-color: var(--line-strong);
  box-shadow: var(--el-box-shadow);
  transform: translateY(-2px);
}

.entry:hover::before {
  opacity: 1;
}

.entry:focus-visible {
  border-color: var(--blue);
  outline: 2px solid var(--blue);
  outline-offset: 2px;
}

.entry__title {
  font-family: var(--font-serif);
  font-size: clamp(24px, 2.2vw, 30px);
  font-weight: 600;
  letter-spacing: -0.01em;
}

.entry__arrow {
  margin-left: auto;
  color: var(--ink-faint);
  font-size: clamp(18px, 1.6vw, 22px);
  transition: color 180ms ease, transform 180ms ease;
}

.entry:hover .entry__arrow {
  color: var(--blue);
  transform: translateX(3px);
}

@media (max-width: 1024px) {
  .entry {
    min-height: clamp(132px, 15vw, 156px);
    padding: 28px 30px;
  }
}

@media (max-width: 720px) {
  .entries {
    grid-template-columns: 1fr;
    gap: 14px;
  }

  .entry {
    min-height: 0;
    gap: 16px;
    padding: 24px 22px;
  }

  .entry__title {
    font-size: 23px;
  }
}

@media (max-width: 420px) {
  .entry__title {
    font-size: 21px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .entry {
    transition: none;
  }

  .entry:hover {
    transform: none;
  }

  .entry:hover .entry__arrow {
    transform: none;
  }
}
</style>
