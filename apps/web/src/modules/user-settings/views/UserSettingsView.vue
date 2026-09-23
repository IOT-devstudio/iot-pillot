<script setup lang="ts">
/**
 * 个人设置页 `/user/settings`（占位实现）。
 *
 * 后续 commit 会在本分支上叠加：
 *   - GET /me 拿完整资料字段
 *   - 方向下拉与表单校验
 *   - PUT /me 保存
 *
 * 当前仅确认路由能进入、内容区形态与 home/about 一致。
 */
import AppHeader from "@/components/common/AppHeader.vue";
import BackToDashboard from "@/components/common/BackToDashboard.vue";
import PageHeader from "@/components/common/PageHeader.vue";
</script>

<template>
  <div class="page">
    <AppHeader />

    <main class="page__body">
      <BackToDashboard />

      <PageHeader
        eyebrow="iot-pillot · 用户侧"
        title="个人设置"
        description="补全与维护你自己的资料。姓名、邮箱为登录凭据，本期不在此处变更。"
      />

      <section class="panel panel--pad placeholder-card">
        <p class="placeholder-card__index" aria-hidden="true">开发中</p>
        <p class="placeholder-card__note">
          路由已接入，表单、保存与字段校验随后续 commit 落地。
        </p>
      </section>
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

.placeholder-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 220px;
}

.placeholder-card__index {
  margin: 0;
  color: var(--red);
  font-family: var(--font-kai);
  font-size: 18px;
}

.placeholder-card__note {
  margin: 0;
  color: var(--ink-soft);
  font-size: 13px;
  line-height: 1.7;
}
</style>