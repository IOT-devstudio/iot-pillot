<script setup lang="ts">
/**
 * 修改密码入口（issue #85 重构）。
 *
 * 设置页只放一行「修改密码」字样，点击弹出液态玻璃弹窗
 * （PasswordChangeModal）完成验证码流程——上一版常驻表单卡片已移除。
 * 邮箱从父级的 /me 资料传入；未绑定邮箱时弹窗内会指路登录页忘记密码。
 */
import { ref } from "vue";

import PasswordChangeModal from "./PasswordChangeModal.vue";

defineProps<{
  /** 账号绑定邮箱（/me detail.email）；空串视为未绑定 */
  email: string;
}>();

const open = ref(false);
</script>

<template>
  <div class="pwd-entry">
    <button type="button" class="pwd-entry__link" @click="open = true">
      修改密码 <span aria-hidden="true">↗</span>
    </button>

    <PasswordChangeModal :open="open" :email="email" @close="open = false" />
  </div>
</template>

<style scoped>
.pwd-entry {
  padding: 18px 26px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: #fff;
}

.pwd-entry__link {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--blue);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.pwd-entry__link:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}
</style>
