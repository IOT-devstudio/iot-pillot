<script setup lang="ts">
/**
 * 个人资料表单（issue #149：字段级直接编辑 + 自动保存）。
 *
 * - 姓名/班级/学号/QQ 常驻 input，失焦后自动保存（父组件 runFieldSave）。
 * - 每个字段下方显示状态：保存中… / 已保存 / 失败（保留输入 + 重试）。
 * - 邮箱继续只读；普通成员方向只读（最新报名带出），管理员方向下拉失焦保存。
 * - 校验错误直接显示在字段下；输入即清掉该字段的旧错误。
 */
import { computed } from "vue";

import type { Direction } from "@iot-pillot/shared-types";

import type { FormShape, MyProfile } from "@/api/profile";
import type { FieldErrors, FieldKey, FieldStatus } from "../composables/useUserSettings";

const props = defineProps<{
  profile: MyProfile;
  form: FormShape;
  needsCompletion: boolean;
  fieldStatuses: Record<FieldKey, FieldStatus>;
  fieldErrors: FieldErrors;
  latestDirection: Direction | "";
  directionError: string;
}>();

const emit = defineEmits<{
  (event: "save-field", key: FieldKey): void;
  (event: "retry-field", key: FieldKey): void;
  (event: "retry-direction"): void;
  (event: "save-direction"): void;
  /** 成员方向未选择时点「去选择」——由父组件跳首页方向版块走真实报名 */
  (event: "pick-direction"): void;
}>();

const isAdmin = computed(() => props.profile.role === "admin");

const directionUnset = computed(
  () => !(props.latestDirection || props.form.direction),
);

const DIRECTION_OPTIONS: Array<{ value: Direction; label: string }> = [
  { value: "front-end", label: "前端" },
  { value: "back-end", label: "后端" },
  { value: "agent", label: "智能体" },
  { value: "all", label: "全栈" },
  { value: "game", label: "游戏" },
  { value: "other", label: "其他" },
];

const emailDisplay = computed(
  () => props.profile.detail?.email || "未提供",
);
const directionLabel = (code: string) => DIRECTION_OPTIONS.find((item) => item.value === code)?.label || code || "未选择";

function onBlur(key: FieldKey): void {
  emit("save-field", key);
}

function onInput(key: FieldKey): void {
  // 用户正在改：状态从 saved/error 回到 idle，错误文案由父组件在校验通过后清除
  if (props.fieldStatuses[key] === "saved" || props.fieldStatuses[key] === "error") {
    props.fieldStatuses[key] = "idle";
  }
}
</script>

