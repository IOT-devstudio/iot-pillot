<script setup lang="ts">
/**
 * Markdown 正文编辑器（#220）。
 *
 * 三件事：
 *   1. 纯文本域 + 「编辑 / 预览」双 Tab，预览复用 MarkdownContent（与详情页同一组件，
 *      杜绝「预览对、发出去不对」——ADR 0005 决策 6）。
 *   2. 插图：选图后串行上传，成功后把语法插到光标处，同时在下方显示缩略图条带。
 *   3. 单张独立状态：上传中骨架 / 失败可重试 / 可移除；一次最多 9 张。
 */
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage } from "element-plus";

import MarkdownContent from "@/components/markdown/MarkdownContent.vue";
import { uploadAttachment } from "@/api/attachment";
import {
  altFromFileName,
  blockReasonOnSubmit,
  insertSyntaxAtCursor,
  markdownSyntaxFor,
  overLimitMessage,
  reconcileWithBody,
  remainingSlots,
  removeSyntax,
  type ImageItem,
} from "../editor-model";

const props = defineProps<{
  /** 正文（v-model:body）。 */
  body: string;
  /** 占位提示。 */
  placeholder?: string;
  /** 正文长度上限（rune 计）。超限时提示并阻止提交（由父组件读 hasError）。 */
  maxLength: number;
  /** 是否禁用（提交中）。 */
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:body", value: string): void;
}>();

const tab = ref<"edit" | "preview">("edit");
const items = ref<ImageItem[]>([]);
const textarea = ref<HTMLTextAreaElement | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
/** 串行上传的序号，用于生成稳定 key。 */
let seq = 0;

const bodyLength = computed(() => [...props.body].length);
/** 是否超过上传上限（父组件据此禁用提交按钮）。 */
const overLimit = computed(() => bodyLength.value > props.maxLength);
const lengthHint = computed(
  () => `${bodyLength.value} / ${props.maxLength}`,
);
/** 提交闸门：还有失败或上传中的图时给出原因。 */
const submitBlockReason = computed(() => blockReasonOnSubmit(items.value));

/** 供父组件判断能否提交。 */
defineExpose({
  submitBlockReason,
  overLimit,
});

function setBody(next: string): void {
  emit("update:body", next);
}

/* ---------------- 图片选择与串行上传 ---------------- */

function pickFiles(): void {
  if (props.disabled) return;
  fileInput.value?.click();
}

async function onFilesChosen(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const chosen = Array.from(input.files ?? []);
  // 允许重复选同一文件：清空 value，否则第二次选同一张不会触发 change。
  input.value = "";
  if (chosen.length === 0) return;

  const warning = overLimitMessage(chosen.length, items.value);
  if (warning) ElMessage.warning(warning);

  const left = remainingSlots(items.value);
  const accepted = chosen.slice(0, left);
  if (accepted.length === 0) return;

  // 串行：一次只传一张（ADR 0004 决策 4）。用 for-of + await 而不是 Promise.all，
  // 并发会让弱网下多张同时超时，也让「失败可单独重试」的状态更难读。
  for (const file of accepted) {
    await uploadOne(file);
  }
}

async function uploadOne(file: File): Promise<void> {
  const key = `img-${++seq}`;
  const previewUrl = URL.createObjectURL(file);
  const item: ImageItem = {
    key,
    attachmentId: null,
    fileName: file.name,
    status: "uploading",
    error: "",
    previewUrl,
  };
  items.value = [...items.value, item];

  try {
    const resp = await uploadAttachment(file);
    // 用后端回的 id 重新拼语法，而不是相信本地文件名生成的结果。
    updateItem(key, {
      attachmentId: resp.id,
      status: "done",
      error: "",
      fileName: file.name,
    });
    insertSyntaxIntoBody({ ...item, attachmentId: resp.id, status: "done" });
  } catch (error) {
    updateItem(key, {
      status: "failed",
      error: error instanceof Error ? error.message : "上传失败",
    });
  }
}

function updateItem(key: string, patch: Partial<ImageItem>): void {
  items.value = items.value.map((it) =>
    it.key === key ? { ...it, ...patch } : it,
  );
}

/** 把某张图的语法插到当前光标处。 */
function insertSyntaxIntoBody(item: ImageItem): void {
  const syntax = markdownSyntaxFor(item);
  if (syntax === null) return;
  const cursor = textarea.value?.selectionStart ?? props.body.length;
  const next = insertSyntaxAtCursor(props.body, cursor, syntax);
  setBody(next.body);
  // 等 DOM 更新后把光标放回去，用户可以接着往下写。
  void nextTick(() => {
    const el = textarea.value;
    if (el) {
      el.focus();
      el.setSelectionRange(next.cursor, next.cursor);
    }
  });
}

