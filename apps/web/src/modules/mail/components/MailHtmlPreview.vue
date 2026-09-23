<script setup lang="ts">
/**
 * 邮件 HTML 实时预览（issue #52 扩展）。
 *
 * 用带 sandbox 的 iframe srcdoc 渲染而不是 div v-html：
 * 粘贴来的邮件模板常带 <style>（会全局污染管理端页面）和 <script>，
 * sandbox 空权限串两者都不执行——预览环境因此比 v-html 更接近
 * 「只看排版」的意图，也顺带堵住脚本注入。
 *
 * 「放大预览」打开同内容的大弹窗：内嵌框只有 200px 高，
 * 长邮件只能看个头，完整排版要在大画布里才看得全。
 *
 * 主题行放在 iframe 外，和 SendMailDialog 的预览面板同一套视觉。
 */
import { ref } from "vue";

defineProps<{
  /** 渲染后的主题（调用方先做变量替换） */
  subject: string;
  /** 渲染后的正文 HTML */
  html: string;
}>();

/** 放大弹窗开关（组件内自治，不打扰调用方） */
const enlarged = ref(false);

/** 邮件正文的默认观感：邮件客户端大多从这个基准起步 */
const DOC_PREFIX =
  '<!doctype html><html><head><meta charset="utf-8"></head>' +
  '<body style="margin:0;font:14px/1.7 -apple-system,Segoe UI,PingFang SC,sans-serif;color:#102b4e;">';
const DOC_SUFFIX = "</body></html>";

function srcdocOf(html: string): string {
  return DOC_PREFIX + html + DOC_SUFFIX;
}
</script>

<template>
  <div class="mail-preview">
    <div class="mail-preview__head">
      <span class="mail-preview__label">HTML 预览（变量显示为占位符）</span>
    </div>
    <div class="mail-preview__frame">
      <p class="mail-preview__subject">{{ subject || "（未填主题）" }}</p>
      <!-- 放大键压在预览框右上角（主题行右端）：图标按钮的惯常位置，
           和预览内容同框，不用在行外另找操作点。
           无框裸图标：悬停才给浅填充反馈，静置不带任何边界 -->
      <el-button
        class="mail-preview__zoom-btn"
        aria-label="放大预览"
        title="放大预览"
        @click="enlarged = true"
      >
        <!-- 经典 expand 双箭头（左上 ↖ + 右下 ↘）：
             短臂 + 长杆 + 细线宽是「修长」的关键，比例同 Feather maximize-2 -->
        <svg
          class="mail-preview__zoom-icon"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path d="M9 3H3v6" />
          <path d="M3 3l7 7" />
          <path d="M15 21h6v-6" />
          <path d="M21 21l-7-7" />
        </svg>
      </el-button>
      <!-- sandbox=""：无脚本、无表单、无同源——模板里的 <style>/<script> 都出不来 -->
      <iframe
        class="mail-preview__body"
        title="邮件 HTML 预览"
        sandbox=""
        :srcdoc="srcdocOf(html)"
      />
    </div>

    <!-- 放大版：同一份内容，大画布看完整排版。
         append-to-body 必须开：嵌在外层编辑弹窗的 DOM 里时，
         两层 overlay 的层叠合成会让内层弹窗几何存在却画不出来 -->
    <el-dialog
      v-model="enlarged"
      title="邮件预览"
      width="min(920px, 94vw)"
      class="mail-preview__zoom"
      append-to-body
    >
      <p class="mail-preview__subject">{{ subject || "（未填主题）" }}</p>
      <iframe
        class="mail-preview__zoom-body"
        title="放大的邮件 HTML 预览"
        sandbox=""
        :srcdoc="srcdocOf(html)"
      />
    </el-dialog>
  </div>
</template>

<style scoped>
.mail-preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.mail-preview__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.mail-preview__label {
  color: var(--ink-faint);
  font-size: 12px;
  font-weight: 600;
}

/* 纯图标按钮：无边框、无背景，静置时只有箭头本身；
   悬停给一层浅填充作为可点反馈（状态齐全但不画永久方框）。
   绝对定位压在预览框主题行右端——图标按钮的惯常落点 */
.mail-preview__frame {
  position: relative;
  border: 1px solid var(--line);
  background: var(--surface);
}

.mail-preview__zoom-btn {
  position: absolute;
  top: 2px;
  right: 4px;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 6px;
  border: none;
  background: transparent;
}

.mail-preview__zoom-btn:hover {
  background: var(--paper);
}

.mail-preview__zoom-icon {
  display: block;
  width: 22px;
  height: 22px;
  fill: none;
  stroke: currentColor;
  /* 1.5 @24 viewBox：比 Feather 默认的 2 更细一档，杆长臂短才显得修长 */
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.mail-preview__subject {
  margin: 0;
  /* 右侧留出给放大键的位置，主题长也不至于钻到按钮底下 */
  padding: 10px 56px 10px 14px;
  border-bottom: 1px solid var(--line);
  color: var(--ink);
  font-size: 13px;
  font-weight: 700;
}

.mail-preview__body {
  display: block;
  width: 100%;
  height: 200px;
  border: none;
  background: #fff;
}

/* 放大弹窗 append-to-body 后挂在 body 下，scoped 属性仍随模板编译带上，
   这里的选择器能命中；:deep 只用于弹窗内部的 Element 结构 */
.mail-preview__zoom :deep(.el-dialog__body) {
  padding-top: 4px;
}

.mail-preview__zoom-body {
  display: block;
  width: 100%;
  height: min(65vh, 640px);
  border: 1px solid var(--line);
  background: #fff;
}
</style>
