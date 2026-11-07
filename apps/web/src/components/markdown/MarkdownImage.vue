<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { isAllowedUrl } from "./protocolAllowlist";
import {
  isAttachmentUrl,
  loadAttachmentObjectUrl,
  parseAttachmentId,
} from "@/api/attachment";

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
 *
 * 站内附件（/api/v1/attachments/{id}）**不能直接当 src 用**：该端点挂了
 * authRequired，而浏览器加载 <img> 不会带 Authorization 头，直接写必然 401、
 * 退化成本该只在真失败时出现的占位块。因此站内附件走带 token 的 fetch 取回
 * 二进制，再转成 blob URL；外链仍直接交给 <img>（它们本来就不需要我们的凭据）。
 */
const props = defineProps<{
  /** 图片地址：站内 /api/v1/attachments/... 或外链 http(s)。 */
  src: string;
  /** 替代文字；空表示装饰性图片。 */
  alt: string;
}>();

const failed = ref(false);
/** 站内附件的实际 src（blob URL）；外链直接落原始地址。 */
const resolvedSrc = ref<string>(isAttachmentUrl(props.src) ? "" : props.src);
/** 站内附件是否还在取数（用于骨架，不闪「不可用」）。 */
const loadingAttachment = ref(isAttachmentUrl(props.src));

let objectUrl: string | null = null;

function revoke(): void {
  if (objectUrl !== null) {
    URL.revokeObjectURL(objectUrl);
    objectUrl = null;
  }
}

async function loadAttachment(src: string): Promise<void> {
  revoke();
  failed.value = false;
  loadingAttachment.value = true;
  resolvedSrc.value = "";

  const id = parseAttachmentId(src);
  if (id === null) {
    failed.value = true;
    loadingAttachment.value = false;
    return;
  }
  try {
    const url = await loadAttachmentObjectUrl(id);
    objectUrl = url;
    resolvedSrc.value = url;
  } catch {
    // 401 / 404 / 网络错误一律落占位块：对读者而言「看不到这张图」是同一种结果。
    failed.value = true;
  } finally {
    loadingAttachment.value = false;
  }
}

watch(
  () => props.src,
  (next) => {
    if (isAttachmentUrl(next)) {
      void loadAttachment(next);
    } else {
      revoke();
      failed.value = false;
      loadingAttachment.value = false;
      resolvedSrc.value = next;
    }
  },
  { immediate: true },
);

// blob URL 是组件私有的，卸载必须回收，否则整页浏览下来会一直占内存。
onBeforeUnmount(revoke);

/** 我们自己 fetch 回来生成的对象地址（局部规则，不在正文协议白名单里）。 */
function isOwnObjectUrl(url: string): boolean {
  return url.startsWith("blob:");
}

/**
 * 是否真的渲染出 <img>。
 *
 * 两条判定，故意分开写：
 *   1. **自己生成的 blob URL** 单独放行。它不是正文里写的东西，而是组件为了绕过
 *      「<img> 带不上 Authorization 头」生成的中间产物，所以不能拿正文协议白名单
 *      去量它——`new URL("blob:http://…").protocol` 是 `blob:`，必然被拒；
 *      把 `blob:` 加进 isAllowedUrl 又会顺带让 `![](blob:…)` 变成合法正文语法，
 *      而 ADR 0005 决策 5 正是要排除这种东西。
 *   2. 其余地址（外链 / 站内相对路径）仍按**原始 src** 走 isAllowedUrl 做纵深防护
 *      （javascript: / data: 等一律落占位块）。
 */
const showImage = computed(() => {
  if (failed.value) return false;
  const src = resolvedSrc.value;
  if (src === "") return false;
  if (isOwnObjectUrl(src)) return isAttachmentUrl(props.src);
  return isAllowedUrl(props.src);
});

const fallbackLabel = computed(() =>
  props.alt ? `图片不可用：${props.alt}` : "图片不可用",
);
</script>

<template>
  <span v-if="loadingAttachment" class="md-img-skeleton" role="img" aria-label="图片加载中" />
  <img
    v-else-if="showImage"
    :src="resolvedSrc"
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
.md-img-skeleton {
  display: inline-block;
  min-width: 120px;
  min-height: 72px;
  border-radius: 6px;
  background: linear-gradient(90deg, #eef1f6 25%, #f6f8fb 37%, #eef1f6 63%);
  background-size: 400% 100%;
  animation: md-img-shimmer 1.4s ease infinite;
}
@keyframes md-img-shimmer {
  0% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0 50%;
  }
}
</style>
