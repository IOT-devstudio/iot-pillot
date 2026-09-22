<script setup lang="ts">
/**
 * 当前用户信息区（纯展示组件）。
 *
 * 字段口径严格跟随 `api/admin.ts` 的 `CurrentUser`，也就是后端
 * `GET /api/v1/me` 真实会返回的形状。**这里不显示邮箱**：
 * 当前后端 `domain.User` 没有 email 字段，与其在界面上摆一个编造的邮箱，
 * 不如先不显示——后端补齐后再加一行即可。
 *
 * 组件不持有状态、不发请求，数据由页面传入。
 */
import { computed } from "vue";

import type { CurrentUser } from "@/api/admin";

const props = defineProps<{
  user: CurrentUser;
}>();

/**
 * 角色的中文标签。
 * 后端加了新角色时不吞信息，直接回落到原始值——前端只做可见性展示，
 * 真正的权限判定始终在服务端 RequireRole 中间件。
 */
const ROLE_LABELS: Record<string, string> = {
  admin: "管理员",
  member: "成员",
};

const roleLabel = computed(() => ROLE_LABELS[props.user.role] ?? props.user.role);
</script>

<template>
  <section class="profile" aria-labelledby="home-profile-title">
    <h2 id="home-profile-title" class="profile__title">我的信息</h2>

    <dl class="profile__list">
      <div class="profile__row">
        <dt class="profile__term">用户名</dt>
        <dd class="profile__value">{{ user.username }}</dd>
      </div>
      <div class="profile__row">
        <dt class="profile__term">角色</dt>
        <dd class="profile__value">
          <span class="profile__role" :data-role="user.role">
            {{ roleLabel }}
          </span>
        </dd>
      </div>
    </dl>
  </section>
</template>

<style scoped>
.profile {
  padding: 18px 20px;
  border: 1px solid var(--home-border);
  border-radius: 10px;
  background: #fff;
}

.profile__title {
  margin: 0 0 14px;
  color: var(--home-ink);
  font-size: 15px;
}

.profile__list {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 40px;
  margin: 0;
}

.profile__row {
  min-width: 140px;
}

.profile__term {
  margin-bottom: 4px;
  color: var(--home-muted);
  font-size: 12px;
}

.profile__value {
  margin: 0;
  color: var(--home-ink);
  font-size: 14px;
}

.profile__role {
  display: inline-block;
  padding: 2px 10px;
  color: var(--home-accent);
  border: 1px solid var(--home-accent);
  border-radius: 999px;
  font-size: 12px;
}

/* 管理员用实心标签区分，成员保持描边即可 */
.profile__role[data-role="admin"] {
  color: #fff;
  background: var(--home-accent);
}
</style>
