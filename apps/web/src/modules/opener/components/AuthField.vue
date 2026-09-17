<script setup lang="ts">
/**
 * 单个表单字段：label + input + 错误提示，右侧可选操作区（用 #action 插槽，
 * 例如注册页「获取验证码」按钮）。
 *
 * 用 v-model 双向绑定值。样式通过 var(--opener-*) 令牌继承自 StudioOpener
 * 根元素，因此这一个组件就能覆盖登录与注册的全部字段。
 */
defineProps<{
  /** input 的 id，同时作为 label 的 for 目标与错误提示的关联 id 前缀 */
  id: string;
  label: string;
  /** 提交给后端的字段名；省略时与 id 相同 */
  name?: string;
  type?: string;
  autocomplete?: string;
  placeholder?: string;
  inputmode?: "none" | "text" | "numeric" | "email";
  maxlength?: number;
  error?: string;
}>();

const model = defineModel<string>({ required: true });
</script>

<template>
  <div class="auth-field">
    <label :for="id">{{ label }}</label>

    <div
      class="auth-field__control"
      :class="{ 'auth-field__control--with-action': Boolean($slots.action) }"
    >
      <input
        :id="id"
        v-model="model"
        :name="name ?? id"
        :type="type ?? 'text'"
        :autocomplete="autocomplete"
        :placeholder="placeholder"
        :inputmode="inputmode"
        :maxlength="maxlength"
        :aria-invalid="error ? 'true' : undefined"
        :aria-describedby="error ? `${id}-error` : undefined"
      />
      <slot name="action" />
    </div>

    <p v-if="error" :id="`${id}-error`" class="auth-field__error">
      {{ error }}
    </p>
  </div>
</template>

<style scoped>
.auth-field {
  min-width: 0;
}

.auth-field label {
  display: block;
  margin-bottom: 6px;
  color: var(--opener-ink);
  font-size: 12px;
  font-weight: 700;
}

.auth-field__control {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 8px;
}

/* 有 #action 插槽（验证码按钮）时把输入框与按钮并排 */
.auth-field__control--with-action {
  grid-template-columns: minmax(0, 1fr) auto;
}

.auth-field input {
  width: 100%;
  height: 42px;
  padding: 0 12px;
  color: var(--opener-ink);
  border: 1px solid var(--opener-border);
  border-radius: 6px;
  outline: 0;
  background: var(--opener-input-bg);
  font: inherit;
  font-size: 14px;
  transition: border-color 160ms ease, box-shadow 160ms ease;
}

.auth-field input::placeholder {
  color: var(--opener-muted);
}

.auth-field input:focus-visible {
  border-color: var(--opener-blue-bright);
  outline: 0;
  box-shadow: 0 0 0 3px var(--opener-focus-ring);
}

.auth-field input[aria-invalid="true"] {
  border-color: var(--opener-danger);
}

.auth-field__error {
  margin: 6px 0 0;
  color: var(--opener-danger);
  font-size: 12px;
  line-height: 1.4;
}

@media (max-width: 480px) {
  /* 手机上「输入框 + 验证码按钮」并排会太挤，改为上下排列 */
  .auth-field__control--with-action {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
