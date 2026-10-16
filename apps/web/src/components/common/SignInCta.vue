<script setup lang="ts">
import { useSignInGate } from "@/auth/useSignInGate";
const { signIn } = useSignInGate();
</script>

<template>
  <button type="button" class="sign-in-cta" @click="signIn">登录 / 注册 <span aria-hidden="true">↗</span></button>
</template>

<style scoped>
.sign-in-cta {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  padding: 10px 14px;
  border: 1px solid currentColor;
  background: var(--acid, #e4ff5d);
  color: var(--ink, #111);
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
  cursor: pointer;
}
.sign-in-cta:hover { background: var(--ink, #111); color: var(--paper, #f6f4eb); }
.sign-in-cta:focus-visible { outline: 2px solid currentColor; outline-offset: 4px; }
</style>
