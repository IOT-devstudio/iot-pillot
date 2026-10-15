<script setup lang="ts">
import { computed } from "vue";
import { currentProfile } from "../profileIdentity";
import { missingRequiredProfile } from "../profileCompletion";

const missing = computed(() => currentProfile.value ? missingRequiredProfile(currentProfile.value.detail) : []);
</script>

<template>
  <aside v-if="missing.length" class="profile-notice" role="status" aria-live="polite">
    <span>请补全{{ missing.join("、") }}，方便我们核对报名资料。</span>
    <RouterLink to="/user/settings">去完善资料 →</RouterLink>
  </aside>
</template>

<style scoped>
.profile-notice {
  position: relative;
  z-index: 1;
  margin: 16px auto;
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
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.6;
}
.profile-notice a { color: #164d80; font-weight: 600; text-underline-offset: 3px; }
</style>
