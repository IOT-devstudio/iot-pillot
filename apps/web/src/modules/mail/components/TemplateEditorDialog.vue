<script setup lang="ts">
/**
 * 模板新建 / 编辑弹窗（issue #52）。
 *
 * 保存前做两层前端校验，都镜像后端规则，只为让人在点保存前就看到问题：
 *  1. 字段长度——与 request.MailTemplateReq 的 binding 标签一致
 *  2. 占位符括号成对——与后端 ValidateTemplateSyntax 一致，
 *     不成对的 {{ 是「静默发出错信」级别的错误，必须拦
 *
 * 变量列表实时从主题+正文扫描（mergeVariables，同后端 toMailModelResp），
 * 让人写模板时随时知道「这封信需要收件人有哪些资料」。
 */
import { computed, reactive, ref, watch } from "vue";

import type { MailTemplate, MailTemplateInput } from "@/api/mail";
import { mergeVariables, renderPreview, validateBraces } from "../utils/template-vars";
import MailHtmlPreview from "./MailHtmlPreview.vue";

const props = defineProps<{
  visible: boolean;
  /** 传模板 = 编辑；传 null = 新建 */
  template: MailTemplate | null;
}>();

const emit = defineEmits<{
  "update:visible": [value: boolean];
  save: [input: MailTemplateInput];
}>();

const form = reactive<MailTemplateInput>({
  name: "",
  type: "",
  title: "",
  mail_example: "",
  mail_model: "",
});

/** 提交过一次才显示字段级错误，打开就是满屏红字没有意义。 */
const submitted = ref(false);

/** 括号校验：主题与正文各查一次，错误文案带前缀区分来源。 */
const braceErrors = computed(() => {
  const errors: string[] = [];
  const titleErr = validateBraces(form.title);
  if (titleErr) errors.push(`主题：${titleErr}`);
  const bodyErr = validateBraces(form.mail_model);
  if (bodyErr) errors.push(`正文：${bodyErr}`);
  return errors;
});

const variables = computed(() => mergeVariables(form.title, form.mail_model));

/**
 * 实时 HTML 预览：编辑态没有变量值可填，传空表让占位符原样显示——
 * 写模板时要看的是排版与变量位置，不是替换结果（那在发送弹窗里看）。
 */
const previewSubject = computed(() => renderPreview(form.title, {}));
const previewBody = computed(() => renderPreview(form.mail_model, {}));

const fieldErrors = computed(() => {
  const errors: Partial<Record<keyof MailTemplateInput, string>> = {};
  if (!form.name.trim()) errors.name = "请输入模板名称";
  else if (form.name.length > 50) errors.name = "模板名称不能超过 50 字";
  if (!form.type.trim()) errors.type = "请输入模板类型";
  else if (form.type.length > 30) errors.type = "模板类型不能超过 30 字";
  if (!form.title.trim()) errors.title = "请输入邮件主题";
  else if (form.title.length > 200) errors.title = "邮件主题不能超过 200 字";
  if (!form.mail_model.trim()) errors.mail_model = "请输入邮件正文";
  else if (form.mail_model.length > 20000) {
    errors.mail_model = "邮件正文不能超过 20000 字";
  }
  if (form.mail_example.length > 5000) {
    errors.mail_example = "示例内容不能超过 5000 字";
  }
  return errors;
});

const isValid = computed(
  () =>
    Object.keys(fieldErrors.value).length === 0 && braceErrors.value.length === 0,
);

function submit(): void {
  submitted.value = true;
  if (!isValid.value) return;
  emit("save", {
    name: form.name.trim(),
    type: form.type.trim(),
    title: form.title.trim(),
    mail_example: form.mail_example,
    mail_model: form.mail_model,
  });
}

function close(): void {
  emit("update:visible", false);
}

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    submitted.value = false;
    form.name = props.template?.name ?? "";
    form.type = props.template?.type ?? "";
    form.title = props.template?.title ?? "";
    form.mail_example = props.template?.mail_example ?? "";
    form.mail_model = props.template?.mail_model ?? "";
  },
);

/** 字段错误只在提交过（或已有输入）后展示，空表单打开不报红。 */
function errOf(field: keyof MailTemplateInput): string {
  if (!submitted.value) return "";
  return fieldErrors.value[field] ?? "";
}

