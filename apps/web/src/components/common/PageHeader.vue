<script setup lang="ts">
/**
 * 页面档案头：技术 eyebrow + 宋体大标题 + 一句说明。
 *
 * 右侧 actions 插槽放主操作（状态标签、按钮），meta 放技术标注
 * （如「共 5 人」「编号 001—005」）。纯展示组件，无业务逻辑。
 */
defineProps<{
  /** 顶部技术标注，如「招新管理 · 意向档案」 */
  eyebrow?: string;
  title: string;
  description?: string;
  /** 右侧技术标注，如「共 5 人」 */
  meta?: string;
}>();
</script>

<template>
  <header class="page-head">
    <div class="page-head__main">
      <p v-if="eyebrow" class="page-head__eyebrow">{{ eyebrow }}</p>
      <h1 class="page-head__title">{{ title }}</h1>
      <p v-if="description" class="page-head__desc">{{ description }}</p>
    </div>

    <div class="page-head__aside">
      <slot name="actions" />
      <span v-if="meta" class="page-head__meta">{{ meta }}</span>
    </div>
  </header>
</template>

<style scoped>
.page-head {
  display: flex;
  gap: 24px;
  align-items: flex-end;
  justify-content: space-between;
}

.page-head__eyebrow {
  display: flex;
  gap: 12px;
  align-items: center;
  margin: 0 0 16px;
  color: var(--ink-soft);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.page-head__eyebrow::before {
  width: 32px;
  height: 2px;
  background: var(--red);
  content: "";
}

.page-head__title {
  margin: 0;
  font-family: var(--font-serif);
  font-size: clamp(34px, 4.4vw, 52px);
  font-weight: 600;
  line-height: 1.05;
  letter-spacing: -0.03em;
}

.page-head__desc {
  max-width: 560px;
  margin: 16px 0 0;
  color: var(--ink-soft);
  font-size: 14px;
  line-height: 1.75;
}

.page-head__aside {
  display: flex;
  gap: 14px;
  align-items: center;
  flex: 0 0 auto;
  padding-bottom: 4px;
}

.page-head__meta {
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 12px;
  letter-spacing: 0.04em;
  white-space: nowrap;
}

@media (max-width: 640px) {
  .page-head {
    flex-direction: column;
    align-items: flex-start;
  }

  .page-head__aside {
    padding-bottom: 0;
  }
}
</style>
