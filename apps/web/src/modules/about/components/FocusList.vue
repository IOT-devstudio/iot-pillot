<script setup lang="ts">
/**
 * 方向 / 定位（纯展示组件）。
 *
 * 这里展示的是工作室**长期的技术方向**，与 home 模块「招新方向」
 * （`RecruitmentDirection`，带 `isOpen`、随时会关闭）不是一回事，
 * 所以两边各存一份数据、不互相 import —— 详见 ../fixtures.ts 头部说明。
 *
 * 单条方向不单独拆组件：每条只有标题 + 一句话 + 一行栈标签，
 * 按「当下用不到的抽象不写」内联 v-for 即可。
 */
import type { StudioFocus } from "../fixtures";

defineProps<{
  focuses: StudioFocus[];
}>();
</script>

<template>
  <section class="focus" aria-labelledby="about-focus-title">
    <h2 id="about-focus-title" class="focus__title">方向 / 定位</h2>

    <ul class="focus__list">
      <li v-for="focus in focuses" :key="focus.id" class="focus__item">
        <h3 class="focus__name">{{ focus.title }}</h3>
        <p class="focus__summary">{{ focus.summary }}</p>

        <ul class="focus__stack">
          <li v-for="tech in focus.stack" :key="tech" class="focus__chip">
            {{ tech }}
          </li>
        </ul>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.focus {
  padding: 18px 20px;
  border: 1px solid var(--about-border);
  border-radius: 10px;
  background: #fff;
}

.focus__title {
  margin: 0 0 14px;
  color: var(--about-ink);
  font-size: 15px;
}

.focus__list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.focus__item {
  padding: 14px 16px;
  border: 1px solid var(--about-border);
  border-radius: 8px;
}

.focus__name {
  margin: 0 0 6px;
  color: var(--about-ink);
  font-size: 14px;
}

.focus__summary {
  margin: 0;
  color: var(--about-muted);
  font-size: 13px;
  line-height: 1.6;
}

.focus__stack {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 12px 0 0;
  padding: 0;
  list-style: none;
}

.focus__chip {
  padding: 2px 10px;
  color: var(--about-accent);
  border: 1px solid var(--about-border);
  border-radius: 999px;
  background: #f4f7fa;
  font-size: 11px;
  white-space: nowrap;
}
</style>
