<script setup lang="ts">
/**
 * 个人设置表单（姓名及资料可编辑，绑定邮箱只读，issue #87）。
 *
 * 父组件 useUserSettings 提供响应式 form 与 profile，本组件只做接线。
 *
 * 姓名非空时为 3–20 个 Unicode 字符，留空发送 null 清除。
 * 班级、学号必填；默认只读，方向取最新报名，无报名时保留资料方向（#101）。
 *
 * 方向选项的中文标签与后端枚举一一对应（见 packages/shared-types 的 Direction）。
 */
import { computed } from "vue";

import type { Direction } from "@iot-pillot/shared-types";

import type { FormShape, MyProfile } from "@/api/profile";
import type { RequiredProfileErrors } from "../profileCompletion";

const props = defineProps<{
  profile: MyProfile;
  form: FormShape;
  /** 缺少班级或学号时提示补全。 */
  needsCompletion: boolean;
  saving: boolean;
  nameError: string;
  editing: boolean;
  fieldErrors: RequiredProfileErrors;
  latestDirection: Direction | "";
  directionError: string;
}>();

const emit = defineEmits<{
  (event: "submit"): void;
  (event: "cancel"): void;
  (event: "edit"): void;
  (event: "reset"): void;
  (event: "retry"): void;
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
  () => props.profile.detail?.email || "未提供",
);
const directionLabel = (code: string) => DIRECTION_OPTIONS.find((item) => item.value === code)?.label || code || "未选择";

function handleSubmit(event: Event): void {
  event.preventDefault();
  emit("submit");
}
</script>

