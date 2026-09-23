<script setup lang="ts">
/**
 * 发送弹窗（issue #52）：三种模式共用 template_id + vars 的契约。
 *
 * 预览区用 renderPreview 本地渲染——和后端 RenderTemplate 同一套
 * 替换与转义规则，发送前就能看到变量填没填对；缺失的变量保留
 * 原占位符，在预览里一眼可见。
 *
 * 群发模式要求先看到收件人数量与 200 上限（验收项），超限直接禁发，
 * 不把注定 400 的请求发出去。
 */
import { computed, ref, watch } from "vue";

import { MAIL_BULK_LIMIT, type MailTemplate } from "@/api/mail";
import { useMailSend, type SendMode } from "../composables/useMailSend";
import { renderPreview } from "../utils/template-vars";

const props = defineProps<{
  visible: boolean;
  template: MailTemplate | null;
  /**
   * 可选收件人（注册用户），由父组件传入用户名单。
   * 弹窗自己不拉名单：用户与权限页已有同一份数据，重复请求没有意义。
   */
  recipients?: { user_id: number; name: string }[];
  /** 按邮箱单发时的默认地址（如从别处带入） */
  defaultEmail?: string;
}>();

const emit = defineEmits<{ "update:visible": [value: boolean] }>();

const {
  mode,
  target,
  bulkUserIds,
  vars,
  sending,
  error,
  lastSend,
  bulkResult,
  bulkCount,
  bulkOverLimit,
  canSend,
  reset,
  send,
} = useMailSend();

/** 按邮箱模式的输入独立存，提交时并进 target */
const emailInput = ref("");

const modeLabels: { value: SendMode; label: string }[] = [
  { value: "user", label: "发给注册用户" },
  { value: "email", label: "按邮箱发送" },
  { value: "bulk", label: "群发" },
];

/**
 * 收件用户的 v-model 桥：target 初值 0 表示「未选」，
 * 直接绑 0 会让下拉把数字 0 当成标签渲染出来，映射成空串才显示占位符。
 */
const selectedUserId = computed<number | string>({
  get: () => target.value.to_user_id || "",
  set: (value) => {
    target.value = { ...target.value, to_user_id: Number(value) || 0 };
  },
});

const missing = computed(() => {
  if (!props.template) return [];
  return props.template.variables.filter((name) => {
    const value = vars.value[name];
    return value === undefined || value.trim() === "";
  });
});

const previewTitle = computed(() =>
  props.template ? renderPreview(props.template.title, vars.value) : "",
);
const previewBody = computed(() =>
  props.template ? renderPreview(props.template.mail_model, vars.value) : "",
);

function ensureVarSlots(): void {
  if (!props.template) return;
  for (const name of props.template.variables) {
    if (vars.value[name] === undefined) vars.value[name] = "";
  }
}

function toggleBulkUser(userId: number): void {
  const index = bulkUserIds.value.indexOf(userId);
  if (index >= 0) bulkUserIds.value.splice(index, 1);
  else bulkUserIds.value.push(userId);
}

async function submit(): Promise<void> {
  if (!props.template) return;
  if (mode.value === "email") {
    target.value = { to_user_id: 0, email: emailInput.value };
  }
  await send(props.template.id);
}

function close(): void {
  emit("update:visible", false);
}

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    reset();
    emailInput.value = props.defaultEmail ?? "";
    ensureVarSlots();
  },
);

