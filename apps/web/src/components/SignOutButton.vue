<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";

import { signOut } from "@/auth/session";

const router = useRouter();
const signingOut = ref(false);

async function handleSignOut(): Promise<void> {
  if (signingOut.value) {
    return;
  }

  signingOut.value = true;
  try {
    await signOut();
  } catch {
    // signOut 已在 finally 中清理本地会话；网络失败不应把用户留在已退出页面。
  } finally {
    signingOut.value = false;
    await router.replace("/");
  }
}
</script>

<template>
  <button
    class="sign-out"
    type="button"
    :disabled="signingOut"
    @click="handleSignOut"
  >
    {{ signingOut ? "退出中…" : "退出登录" }}
  </button>
</template>

<style scoped>
.sign-out {
  padding: 8px 14px;
  color: #164d80;
  border: 1px solid #b8c7d6;
  border-radius: 999px;
  background: #fff;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.sign-out:hover:not(:disabled) {
  background: #edf4fa;
}

.sign-out:disabled {
  cursor: wait;
  opacity: 0.6;
}
</style>
