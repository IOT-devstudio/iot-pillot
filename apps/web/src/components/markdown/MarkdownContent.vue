<script setup lang="ts">
import { computed, type VNode } from "vue";
import { renderMarkdown } from "./renderMarkdown";

/**
 * 唯一的正文渲染组件（ADR 0005 决策 6）。
 *
 * 详情页直接用它，编辑器的「预览」Tab 也用它——两处共用同一份渲染逻辑，
 * 杜绝「预览对、发出去不对」的漂移，也让图片失败占位块在两端自动一致。
 *
 * 本组件不接收任何渲染选项（决策 10）：渲染规则的唯一真源是 renderMarkdown.ts。
 */
const props = defineProps<{
  /** Markdown 原文（数据库里存的就是这个）。 */
  source: string;
}>();

const nodes = computed<VNode[]>(() => renderMarkdown(props.source));
</script>

<template>
  <div class="md-body">
    <component :is="node" v-for="(node, i) in nodes" :key="i" />
  </div>
</template>

<style scoped>
.md-body {
  color: var(--ink, #102b4e);
  font-size: 14px;
  line-height: 1.75;
  word-break: break-word;
}

.md-body :deep(p) {
  margin: 0 0 10px;
}

.md-body :deep(h2) {
  margin: 18px 0 10px;
  font-size: 19px;
  font-weight: 700;
  line-height: 1.4;
}

.md-body :deep(h3) {
  margin: 16px 0 8px;
  font-size: 16px;
  font-weight: 700;
  line-height: 1.4;
}

.md-body :deep(h4) {
  margin: 14px 0 6px;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.4;
}

.md-body :deep(ul),
.md-body :deep(ol) {
  margin: 0 0 10px;
  padding-left: 22px;
}

.md-body :deep(li) {
  margin: 2px 0;
}

.md-body :deep(blockquote) {
  margin: 8px 0;
  padding: 4px 12px;
  border-left: 3px solid var(--line, #e3e8ef);
  color: var(--ink-faint, #5b6b86);
}

.md-body :deep(a) {
  color: var(--accent, #2f6fed);
  text-decoration: underline;
}

/* 行内代码：等宽 + 浅底，无语法高亮（决策 7）。 */
.md-body :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.92em;
  background: var(--code-bg, #f6f8fa);
  padding: 1px 4px;
  border-radius: 4px;
}

/* 围栏代码块：横向滚动、不折行、tab 两格（决策 13）。 */
.md-body :deep(pre) {
  margin: 0 0 12px;
  padding: 10px 12px;
  overflow-x: auto;
  tab-size: 2;
  background: var(--code-bg, #f6f8fa);
  border: 1px solid var(--line, #e3e8ef);
  border-radius: 6px;
}

.md-body :deep(pre code) {
  padding: 0;
  background: none;
  white-space: pre;
}

.md-body :deep(hr) {
  margin: 16px 0;
  border: none;
  border-top: 1px solid var(--line, #e3e8ef);
}

.md-body :deep(img) {
  max-width: 100%;
  border-radius: 6px;
}
</style>