/** 重试一张失败的图。 */
async function retryItem(item: ImageItem): Promise<void> {
  if (props.disabled) return;
  if (item.status === "uploading") return;
  // 重新上传需要原始 File；浏览器不给回读，只能让用户重选这一张。
  // 因此失败项的「重试」是重新打开文件选择器，并把这一项标记为待替换。
  pendingRetryKey.value = item.key;
  fileInput.value?.click();
}

/** 正在等待替换的失败项 key。 */
const pendingRetryKey = ref<string | null>(null);

async function onRetryChosen(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  const key = pendingRetryKey.value;
  pendingRetryKey.value = null;
  if (!file || key === null) return;

  updateItem(key, { status: "uploading", error: "" });
  try {
    const resp = await uploadAttachment(file);
    updateItem(key, {
      attachmentId: resp.id,
      status: "done",
      fileName: file.name,
      error: "",
    });
    const item = items.value.find((it) => it.key === key);
    if (item) insertSyntaxIntoBody(item);
  } catch (error) {
    updateItem(key, {
      status: "failed",
      error: error instanceof Error ? error.message : "上传失败",
    });
  }
}

/** 移除一张图：删缩略图即从正文移除对应语法（ADR 0004 决策 4）。 */
function removeItem(item: ImageItem): void {
  setBody(removeSyntax(props.body, item));
  if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
  items.value = items.value.filter((it) => it.key !== item.key);
}

/* ---------------- 正文是唯一真源 ---------------- */

// 用户在文本域里手动删掉某条语法时，缩略图条带要跟着消失，
// 否则条带与正文各说各话。只在编辑态同步，预览态没有输入。
watch(
  () => props.body,
  (next) => {
    if (items.value.length === 0) return;
    const kept = reconcileWithBody(items.value, next);
    if (kept.length === items.value.length) return;
    // 回收被移除条目的 blob URL，避免泄漏。
    for (const it of items.value) {
      if (!kept.some((k) => k.key === it.key) && it.previewUrl) {
        URL.revokeObjectURL(it.previewUrl);
      }
    }
    items.value = kept;
  },
);

onBeforeUnmount(() => {
  for (const it of items.value) {
    if (it.previewUrl) URL.revokeObjectURL(it.previewUrl);
  }
});
</script>

<template>
  <div class="md-editor">
    <div class="md-editor__tabs" role="tablist">
      <button
        type="button"
        role="tab"
        class="md-editor__tab"
        :class="{ 'md-editor__tab--active': tab === 'edit' }"
        :aria-selected="tab === 'edit'"
        @click="tab = 'edit'"
      >
        编辑
      </button>
      <button
        type="button"
        role="tab"
        class="md-editor__tab"
        :class="{ 'md-editor__tab--active': tab === 'preview' }"
        :aria-selected="tab === 'preview'"
        @click="tab = 'preview'"
      >
        预览
      </button>
      <span class="md-editor__hint">支持 Markdown</span>
    </div>

    <template v-if="tab === 'edit'">
      <textarea
        ref="textarea"
        class="md-editor__textarea"
        :value="body"
        :placeholder="placeholder ?? '写点什么…'"
        :disabled="disabled"
        @input="setBody(($event.target as HTMLTextAreaElement).value)"
      />

      <div class="md-editor__toolbar">
        <button
          type="button"
          class="md-editor__btn"
          :disabled="disabled || remainingSlots(items) === 0"
          @click="pickFiles"
        >
          插入图片
        </button>
        <span v-if="items.length" class="md-editor__count">
          已选 {{ items.length }} / 9
        </span>
      </div>

      <input
        ref="fileInput"
        type="file"
        accept="image/*"
        multiple
        hidden
        @change="pendingRetryKey ? onRetryChosen($event) : onFilesChosen($event)"
      />

      <ul v-if="items.length" class="md-strip">
        <li v-for="item in items" :key="item.key" class="md-strip__item">
          <div
            class="md-strip__thumb"
            :class="{ 'md-strip__thumb--failed': item.status === 'failed' }"
          >
            <img
              v-if="item.previewUrl && item.status !== 'failed'"
              :src="item.previewUrl"
              :alt="altFromFileName(item.fileName)"
            />
            <span v-if="item.status === 'uploading'" class="md-strip__badge">上传中</span>
            <span v-else-if="item.status === 'failed'" class="md-strip__badge md-strip__badge--fail">
              失败
            </span>
          </div>
          <div class="md-strip__actions">
            <button
              v-if="item.status === 'failed'"
              type="button"
              class="md-strip__link"
              :disabled="disabled"
              @click="retryItem(item)"
            >
              重试
            </button>
            <button
              type="button"
              class="md-strip__link md-strip__link--danger"
              :disabled="disabled"
              @click="removeItem(item)"
            >
              移除
            </button>
          </div>
          <p v-if="item.status === 'failed' && item.error" class="md-strip__error">
            {{ item.error }}
          </p>
        </li>
      </ul>
    </template>

    <div v-else class="md-editor__preview">
      <MarkdownContent v-if="body.trim()" :source="body" />
      <p v-else class="md-editor__empty">还没有内容可以预览</p>
    </div>

    <p class="md-editor__length" :class="{ 'md-editor__length--over': overLimit }">
      {{ lengthHint }}
    </p>
    <p v-if="submitBlockReason" class="md-editor__block" role="alert">
      {{ submitBlockReason }}
    </p>
  </div>
