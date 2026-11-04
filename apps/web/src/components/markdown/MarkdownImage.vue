<script setup lang="ts">
import { computed, ref } from "vue";
import { isAllowedUrl } from "./protocolAllowlist";

/**
 * 正文图片（ADR 0005 决策 3 / 12）。
 *
 * 所有 <img> 统一加两个属性：
 *   - loading="lazy"：一屏十几张大图不会同时拉；
 *   - referrerpolicy="no-referrer"：外链图片拿不到当前（敏感）页面地址。
 *
 * 加载失败 → 占位块（ADR 0004 要求的「图片加载失败占位块」），
 * 详情页与编辑器预览 Tab 通过 MarkdownContent 自动共享这一行为。
 * 空 alt 视为装饰性图片（决策 12）：占位块只显示「图片不可用」，不带补充说明。
 */
const props = defineProps<{
  /** 图片地址：站内 /api/v1/attachments/... 或外链 http(s)。 */
  src: string;
  /** 替代文字；空表示装饰性图片。 */
  alt: string;
}>();

const failed = ref(false);

// 纵深防护：协议不在白名单（javascript: / data: 等）时也不渲染 <img>，直接走占位块。
const showImage = computed(() => !failed.value && isAllowedUrl(props.src));

const fallbackLabel = computed(() =>
  props.alt ? `图片不可用：${props.alt}` : "图片不可用",
);
</script>

<template>
  <img
    v-if="showImage"
    :src="src"
    :alt="alt"
    loading="lazy"
    referrerpolicy="no-referrer"
    @error="failed = true"
  />
  <span
    v-else
    class="md-img-fallback"
    role="img"
    :aria-label="fallbackLabel"
    >{{ fallbackLabel }}</span
  >
</template>

<style scoped>
.md-img-fallback {
  display: inline-block;
  padding: 4px 10px;
  border: 1px dashed var(--line, #e3e8ef);
  border-radius: 4px;
  color: var(--ink-faint, #5b6b86);
  font-size: 13px;
  line-height: 1.6;
}
</style>