<template>
  <form class="settings liquid-glass" novalidate>
    <section class="settings__readonly">
      <h2 class="settings__section-title">账号邮箱</h2>
      <dl class="settings__readout">
        <div class="settings__row">
          <dt class="settings__term">邮箱</dt>
          <dd class="settings__value">{{ emailDisplay }}</dd>
        </div>
      </dl>
      <p class="settings__hint">
        绑定邮箱不可修改。修改姓名不影响邮箱登录，也不会退出当前会话。
      </p>
    </section>

    <section class="settings__editable">
      <h2 class="settings__section-title">个人资料</h2>
      <p class="settings__hint settings__hint--top">直接编辑字段，失焦后自动保存。</p>

      <p
        v-if="needsCompletion"
        class="settings__completion"
        role="status"
        aria-live="polite"
      >
        请补全或修正班级和学号；QQ 为选填。注册满 7 天资料仍不合规的账号会被定时冻结。
      </p>

      <!-- 姓名 -->
      <label class="settings__field">
        <span id="settings-name-label" class="settings__label">姓名</span>
        <input
          v-model="form.name"
          class="settings__input"
          type="text"
          autocomplete="name"
          aria-labelledby="settings-name-label"
          :aria-invalid="Boolean(fieldErrors.name)"
          aria-describedby="settings-name-hint settings-name-status"
          @input="onInput('name')"
          @blur="onBlur('name')"
        />
        <span id="settings-name-hint" class="settings__hint">姓名须为 2–20 个中文汉字。</span>
        <span id="settings-name-status" class="settings__status" role="status" :data-status="fieldStatuses.name">
          <template v-if="fieldStatuses.name === 'saving'">保存中…</template>
          <template v-else-if="fieldStatuses.name === 'saved'">已保存</template>
          <template v-else-if="fieldStatuses.name === 'error' && fieldErrors.name">
            {{ fieldErrors.name }}
            <button type="button" class="settings__retry" @click="emit('retry-field', 'name')">重试</button>
          </template>
        </span>
      </label>

      <!-- 班级 -->
      <label class="settings__field">
        <span class="settings__label">班级 · 必填</span>
        <input
          v-model.trim="form.class"
          class="settings__input"
          type="text"
          maxlength="64"
          autocomplete="off"
          placeholder="如：软件工程230301"
          required
          :aria-invalid="Boolean(fieldErrors.class)"
          aria-describedby="settings-class-hint"
          @input="onInput('class')"
          @blur="onBlur('class')"
        />
        <span id="settings-class-hint" class="settings__hint">
          汉字 + 6 位数字，例如 软件工程230301。
        </span>
        <span class="settings__status" role="status" :data-status="fieldStatuses.class">
          <template v-if="fieldStatuses.class === 'saving'">保存中…</template>
          <template v-else-if="fieldStatuses.class === 'saved'">已保存</template>
          <template v-else-if="fieldStatuses.class === 'error' && fieldErrors.class">
            {{ fieldErrors.class }}
            <button type="button" class="settings__retry" @click="emit('retry-field', 'class')">重试</button>
          </template>
        </span>
      </label>

      <!-- 学号 -->
      <label class="settings__field">
        <span class="settings__label">学号 · 必填</span>
        <input
          v-model="form.studentId"
          class="settings__input"
          type="text"
          inputmode="numeric"
          placeholder="如：202311451"
          required
          :aria-invalid="Boolean(fieldErrors.studentId)"
          aria-describedby="settings-student-hint"
          @input="onInput('studentId')"
          @blur="onBlur('studentId')"
        />
        <span id="settings-student-hint" class="settings__hint">9 位数字，例如 202311451。</span>
        <span class="settings__status" role="status" :data-status="fieldStatuses.studentId">
          <template v-if="fieldStatuses.studentId === 'saving'">保存中…</template>
          <template v-else-if="fieldStatuses.studentId === 'saved'">已保存</template>
          <template v-else-if="fieldStatuses.studentId === 'error' && fieldErrors.studentId">
            {{ fieldErrors.studentId }}
            <button type="button" class="settings__retry" @click="emit('retry-field', 'studentId')">重试</button>
          </template>
        </span>
      </label>

      <!-- QQ -->
      <label class="settings__field">
        <span class="settings__label">QQ · 选填</span>
        <input
          v-model.trim="form.qq"
          class="settings__input"
          type="text"
          inputmode="numeric"
          maxlength="20"
          autocomplete="off"
          placeholder="如：1044696157"
          @input="onInput('qq')"
          @blur="onBlur('qq')"
        />
        <span class="settings__status" role="status" :data-status="fieldStatuses.qq">
          <template v-if="fieldStatuses.qq === 'saving'">保存中…</template>
          <template v-else-if="fieldStatuses.qq === 'saved'">已保存</template>
          <template v-else-if="fieldStatuses.qq === 'error' && fieldErrors.qq">
            {{ fieldErrors.qq }}
            <button type="button" class="settings__retry" @click="emit('retry-field', 'qq')">重试</button>
          </template>
        </span>
      </label>

      <!-- 管理员：下拉选择方向 -->
      <label v-if="isAdmin" class="settings__field">
        <span class="settings__label">方向</span>
        <select v-model="form.direction" class="settings__select" @change="emit('save-direction')">
          <option value="">未选择</option>
          <option v-for="option in DIRECTION_OPTIONS" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <span class="settings__hint">管理员不参与报名，方向在此直接选择并保存。</span>
      </label>

      <!-- 普通成员：方向只读 -->
      <div v-else class="settings__field">
        <span class="settings__label">方向</span>
        <span class="settings__value">
          {{ directionLabel(latestDirection || form.direction) }}
          <button
            v-if="!directionError && directionUnset"
            type="button"
            class="settings__pick-link"
            aria-label="去首页选择方向"
            @click="emit('pick-direction')"
          >
            去选择
          </button>
        </span>
        <span class="settings__hint">{{ latestDirection ? '根据最新一条报名记录自动带出，无需重复填写。' : '暂无报名记录，保留当前资料方向。' }}</span>
        <span v-if="latestDirection && profile.detail?.direction && profile.detail.direction !== latestDirection" class="settings__hint">当前资料方向：{{ directionLabel(profile.detail.direction) }}</span>
        <p v-if="directionError" class="settings__field-error" role="alert">{{ directionError }}</p>
        <button v-if="directionError" type="button" class="settings__btn settings__btn--ghost" @click="emit('retry-direction')">重新核对方向</button>
      </div>
    </section>
  </form>
</template>

<style scoped>
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

.settings__readonly {
  border-bottom: 1px solid var(--line);
  padding-bottom: 16px;
}

.settings__readout {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 32px;
  margin: 0 0 8px;
}

.settings__row { min-width: 180px; }

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

.settings__editable {
  display: grid;
  gap: 14px;
}

.settings__hint--top { margin-top: -6px; }

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
  box-sizing: border-box;
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

.settings__hint {
  margin: 0;
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

.settings__status {
  font-size: 12px;
  min-height: 18px;
}

.settings__status[data-status="saving"] { color: var(--ink-soft); }
.settings__status[data-status="saved"] { color: #1c6b45; }
.settings__status[data-status="error"] { color: #a33d32; }

.settings__field-error {
  color: #a33d32;
  font-size: 12px;
  margin: 4px 0 0;
}

.settings__retry {
  margin-left: 8px;
  border: 0;
  background: none;
  color: var(--blue);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
  text-decoration: underline;
}

.settings__pick-link {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  margin-left: 2px;
  padding: 0 8px;
  border: 0;
  background: none;
  color: var(--blue);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
}

@media (pointer: coarse) {
  .settings__pick-link { min-height: 44px; }
}

.settings__pick-link:hover { text-decoration: underline; text-underline-offset: 3px; }

.settings__btn {
  padding: 9px 18px;
  border-radius: 999px;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.settings__btn--ghost {
  color: var(--ink);
  border: 1px solid var(--line);
  background: #fff;
}

@media (max-width: 400px) {
  .settings { padding: 20px 16px; }
}
</style>
