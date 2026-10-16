<script setup lang="ts">
import SignalDialog from "@/modules/swiss-signal/SignalDialog.vue";
defineProps<{ action: string | null }>();
defineEmits<{ close: []; confirm: [] }>();
</script>

<template>
  <SignalDialog v-if="action" title="需要登录" @close="$emit('close')">
    <p class="dialog-description">登录后即可{{ action }}，我们会带你回到当前页面。</p>
    <p class="dialog-note">你也可以继续浏览招新方向与工作室介绍。</p>
    <button type="button" class="action-button" @click="$emit('confirm')">
      <span>去登录</span><b aria-hidden="true">↗</b>
    </button>
    <button type="button" class="plain-link sign-in-dismiss" @click="$emit('close')">继续浏览</button>
  </SignalDialog>
</template>

<style scoped>
.sign-in-dismiss { display: block; margin-top: 20px; min-height: 44px; }
</style>
