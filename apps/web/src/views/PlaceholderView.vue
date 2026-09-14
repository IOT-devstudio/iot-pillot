<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

const route = useRoute();
const pageTitle = computed(() => String(route.meta.title ?? "页面"));
</script>

<template>
  <el-container class="placeholder-page">
    <el-main>
      <el-card class="placeholder-card">
        <h1>{{ pageTitle }}</h1>
        <p>待开发</p>
      </el-card>
    </el-main>
  </el-container>
</template>

<style scoped>
.placeholder-page {
  min-height: 100vh;
  background: #f5f7fa;
}

.placeholder-card {
  max-width: 720px;
  margin: 15vh auto 0;
  text-align: center;
}

h1 {
  margin: 0 0 16px;
}

p {
  margin: 0;
  color: #909399;
}
</style>
