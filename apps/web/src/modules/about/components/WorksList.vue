<script setup lang="ts">
/**
 * 成果 / 项目（纯展示组件）。
 *
 * 内容全部是占位示例（见 ../fixtures.ts 头部说明），页面底部另有一行标注。
 * 组件不持状态、不发请求，数据由页面传入。
 */
import type { StudioWork } from "../fixtures";

defineProps<{
  works: StudioWork[];
}>();
</script>

<template>
  <section class="works" aria-labelledby="about-works-title">
    <h2 id="about-works-title" class="works__title">成果</h2>

    <ul class="works__list">
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
.works {
  padding: 18px 20px;
  border: 1px solid var(--about-border);
  border-radius: 10px;
  background: #fff;
}

.works__title {
  margin: 0 0 14px;
  color: var(--about-ink);
  font-size: 15px;
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
