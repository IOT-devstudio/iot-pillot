<script setup lang="ts">
import { computed } from "vue";
import { currentProfile } from "../profileIdentity";
import { missingRequiredProfile } from "../profileCompletion";

/**
 * 创始管理员不出这条提示（review 裁决）：无邮箱、无资料可补。
 * 普通晋升管理员照旧——他们可能也是报名上来的成员，需要核对资料。
 */
const missing = computed(() => {
  const profile = currentProfile.value;
  if (profile === null || profile.is_founding === true) {
    return [];
  }
  return missingRequiredProfile(profile.detail);
});
</script>

<template>
  <aside v-if="missing.length" class="profile-notice" role="status" aria-live="polite">
    <span>请补全{{ missing.join("、") }}，方便我们核对报名资料。</span>
    <RouterLink to="/user/settings">去完善资料 →</RouterLink>
  </aside>
</template>

<style scoped>
/* 浮动悬浮（review 反馈）：不占文档流，压在页面内容与侧边栏之上——
   此前它是块级一行，会把管理端侧边栏整体顶下去 */
.profile-notice {
  position: fixed;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 80;
  width: min(560px, calc(100% - 24px));
  box-sizing: border-box;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 20px;
  padding: 14px 18px;
  color: #183c55;
  background: #f2f7fa;
  border: 1px solid #b8cddc;
  border-radius: 10px;
  box-shadow: 0 12px 32px -12px rgba(10, 16, 40, 0.25);
  font-size: 13px;
  line-height: 1.6;
}
.profile-notice a { color: #164d80; font-weight: 600; text-underline-offset: 3px; }
</style>
