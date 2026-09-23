<script setup lang="ts">
/**
 * 个人设置表单（用户名/邮箱只读；班级/学号/QQ/方向可编辑）。
 *
 * 父组件 useUserSettings 提供响应式 form 与 profile，本组件只做接线。
 *
 * 字段顺序与文案与 issue #58 对齐：
 *   - 用户名、邮箱：只读展示，写明原因（登录凭据，本期不改）
 *   - 班级、学号、QQ、方向：可编辑，全部选填
 *
 * 方向选项的中文标签与后端枚举一一对应（见 packages/shared-types 的 Direction）。
 */
import { computed } from "vue";

import type { Direction } from "@iot-pillot/shared-types";

import type { FormShape, MyProfile } from "@/api/profile";

const props = defineProps<{
  profile: MyProfile;
  form: FormShape;
  /** 全空表单时展示「建议补全」——父组件持有 needsCompletion 是单一真相 */
  needsCompletion: boolean;
  saving: boolean;
}>();

const emit = defineEmits<{
  (event: "submit"): void;
  (event: "cancel"): void;
}>();

/**
 * 方向枚举到中文标签。
 * 后端加新方向时必须同步补这里与 shared-types 的 Direction；
 * 漏一处会让用户提交非法值被 400 弹回。
 */
const DIRECTION_OPTIONS: Array<{ value: Direction; label: string }> = [
  { value: "front-end", label: "前端" },
  { value: "back-end", label: "后端" },
  { value: "agent", label: "智能体" },
  { value: "all", label: "全栈" },
  { value: "game", label: "游戏" },
  { value: "other", label: "其他" },
];

/** 后端还没把邮箱挂到 detail 里时，给一句「未提供」比留空白更诚实。 */
const emailDisplay = computed(
  () => props.profile.detail.email ?? "后端未提供（可在注册流程维护）",
);

const usernameDisplay = computed(() => props.profile.username);

function handleSubmit(event: Event): void {
  event.preventDefault();
  emit("submit");
}
</script>

<template>
  <form class="settings" novalidate @submit="handleSubmit">
    <section class="settings__readonly">
      <h2 class="settings__section-title">登录凭据（本期不可更改）</h2>
      <dl class="settings__readout">
        <div class="settings__row">
          <dt class="settings__term">用户名</dt>
          <dd class="settings__value">{{ usernameDisplay }}</dd>
        </div>
        <div class="settings__row">
          <dt class="settings__term">邮箱</dt>
          <dd class="settings__value">{{ emailDisplay }}</dd>
        </div>
      </dl>
      <p class="settings__hint">
        改名 / 改邮箱需要验证码流程，本期未做；如需修改请联系管理员。
      </p>
    </section>

    <section class="settings__editable">
      <h2 class="settings__section-title">资料字段（全部选填）</h2>

      <p
        v-if="needsCompletion"
        class="settings__completion"
        role="status"
        aria-live="polite"
      >
        建议补全你的资料 —— 但不强制，空值也能保存。
      </p>

      <label class="settings__field">
        <span class="settings__label">班级</span>
        <input
          v-model.trim="form.class"
          class="settings__input"
          type="text"
          maxlength="64"
          autocomplete="off"
          placeholder="如：物联网 2301"
        />
      </label>

      <label class="settings__field">
        <span class="settings__label">学号</span>
        <input
          v-model="form.studentId"
          class="settings__input"
          type="number"
          inputmode="numeric"
          min="0"
          step="1"
          placeholder="如：2023114514"
        />
      </label>

      <label class="settings__field">
        <span class="settings__label">QQ</span>
        <input
          v-model.trim="form.qq"
          class="settings__input"
          type="text"
          inputmode="numeric"
          maxlength="20"
          autocomplete="off"
          placeholder="如：1044696157"
        />
      </label>

      <label class="settings__field">
        <span class="settings__label">方向</span>
        <select v-model="form.direction" class="settings__select">
          <option value="">未选择</option>
          <option
            v-for="option in DIRECTION_OPTIONS"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </option>
        </select>
      </label>
    </section>

    <footer class="settings__actions">
      <button
        type="button"
        class="settings__btn settings__btn--ghost"
        :disabled="saving"
        @click="emit('cancel')"
      >
        重置
      </button>
      <button
        type="submit"
        class="settings__btn settings__btn--primary"
        :disabled="saving"
      >
        {{ saving ? "保存中…" : "保存" }}
      </button>
    </footer>
  </form>
</template>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 24px 26px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface, #fffdf7);
}

.settings__section-title {
  margin: 0 0 12px;
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 16px;
  font-weight: 600;
}

.settings__readout {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 32px;
  margin: 0 0 8px;
}

.settings__row {
  min-width: 180px;
}

.settings__term {
  margin: 0 0 4px;
  color: var(--ink-soft);
  font-size: 11px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.settings__value {
  margin: 0;
  color: var(--ink);
  font-size: 14px;
}

.settings__hint {
  margin: 8px 0 0;
  color: var(--ink-faint);
  font-size: 12px;
  line-height: 1.6;
}

.settings__completion {
  margin: 0 0 4px;
  padding: 10px 14px;
  color: #1c6b45;
  border-left: 3px solid #2e8b57;
  border-radius: 8px;
  background: #e9f6ef;
  font-size: 12px;
  line-height: 1.6;
}

.settings__editable {
  display: grid;
  gap: 14px;
}

.settings__field {
  display: grid;
  gap: 6px;
}

.settings__label {
  color: var(--ink-soft);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

.settings__input,
.settings__select {
  width: 100%;
  padding: 9px 12px;
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 6px;
  background: #fff;
  font: inherit;
  font-size: 14px;
  transition: border-color 160ms ease, box-shadow 160ms ease;
}

.settings__input:focus,
.settings__select:focus {
  outline: none;
  border-color: var(--blue);
  box-shadow: 0 0 0 3px rgba(22, 77, 128, 0.15);
}

.settings__actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding-top: 8px;
  border-top: 1px solid var(--line);
}

.settings__btn {
  padding: 9px 18px;
  border-radius: 999px;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease, border-color 160ms ease;
}

.settings__btn--primary {
  color: #fff;
  border: 1px solid var(--blue);
  background: var(--blue);
}

.settings__btn--primary:hover:not(:disabled) {
  background: #0e3a64;
}

.settings__btn--ghost {
  color: var(--ink);
  border: 1px solid var(--line);
  background: #fff;
}

.settings__btn--ghost:hover:not(:disabled) {
  border-color: var(--ink-soft);
}

.settings__btn:disabled {
  cursor: wait;
  opacity: 0.55;
}
</style>