/** 占位符示例文本：在模板里写字面量 {{…}} 必须走插值，否则被当成模板语法。 */
const BRACE_EXAMPLE = "{{name}}";
/** placeholder 里的 {{name}} 同理——attribute 中的 mustache 也会被求值。 */
const TITLE_PLACEHOLDER = "如：{{name}}，欢迎加入 IoT 工作室招新";
const BODY_PLACEHOLDER = "Dear {{name}}，…";
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="template ? '编辑模板' : '新建模板'"
    :width="'min(720px, calc(100vw - 32px))'"
    :close-on-click-modal="false"
    @update:model-value="close"
  >
    <div class="editor">
      <div class="editor__row">
        <div class="field">
          <label class="field__label" for="tpl-name">模板名称</label>
          <el-input
            id="tpl-name"
            v-model="form.name"
            placeholder="如：面试邀请"
            maxlength="51"
            :status="errOf('name') ? 'error' : ''"
          />
          <p v-if="errOf('name')" class="field__error">{{ errOf("name") }}</p>
        </div>
        <div class="field">
          <label class="field__label" for="tpl-type">模板类型</label>
          <!-- 类型是数据库唯一键：允许自创，但撞名由服务端 400 拦（错误文案可读） -->
          <el-select
            id="tpl-type"
            v-model="form.type"
            filterable
            allow-create
            default-first-option
            placeholder="选择或输入类型"
            :status="errOf('type') ? 'error' : ''"
          >
            <el-option label="邀请函 invitation" value="invitation" />
            <el-option label="录用通知 offer" value="offer" />
            <el-option label="感谢信 rejection" value="rejection" />
            <el-option label="自定义 custom" value="custom" />
          </el-select>
          <p v-if="errOf('type')" class="field__error">{{ errOf("type") }}</p>
        </div>
      </div>

      <div class="field">
        <label class="field__label" for="tpl-title">邮件主题</label>
        <el-input
          id="tpl-title"
          v-model="form.title"
          :placeholder="TITLE_PLACEHOLDER"
          maxlength="201"
          :status="errOf('title') ? 'error' : ''"
        />
        <p v-if="errOf('title')" class="field__error">{{ errOf("title") }}</p>
      </div>

      <div class="field">
        <label class="field__label" for="tpl-body">
          邮件正文（HTML，支持 <code>{{ BRACE_EXAMPLE }}</code> 占位符）
        </label>
        <el-input
          id="tpl-body"
          v-model="form.mail_model"
          type="textarea"
          :rows="10"
          :placeholder="BODY_PLACEHOLDER"
        />
        <p v-if="errOf('mail_model')" class="field__error">
          {{ errOf("mail_model") }}
        </p>
        <MailHtmlPreview :subject="previewSubject" :html="previewBody" />
      </div>

      <div class="field">
        <label class="field__label" for="tpl-example">
          发送效果示例（选填，仅管理端预览用，不参与发送）
        </label>
        <el-input
          id="tpl-example"
          v-model="form.mail_example"
          type="textarea"
          :rows="3"
          placeholder="把变量填好后的样子贴在这里，便于其他管理员理解这封信长什么样"
        />
        <p v-if="errOf('mail_example')" class="field__error">
          {{ errOf("mail_example") }}
        </p>
      </div>

      <!-- 括号校验错误独立展示：它是「这封信会发错」级别的问题，不跟字段错误混在一起 -->
      <p
        v-for="braceError in braceErrors"
        :key="braceError"
        class="brace-error"
        role="alert"
      >
        {{ braceError }}
      </p>

      <div class="variables">
        <span class="variables__label">检测到的变量</span>
        <template v-if="variables.length > 0">
          <el-tag
            v-for="name in variables"
            :key="name"
            size="small"
            effect="plain"
          >
            {{ name }}
          </el-tag>
        </template>
        <span v-else class="variables__empty">还没有变量，正文里写 {{ BRACE_EXAMPLE }} 即可</span>
      </div>
    </div>

    <template #footer>
      <el-button @click="close">取消</el-button>
      <el-button type="primary" :disabled="!isValid" @click="submit">
        {{ template ? "保存修改" : "创建模板" }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.editor__row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
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

.field__error {
  margin: 0;
  color: var(--red);
  font-size: 12px;
}

.brace-error {
  margin: 0;
  padding: 8px 12px;
  border: 1px solid color-mix(in srgb, var(--red) 40%, transparent);
  background: color-mix(in srgb, var(--red) 7%, var(--surface));
  color: var(--red);
  font-size: 13px;
}

.variables {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  padding-top: 4px;
  border-top: 1px dashed var(--line);
}

.variables__label {
  color: var(--ink-faint);
  font-size: 12px;
  font-weight: 600;
}

.variables__empty {
  color: var(--ink-faint);
  font-size: 12px;
}

@media (max-width: 640px) {
  /* 名称/类型两列在窄屏各占半宽会把输入挤成一条缝，落成单列 */
  .editor__row {
    grid-template-columns: 1fr;
  }
}
</style>