</template>

<style scoped>
.md-editor {
  border: 1px solid #e3e8ef;
  border-radius: 10px;
  background: #fff;
  overflow: hidden;
}
.md-editor__tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
  border-bottom: 1px solid #eef1f6;
  background: #fafbfd;
}
.md-editor__tab {
  border: none;
  background: transparent;
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #5b6b82;
  cursor: pointer;
}
.md-editor__tab--active {
  background: #fff;
  color: #102b4e;
  font-weight: 600;
  box-shadow: 0 1px 2px rgb(16 43 78 / 8%);
}
.md-editor__hint {
  margin-left: auto;
  font-size: 12px;
  color: #9aa7ba;
}
.md-editor__textarea {
  display: block;
  width: 100%;
  min-height: 220px;
  border: none;
  outline: none;
  resize: vertical;
  padding: 12px 14px;
  font-family: inherit;
  font-size: 14px;
  line-height: 1.7;
  color: #102b4e;
}
.md-editor__preview {
  min-height: 220px;
  padding: 12px 14px;
}
.md-editor__empty {
  color: #9aa7ba;
  font-size: 13px;
}
.md-editor__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px 10px;
}
.md-editor__btn {
  border: 1px solid #d7dee9;
  background: #fff;
  border-radius: 6px;
  padding: 4px 12px;
  font-size: 13px;
  color: #35507a;
  cursor: pointer;
}
.md-editor__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.md-editor__count {
  font-size: 12px;
  color: #8a97ab;
}
.md-strip {
  list-style: none;
  margin: 0;
  padding: 0 12px 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.md-strip__item {
  width: 96px;
}
.md-strip__thumb {
  position: relative;
  width: 96px;
  height: 72px;
  border-radius: 8px;
  overflow: hidden;
  background: #f2f5f9;
  border: 1px solid #e3e8ef;
}
.md-strip__thumb--failed {
  border-color: #f0b4ab;
  background: #fdf3f2;
}
.md-strip__thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.md-strip__badge {
  position: absolute;
  inset: auto 0 0 0;
  background: rgb(16 43 78 / 62%);
  color: #fff;
  font-size: 11px;
  text-align: center;
  padding: 2px 0;
}
.md-strip__badge--fail {
  background: #c0392b;
}
.md-strip__actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}
.md-strip__link {
  border: none;
  background: none;
  padding: 0;
  font-size: 12px;
  color: #35507a;
  cursor: pointer;
}
.md-strip__link--danger {
  color: #c0392b;
}
.md-strip__link:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.md-strip__error {
  margin: 4px 0 0;
  font-size: 11px;
  color: #c0392b;
  line-height: 1.4;
  word-break: break-all;
}
.md-editor__length {
  margin: 0;
  padding: 6px 12px;
  border-top: 1px solid #eef1f6;
  font-size: 12px;
  color: #9aa7ba;
  text-align: right;
  background: #fafbfd;
}
.md-editor__length--over {
  color: #c0392b;
  font-weight: 600;
}
.md-editor__block {
  margin: 0;
  padding: 8px 12px;
  background: #fdf3f2;
  color: #c0392b;
  font-size: 12px;
}
</style>
