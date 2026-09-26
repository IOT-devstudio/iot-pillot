<script setup lang="ts">
/**
 * 当前用户信息区（纯展示组件）。
 *
 * 字段口径跟随 `api/profile.ts` 的 `MyProfile`，也就是后端
 * `GET /api/v1/me` 真实会返回的形状。后端 #57 落地前由
 * `home/profileFixture.ts` 派生。**这里不显示邮箱**：
 * 邮箱属于登录凭据，与方向/班级/学号不在同一展示语境，
 * 不在卡片里露出来；编辑入口 `/user/settings` 里有专门的字段。
 *
 * 组件不持有状态、不发请求，数据由页面传入。
 *
 * 编辑资料按钮跳 `/user/settings`：编辑能力只在个人设置页，
 * 详情弹窗与个人卡片都按 issue #58 的契约不放编辑控件。
 */
import { computed } from "vue";

import type { Direction } from "@iot-pillot/shared-types";
import type { MyProfile } from "@/api/profile";

const props = defineProps<{
  user: MyProfile;
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

const DIRECTION_LABELS: Record<Direction, string> = {
  "front-end": "前端",
  "back-end": "后端",
  agent: "智能体",
  all: "全栈",
  game: "游戏",
  other: "其他",
};

const directionLabel = computed(() => {
  const d = props.user.detail?.direction;
  if (d === null || d === undefined) {
    return "未选择";
  }
  return DIRECTION_LABELS[d] ?? d;
});

const directionIsEmpty = computed(() => {
  const d = props.user.detail?.direction;
  return d === null || d === undefined;
});
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
      <div class="profile__row">
        <dt class="profile__term">方向</dt>
        <dd class="profile__value">
          <span
            class="profile__direction"
            :data-empty="directionIsEmpty ? '1' : '0'"
          >
            {{ directionLabel }}
          </span>
        </dd>
      </div>
    </dl>

    <footer class="profile__footer">
      <RouterLink class="profile__link" to="/user/settings">
        编辑资料
      </RouterLink>
    </footer>
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

.profile__direction {
  display: inline-block;
  padding: 2px 10px;
  color: var(--home-accent);
  border: 1px solid var(--home-accent);
  border-radius: 999px;
  font-size: 12px;
}

/* 还没填方向时降级为中性样式，避免一颗亮蓝标签写着「未选择」 */
.profile__direction[data-empty="1"] {
  color: var(--home-muted);
  border-color: var(--home-border);
  background: transparent;
}

.profile__footer {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--home-border);
}

.profile__link {
  color: var(--home-accent);
  text-decoration: none;
  font-size: 13px;
}

.profile__link:hover {
  text-decoration: underline;
}
</style>