<template>
  <form class="settings liquid-glass" novalidate @submit="handleSubmit">
    <section class="settings__readonly">
      <h2 class="settings__section-title">账号邮箱</h2>
      <dl class="settings__readout">
        <div class="settings__row">
          <dt class="settings__term">邮箱</dt>
          <dd class="settings__value">{{ emailDisplay }}</dd>
        </div>
      </dl>
      <p class="settings__hint">
        绑定邮箱不可修改。修改姓名不影响邮箱登录，也不会退出当前会话；建议使用邮箱登录。
      </p>
    </section>

    <section class="settings__editable" :aria-busy="saving">
      <h2 class="settings__section-title">个人资料</h2>

      <dl v-if="!editing" class="settings__summary">
        <div><dt>姓名</dt><dd>{{ profile.name || '未填写' }}</dd></div>
        <div><dt>班级 · 必填</dt><dd>{{ profile.detail?.class || '未填写' }}</dd></div>
        <div><dt>学号 · 必填</dt><dd>{{ profile.detail?.student_id || '未填写' }}</dd></div>
        <div><dt>QQ · 选填</dt><dd>{{ profile.detail?.qq || '未填写' }}</dd></div>
      </dl>

      <label v-if="editing" class="settings__field">
        <span id="settings-name-label" class="settings__label">姓名</span>
        <input
          v-model="form.name"
          class="settings__input"
          type="text"
          autocomplete="name"
          aria-labelledby="settings-name-label"
          :disabled="saving"
          :aria-invalid="Boolean(nameError)"
          aria-describedby="settings-name-hint settings-name-error"
        />
        <span id="settings-name-hint" class="settings__hint">3–20 个字符；留空并保存可清除姓名。</span>
        <span id="settings-name-error" class="settings__field-error" role="alert">{{ nameError }}</span>
      </label>

      <p
        v-if="needsCompletion"
        class="settings__completion"
        role="status"
        aria-live="polite"
      >
        请补全班级和学号；QQ 为选填。完善后将不再显示登录补全提醒。
      </p>

      <label v-if="editing" class="settings__field">
        <span class="settings__label">班级 · 必填</span>
        <input
          v-model.trim="form.class"
          class="settings__input"
          type="text"
          maxlength="64"
          autocomplete="off"
          placeholder="如：物联网 2301"
          :disabled="saving"
          required
          :aria-invalid="Boolean(fieldErrors.class)"
          aria-describedby="settings-class-error"
        />
        <span id="settings-class-error" class="settings__field-error" role="alert">{{ fieldErrors.class }}</span>
      </label>

      <label v-if="editing" class="settings__field">
        <span class="settings__label">学号 · 必填</span>
        <input
          v-model="form.studentId"
          class="settings__input"
          type="text"
          inputmode="numeric"
          placeholder="如：2023114514"
          :disabled="saving"
          required
          :aria-invalid="Boolean(fieldErrors.studentId)"
          aria-describedby="settings-student-error settings-student-hint"
        />
        <span id="settings-student-hint" class="settings__hint">填写完整的整数学号；0 表示未填写。</span>
        <span id="settings-student-error" class="settings__field-error" role="alert">{{ fieldErrors.studentId }}</span>
      </label>

      <label v-if="editing" class="settings__field">
        <span class="settings__label">QQ · 选填</span>
        <input
          v-model.trim="form.qq"
          class="settings__input"
          type="text"
          inputmode="numeric"
          maxlength="20"
          autocomplete="off"
          placeholder="如：1044696157"
          :disabled="saving"
        />
      </label>

      <div class="settings__field">
        <span class="settings__label">方向</span>
        <span class="settings__value">{{ directionLabel(latestDirection || form.direction) }}</span>
        <span class="settings__hint">{{ latestDirection ? '根据最新一条报名记录自动带出，无需重复填写。' : '暂无报名记录，保留当前资料方向；请到首页选择方向并完成报名。' }}</span>
        <span v-if="latestDirection && profile.detail?.direction && profile.detail.direction !== latestDirection" class="settings__hint">当前资料方向：{{ directionLabel(profile.detail.direction) }}</span>
        <p v-if="directionError" class="settings__field-error" role="alert">{{ directionError }}</p>
        <button v-if="directionError" type="button" class="settings__btn settings__btn--ghost" :disabled="saving" @click="emit('retry')">重新核对方向</button>
      </div>
    </section>

    <footer class="settings__actions">
      <button v-if="!editing" type="button" class="settings__btn settings__btn--primary" @click="emit('edit')">更改</button>
      <template v-else>
      <button
        type="button"
        class="settings__btn settings__btn--ghost"
        :disabled="saving"
        @click="emit('reset')"
      >
        重置
      </button>
      <button type="button" class="settings__btn settings__btn--ghost" :disabled="saving" @click="emit('cancel')">取消</button>
      <button
        type="submit"
        class="settings__btn settings__btn--primary"
        :disabled="saving"
      >
        {{ saving ? "保存中…" : "保存" }}
      </button>
      </template>
    </footer>
  </form>
</template>

<style scoped>
/* 卡片玻璃质感来自全局 .liquid-glass（issue #85 改造）；
   输入控件保持实底，保证表单可读性 */
.settings {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 24px 26px;
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
  overflow-wrap: anywhere;
}

.settings__field-error {
  color: #a33d32;
  font-size: 12px;
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
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.settings__input:focus,
.settings__select:focus {
  outline: none;
  border-color: var(--blue);
  box-shadow: 0 0 0 3px rgba(22, 77, 128, 0.15);
}

.settings__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: flex-end;
  padding-top: 8px;
  border-top: 1px solid var(--line);
}

.settings__summary { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; margin: 0; }
.settings__summary dt { color: var(--ink-soft); font-size: 12px; }
.settings__summary dd { margin: 6px 0 0; overflow-wrap: anywhere; font-size: 14px; }
.settings__input { box-sizing: border-box; }
@media (max-width: 400px) {
  .settings { padding: 20px 16px; }
  .settings__summary { grid-template-columns: 1fr; }
  .settings__actions { gap: 8px; }
  .settings__btn { padding: 9px 14px; }
}

.settings__btn {
  padding: 9px 18px;
  border-radius: 999px;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 160ms ease,
    color 160ms ease,
    border-color 160ms ease;
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
