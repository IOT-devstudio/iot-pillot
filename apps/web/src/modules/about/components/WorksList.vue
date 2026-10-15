<script setup lang="ts">
/**
 * 成果 / 项目（纯展示组件）。
 *
 * 未经工作室确认的成果不展示（issue #94）：fixtures 为空数组时渲染
 * 「成果待补充」空态，而不是编造的示例项目。组件不持状态、不发请求。
 */
import type { StudioWork } from "../fixtures";

defineProps<{
  works: StudioWork[];
}>();
</script>

<template>
  <section class="works liquid-glass" aria-labelledby="about-works-title">
    <h2 id="about-works-title" class="works__title">成果</h2>

    <p v-if="!works.length" class="works__empty">
      成果待补充：正式项目清单确认后展示。
    </p>

    <ul v-else class="works__list">
      <li v-for="work in works" :key="work.id" class="works__item">
        <div class="works__head">
          <h3 class="works__name">{{ work.title }}</h3>
          <span class="works__year">{{ work.year }}</span>
        </div>
        <p class="works__summary">{{ work.summary }}</p>
      </li>
    </ul>
  </section>
</template>

<style scoped>
/* 玻璃质感来自全局 .liquid-glass（issue #85 改造）；
   卡内小卡（方向/成果/成员项）保持实底描边，保证内容可读 */
.works {
  padding: 18px 20px;
}

.works__title {
  margin: 0 0 14px;
  color: var(--about-ink);
  font-size: 15px;
}

.works__empty {
  margin: 0;
  padding: 14px 16px;
  color: var(--about-muted);
  border: 1px dashed var(--about-border);
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.6;
}

.works__list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.works__item {
  padding: 14px 16px;
  border: 1px solid var(--about-border);
  border-radius: 8px;
}

.works__head {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  align-items: baseline;
  margin-bottom: 6px;
}

.works__name {
  margin: 0;
  color: var(--about-ink);
  font-size: 14px;
}

.works__year {
  color: var(--about-muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.works__summary {
  margin: 0;
  color: var(--about-muted);
  font-size: 13px;
  line-height: 1.6;
}
</style>
