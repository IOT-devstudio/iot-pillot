<script setup lang="ts">
// 根组件：仅承载 router-view 与 Element Plus 全局配置。
</script>

<template>
  <el-config-provider>
    <router-view />
  </el-config-provider>
</template>
