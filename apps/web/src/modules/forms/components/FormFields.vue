<script setup lang="ts">
import type { FormAnswers, FormFieldInput } from "@/api/forms";
const props = withDefaults(
  defineProps<{
    fields: FormFieldInput[];
    modelValue: FormAnswers;
    errors?: Record<string, string>;
    disabled?: boolean;
    prefix?: string;
  }>(),
  { errors: () => ({}), prefix: "answer" },
);
const emit = defineEmits<{ "update:modelValue": [value: FormAnswers] }>();
function update(field: FormFieldInput, event: Event) {
  const input = event.target as HTMLInputElement;
  const value =
    field.type === "checkbox"
      ? input.checked
        ? [field.label]
        : []
      : input.value;
  emit("update:modelValue", { ...props.modelValue, [String(field.id)]: value });
}
/**
 * 控件的可访问描述（issue #183 §4）：错误提示必须通过 aria-describedby 关联，
 * 不能只靠红色边框；邮箱题的「仅作本次答案」说明同样挂上去，读屏才不会漏。
 */
function describedBy(field: FormFieldInput): string | undefined {
  const ids = [
    field.type === "email" ? `${props.prefix}-hint-${field.id}` : "",
    props.errors[String(field.id)] ? `${props.prefix}-error-${field.id}` : "",
  ].filter(Boolean);
  return ids.length ? ids.join(" ") : undefined;
}
</script>

<template>
  <div class="form-fields">
    <div v-for="(field, index) in fields" :key="field.id" class="form-question">
      <!--
        单项确认的题目文字不能是 <label>：它内部还要再放一个包住复选框的
        <label>，嵌套 label 在 HTML 里是非法的，读屏会把两个名字拼在一起。
        所以复选框的题目用 div 呈现，真正的名字交给内层 label。
      -->
      <component
        :is="field.type === 'checkbox' ? 'div' : 'label'"
        :id="`${prefix}-label-${field.id}`"
        :for="field.type === 'checkbox' ? undefined : `${prefix}-${field.id}`"
        class="form-question__label"
      >
        <span class="form-question__number">{{
          String(index + 1).padStart(2, "0")
        }}</span>
        <span
          >{{ field.label || "未命名题目" }}
          <small>{{ field.required ? "必填" : "选填" }}</small></span
        >
      </component>
      <p
        v-if="field.type === 'email'"
        :id="`${prefix}-hint-${field.id}`"
        class="form-hint"
      >
        仅作本次申请答案，不会修改账号绑定邮箱。
      </p>
      <label v-if="field.type === 'checkbox'" class="form-check">
        <input
          :id="`${prefix}-${field.id}`"
          type="checkbox"
          :checked="
            Array.isArray(modelValue[String(field.id)]) &&
            modelValue[String(field.id)]!.length > 0
          "
          :disabled="disabled"
          :aria-required="field.required"
          :aria-invalid="!!errors[String(field.id)]"
          :aria-describedby="describedBy(field)"
          @change="update(field, $event)"
        />
        <span>{{ field.placeholder || "我已阅读并确认以上内容" }}</span>
      </label>
      <component
        :is="
          field.type === 'textarea'
            ? 'textarea'
            : field.type === 'select'
              ? 'select'
              : 'input'
        "
        v-else
        :id="`${prefix}-${field.id}`"
        class="form-control"
        :type="field.type === 'email' ? 'email' : 'text'"
        :rows="field.type === 'textarea' ? 5 : undefined"
        :value="modelValue[String(field.id)] ?? ''"
        :placeholder="field.placeholder"
        :disabled="disabled"
        :maxlength="2000"
        :aria-required="field.required"
        :aria-invalid="!!errors[String(field.id)]"
        :aria-describedby="describedBy(field)"
        @input="update(field, $event)"
      >
        <template v-if="field.type === 'select'">
          <option value="">{{ field.placeholder || "请选择" }}</option>
          <option v-for="option in field.options" :key="option" :value="option">
            {{ option }}
          </option>
        </template>
      </component>
      <p
        v-if="errors[String(field.id)]"
        :id="`${prefix}-error-${field.id}`"
        class="form-error"
      >
        {{ errors[String(field.id)] }}
      </p>
    </div>
  </div>
</template>
