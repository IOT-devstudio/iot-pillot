<script setup lang="ts">
/**
 * 成员（纯展示组件）。
 *
 * 头像用**姓名首字**做文字标记，不引图片：仓库里没有 `src/assets/`，
 * 而要为假成员配头像图也不合适。将来有真实头像时再换。
 *
 * 注意 `member.title` 是工作室内部职能（「前端负责人」），**不是**系统的
 * RBAC 角色 `Role`（admin / member）—— 详见 ../fixtures.ts 的字段注释。
 */
import type { StudioMember } from "../fixtures";

defineProps<{
  members: StudioMember[];
}>();
</script>

<template>
  <section class="members" aria-labelledby="about-members-title">
    <h2 id="about-members-title" class="members__title">成员</h2>

    <ul class="members__list">
      <li v-for="member in members" :key="member.id" class="members__item">
        <span class="members__avatar" aria-hidden="true">
          {{ member.name.slice(0, 1) }}
        </span>

        <div class="members__body">
          <h3 class="members__name">{{ member.name }}</h3>
          <p class="members__role">{{ member.title }}</p>
          <p class="members__bio">{{ member.bio }}</p>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.members {
  padding: 18px 20px;
  border: 1px solid var(--about-border);
  border-radius: 10px;
  background: #fff;
}

.members__title {
  margin: 0 0 14px;
  color: var(--about-ink);
  font-size: 15px;
}

.members__list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.members__item {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 14px 16px;
  border: 1px solid var(--about-border);
  border-radius: 8px;
}

.members__avatar {
  display: flex;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  align-items: center;
  justify-content: center;
  color: var(--about-accent);
  border: 1px solid var(--about-border);
  border-radius: 50%;
  background: #f4f7fa;
  font-size: 14px;
}

.members__body {
  min-width: 0;
}

.members__name {
  margin: 0 0 4px;
  color: var(--about-ink);
  font-size: 14px;
}

.members__role {
  margin: 0 0 6px;
  color: var(--about-accent);
  font-size: 12px;
}

.members__bio {
  margin: 0;
  color: var(--about-muted);
  font-size: 12px;
  line-height: 1.6;
}
</style>