// 模板变量列表在打开期间不应变化，但防御性补齐无害
watch(() => props.template, ensureVarSlots);
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="template ? `发送：${template.name}` : '发送'"
    width="860px"
    :close-on-click-modal="false"
    @update:model-value="close"
  >
    <div v-if="template" class="send">
      <!-- 模式切换 -->
      <el-radio-group v-model="mode" class="send__modes">
        <el-radio-button
          v-for="option in modeLabels"
          :key="option.value"
          :value="option.value"
        >
          {{ option.label }}
        </el-radio-button>
      </el-radio-group>

      <div class="send__grid">
        <div class="send__form">
          <!-- 按用户单发 -->
          <div v-if="mode === 'user'" class="field">
            <label class="field__label">收件用户</label>
            <el-select
              v-model="selectedUserId"
              filterable
              placeholder="选择注册用户"
              style="width: 100%"
            >
              <el-option
                v-for="person in recipients ?? []"
                :key="person.user_id"
                :label="`${person.name}（#${person.user_id}）`"
                :value="person.user_id"
              />
            </el-select>
            <p class="field__hint">变量留空时，服务端会用该用户的资料自动填充。</p>
          </div>

          <!-- 按邮箱单发 -->
          <div v-else-if="mode === 'email'" class="field">
            <label class="field__label" for="send-email">收件邮箱</label>
            <el-input
              id="send-email"
              v-model="emailInput"
              placeholder="someone@example.com"
            />
            <p class="field__hint">
              用于发给未注册的报名者；对方资料无法预填，模板变量需在此处填写。
            </p>
          </div>

          <!-- 群发 -->
          <div v-else class="field">
            <label class="field__label">
              收件人（已选 {{ bulkCount }} 人 · 上限 {{ MAIL_BULK_LIMIT }} 人）
            </label>
            <div
              class="bulk-list"
              role="group"
              aria-label="群发收件人"
            >
              <label
                v-for="person in recipients ?? []"
                :key="person.user_id"
                class="bulk-list__item"
              >
                <input
                  type="checkbox"
                  :checked="bulkUserIds.includes(person.user_id)"
                  @change="toggleBulkUser(person.user_id)"
                />
                <span>{{ person.name }}</span>
                <span class="bulk-list__id">#{{ person.user_id }}</span>
              </label>
              <p
                v-if="(recipients ?? []).length === 0"
                class="field__hint"
              >
                暂无可选用户
              </p>
            </div>
            <p
              v-if="bulkOverLimit"
              class="field__error"
              role="alert"
            >
              已选 {{ bulkCount }} 人，超过后端单次群发上限 {{ MAIL_BULK_LIMIT }} 人，请去掉 {{ bulkCount - MAIL_BULK_LIMIT }} 人
            </p>
            <p v-else class="field__hint">
              同一模板逐个投递，单个失败不影响其他人；结果会分别列出成功与失败。
            </p>
          </div>

          <!-- 变量填写 -->
          <div v-if="template.variables.length > 0" class="field">
            <label class="field__label">模板变量（留空 = 服务端用收件人资料填充）</label>
            <div class="var-grid">
              <div
                v-for="name in template.variables"
                :key="name"
                class="var-grid__row"
              >
                <code class="var-grid__name">{{ name }}</code>
                <el-input v-model="vars[name]" :placeholder="`${name}`" />
              </div>
            </div>
            <!-- 缺变量是提示不是拦截：注册用户大多可被资料预填 -->
            <p v-if="missing.length > 0" class="field__hint">
              未填写：{{ missing.join("、") }}（收件人资料里也没有时，服务端会拒绝发送）
            </p>
          </div>

          <p v-if="error" class="send__error" role="alert">{{ error }}</p>
        </div>

        <!-- 本地预览 -->
        <div class="send__preview">
          <span class="send__preview-label">发送预览（本地渲染，实际以服务端为准）</span>
          <div class="preview-pane">
            <p class="preview-pane__subject">{{ previewTitle }}</p>
            <!-- 正文是管理员自己写的 HTML，预览与实发同一份内容；
                 变量值已由 renderPreview 做过 HTML 转义 -->
            <div class="preview-pane__body" v-html="previewBody" />
          </div>
        </div>
      </div>

      <!-- 群发结果 -->
      <div v-if="bulkResult" class="result">
        <p class="result__head">
          群发完成：成功 {{ bulkResult.sent.length }} / 共 {{ bulkResult.total }} 封
        </p>
        <div class="result__cols">
          <div>
            <span class="result__label result__label--ok">成功</span>
            <ul class="result__list">
              <li v-for="item in bulkResult.sent" :key="`s-${item.to_user_id}-${item.to_email}`">
                {{ item.to_email }}
              </li>
              <li v-if="bulkResult.sent.length === 0" class="result__empty">无</li>
            </ul>
          </div>
          <div>
            <span class="result__label result__label--fail">失败</span>
            <ul class="result__list">
              <li v-for="item in bulkResult.failed" :key="`f-${item.to_user_id}-${item.to_email}`">
                {{ item.to_email }} — {{ item.reason }}
              </li>
              <li v-if="bulkResult.failed.length === 0" class="result__empty">无</li>
            </ul>
          </div>
        </div>
      </div>

      <!-- 单发结果 -->
      <div v-else-if="lastSend" class="result">
        <p class="result__head">已发送至 {{ lastSend.to_email }}</p>
        <p class="result__subject">实际主题：{{ lastSend.title }}</p>
      </div>
    </div>

    <template #footer>
      <el-button @click="close">关闭</el-button>
      <el-button
        type="primary"
        :loading="sending"
        :disabled="!canSend"
        @click="submit"
      >
        {{ mode === "bulk" ? `群发给 ${bulkCount} 人` : "发送" }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.send {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.send__modes {
  align-self: flex-start;
}

.send__grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 20px;
}

.send__form {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field__label {
  color: var(--ink);
  font-size: 13px;
  font-weight: 600;
}

.field__hint {
  margin: 0;
  color: var(--ink-faint);
  font-size: 12px;
  line-height: 1.6;
}

.field__error {
  margin: 0;
  color: var(--red);
  font-size: 12px;
}

.bulk-list {
  max-height: 220px;
  overflow-y: auto;
  border: 1px solid var(--line);
  border-radius: 2px;
  background: var(--surface);
}

.bulk-list__item {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 7px 12px;
  border-bottom: 1px solid var(--line-soft);
  font-size: 13px;
  cursor: pointer;
}

.bulk-list__item:last-of-type {
  border-bottom: none;
}

.bulk-list__item:hover {
  background: var(--paper);
}

.bulk-list__id {
  margin-left: auto;
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 11px;
}

.var-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.var-grid__row {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.var-grid__name {
  overflow: hidden;
  color: var(--blue);
  font-family: var(--font-mono);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.send__error {
  margin: 0;
  padding: 8px 12px;
  border: 1px solid color-mix(in srgb, var(--red) 40%, transparent);
  background: color-mix(in srgb, var(--red) 7%, var(--surface));
  color: var(--red);
  font-size: 13px;
}

.send__preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.send__preview-label {
  color: var(--ink-faint);
  font-size: 12px;
  font-weight: 600;
}

.preview-pane {
  flex: 1;
  min-height: 300px;
  padding: 16px;
  overflow-y: auto;
  border: 1px solid var(--line);
  background: var(--surface);
}

.preview-pane__subject {
  margin: 0 0 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--line);
  color: var(--ink);
  font-size: 14px;
  font-weight: 700;
}

.preview-pane__body {
  color: var(--ink);
  font-size: 13px;
  line-height: 1.8;
  overflow-wrap: anywhere;
}

.result {
  padding: 14px 16px;
  border: 1px solid var(--line);
  background: var(--paper);
}

.result__head {
  margin: 0 0 4px;
  color: var(--ink);
  font-size: 13px;
  font-weight: 700;
}

.result__subject {
  margin: 0;
  color: var(--ink-soft);
  font-size: 13px;
}

.result__cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-top: 10px;
}

.result__label {
  font-size: 12px;
  font-weight: 700;
}

.result__label--ok {
  color: var(--green);
}

.result__label--fail {
  color: var(--red);
}

.result__list {
  margin: 6px 0 0;
  padding-left: 18px;
  color: var(--ink-soft);
  font-size: 12px;
  line-height: 1.8;
}

.result__empty {
  color: var(--ink-faint);
}

@media (max-width: 720px) {
  .send__grid {
    grid-template-columns: 1fr;
  }

  .result__cols {
    grid-template-columns: 1fr;
  }
}
</style>
