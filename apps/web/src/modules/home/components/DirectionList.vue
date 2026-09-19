<script setup lang="ts">
/**
 * 招新方向列表（纯展示组件）。
 *
 * 只负责「长什么样」和「点了通知父级」，报名动作本身由页面决定——
 * 现在页面只是弹个占位提示，将来接了后端也只改页面，不动这里。
 *
 * 单条方向没有拆成独立组件：每条只有标题 + 一句简介 + 一个按钮，
 * 按「当下用不到的抽象不写」内联 v-for 更省事；等条目里真的长出
 * 时间线、标签之类的东西再拆。
 */
import type { RecruitmentDirection } from "../fixtures";

defineProps<{
  directions: RecruitmentDirection[];
}>();

const emit = defineEmits<{
  (event: "apply", direction: RecruitmentDirection): void;
}>();
</script>

<template>
  <section class="directions" aria-labelledby="home-directions-title">
    <h2 id="home-directions-title" class="directions__title">招新方向</h2>

    <ul class="directions__list">
      <li
        v-for="direction in directions"
        :key="direction.id"
        class="directions__item"
      >
        <div class="directions__head">
          <h3 class="directions__name">{{ direction.title }}</h3>
          <span
            class="directions__status"
            :class="
              direction.isOpen
                ? 'directions__status--open'
                : 'directions__status--closed'
            "
          >
            {{ direction.isOpen ? "开放报名" : "未开放" }}
          </span>
        </div>

        <p class="directions__summary">{{ direction.summary }}</p>

        <button
          type="button"
          class="directions__apply"
          :disabled="!direction.isOpen"
          @click="emit('apply', direction)"
        >
          {{ direction.isOpen ? "报名" : "未开放" }}
        </button>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.directions {
  padding: 18px 20px;
  border: 1px solid var(--home-border);
  border-radius: 10px;
  background: #fff;
}

.directions__title {
  margin: 0 0 14px;
  color: var(--home-ink);
  font-size: 15px;
}

.directions__list {
  display: grid;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.directions__item {
  display: grid;
  gap: 8px;
  padding: 14px 16px;
  border: 1px solid var(--home-border);
  border-radius: 8px;
}

.directions__head {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.directions__name {
  margin: 0;
  color: var(--home-ink);
  font-size: 14px;
}

.directions__status {
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 11px;
}

.directions__status--open {
  color: #1c6b45;
  background: #e9f6ef;
}

.directions__status--closed {
  color: var(--home-muted);
  background: #eef1f4;
}

.directions__summary {
  margin: 0;
  color: var(--home-muted);
  font-size: 13px;
  line-height: 1.6;
}

.directions__apply {
  justify-self: start;
  padding: 6px 18px;
  color: #fff;
  border: 1px solid var(--home-accent);
  border-radius: 6px;
  background: var(--home-accent);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.directions__apply:hover:not(:disabled) {
  background: #0f3a61;
}

.directions__apply:disabled {
  color: var(--home-muted);
  border-color: var(--home-border);
  background: #f2f4f7;
  cursor: not-allowed;
}
</style>
