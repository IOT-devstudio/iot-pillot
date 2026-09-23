<script setup lang="ts">
/**
 * 用户详情弹窗（issue #58 验收项 /dashboard/admins 「详情」按钮）。
 *
 * 只读：编辑入口只在个人设置页（/user/settings），弹窗里**不**放任何编辑控件。
 * 这是契约要求：「仅本人可改」—— 管理员不能在这里代改他人。
 *
 * 数据来源：列表里已扩的 detail 字段（#57 后端契约）。打开弹窗不发起第二
 * 个请求；想看最新就刷新页面（验收项的「实时更新」口径）。
 */
import { computed } from "vue";
import type { Direction } from "@iot-pillot/shared-types";

import type { AdminUser } from "@/api/admin";

const props = defineProps<{
  /** 控制弹窗显隐。父组件用 v-model:visible */
  visible: boolean;
  /** 当前查看的用户；为 null 时弹窗内容占位但不显示任何字段 */
  user: AdminUser | null;
  /** 当前登录账号 ID，用于在角色行追加「当前登录账号」标记 */
  currentUserId?: number | null;
}>();

const emit = defineEmits<{
  (event: "update:visible", value: boolean): void;
}>();

function close(): void {
  emit("update:visible", false);
}

const DIRECTION_LABELS: Record<Direction, string> = {
  "front-end": "前端",
  "back-end": "后端",
  agent: "智能体",
  all: "全栈",
  game: "游戏",
  other: "其他",
};

function directionLabel(direction: Direction | null | undefined): string {
  if (direction === null || direction === undefined) {
    return "未选择";
  }
  return DIRECTION_LABELS[direction] ?? direction;
}

function display(value: string | number | null | undefined): string {
  if (value === null || value === undefined) {
    return "未填写";
  }
  const trimmed = typeof value === "string" ? value.trim() : String(value);
  return trimmed === "" ? "未填写" : trimmed;
}

/**
 * 「字段是否真的没数据」和「display(value)」分离——前者用来决定要不要展示
 * 「暂无完整资料」空态，后者只决定如何呈现单个字段。前端只在 detail 整体
 * 缺失时给空态，避免一长串「未填写」看着像坏掉。
 */
const detailMissing = computed(() => props.user?.detail === undefined);

const isSelf = computed(
  () =>
    props.user !== null &&
    props.currentUserId !== null &&
    props.currentUserId !== undefined &&
    props.user.user_id === props.currentUserId,
);
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="用户详情"
    width="560"
    :close-on-click-modal="true"
    :close-on-press-escape="true"
    @update:model-value="emit('update:visible', $event)"
  >
    <!-- 没有 user（点开瞬间）时不渲染任何字段，避免把上一位用户的信息闪一下 -->
    <template v-if="user === null">
      <p class="detail-empty">正在打开…</p>
    </template>

    <template v-else>
      <p
        v-if="detailMissing"
        class="detail-empty"
        role="status"
      >
        暂无完整资料。后端 #57 落地后将自动填充。
      </p>

      <dl v-else class="detail">
        <div class="detail__row">
          <dt class="detail__term">姓名</dt>
          <dd class="detail__value">{{ display(user.name) }}</dd>
        </div>
        <div class="detail__row">
          <dt class="detail__term">班级</dt>
          <dd class="detail__value">{{ display(user.detail?.class) }}</dd>
        </div>
        <div class="detail__row">
          <dt class="detail__term">学号</dt>
          <dd class="detail__value">{{ display(user.detail?.student_id) }}</dd>
        </div>
        <div class="detail__row">
          <dt class="detail__term">邮箱</dt>
          <dd class="detail__value">{{ display(user.detail?.email) }}</dd>
        </div>
        <div class="detail__row">
          <dt class="detail__term">QQ</dt>
          <dd class="detail__value">{{ display(user.detail?.qq) }}</dd>
        </div>
        <div class="detail__row">
          <dt class="detail__term">方向</dt>
          <dd class="detail__value">{{ directionLabel(user.detail?.direction) }}</dd>
        </div>
        <div class="detail__row">
          <dt class="detail__term">注册时间</dt>
          <dd class="detail__value">{{ display(user.created_at) }}</dd>
        </div>
        <div class="detail__row">
          <dt class="detail__term">角色</dt>
          <dd class="detail__value">
            <span class="detail__role" :data-self="isSelf ? '1' : '0'">
              {{ user.is_admin ? "管理员" : "普通用户" }}
            </span>
            <span v-if="isSelf" class="detail__self-tag">（当前登录账号）</span>
          </dd>
        </div>
      </dl>

      <p class="detail__footnote">
        详情只读 —— 编辑入口在「个人设置」页（仅本人可改）。
      </p>
    </template>

    <template #footer>
      <el-button @click="close">关闭</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.detail-empty {
  margin: 0;
  padding: 18px 16px;
  color: var(--ink-soft);
  border: 1px dashed var(--line);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.02);
  font-size: 13px;
  text-align: center;
}

.detail {
  display: grid;
  gap: 12px 28px;
  grid-template-columns: max-content 1fr;
  margin: 0;
  padding: 0;
}

.detail__row {
  display: contents;
}

.detail__term {
  margin: 0;
  color: var(--ink-soft);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  white-space: nowrap;
}

.detail__value {
  margin: 0;
  color: var(--ink);
  font-size: 14px;
}

.detail__role[data-self="1"] {
  padding: 2px 10px;
  color: #fff;
  border-radius: 999px;
  background: var(--blue);
  font-size: 12px;
}

.detail__role[data-self="0"] {
  padding: 2px 10px;
  border: 1px solid var(--line);
  border-radius: 999px;
  font-size: 12px;
}

.detail__self-tag {
  margin-left: 8px;
  color: var(--ink-faint);
  font-size: 12px;
}

.detail__footnote {
  margin: 18px 0 0;
  color: var(--ink-faint);
  font-size: 11px;
  line-height: 1.6;
}
</style>