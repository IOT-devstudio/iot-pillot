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
</script>

<template>
  <div class="form-fields">
    <div v-for="(field, index) in fields" :key="field.id" class="form-question">
      <label :for="`${prefix}-${field.id}`" class="form-question__label">
        <span class="form-question__number">{{
          String(index + 1).padStart(2, "0")
        }}</span>
        <span
          >{{ field.label || "未命名题目" }}
          <small>{{ field.required ? "必填" : "选填" }}</small></span
        >
      </label>
      <p v-if="field.type === 'email'" class="form-hint">
        仅作本次报名答案，不会修改账号绑定邮箱。
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
          :aria-describedby="
            errors[String(field.id)] ? `${prefix}-error-${field.id}` : undefined
          "
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
        :aria-describedby="
          errors[String(field.id)] ? `${prefix}-error-${field.id}` : undefined
        "
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
