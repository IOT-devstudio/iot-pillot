<script setup lang="ts">
import { ref, onMounted } from "vue";
import type { ApiResponse } from "@iot-pillot/shared-types";

interface HealthData {
  status: string;
  time: string;
}

const health = ref<string>("检测中…");

onMounted(async () => {
  try {
    const r = await fetch("/api/health");
    const body = (await r.json()) as ApiResponse<HealthData>;
    health.value = `${body.data.status} @ ${body.data.time}`;
  } catch (e) {
    health.value = `连接失败：${(e as Error).message}`;
  }
});
</script>

<template>
  <el-container class="home">
    <el-header>
      <h1>iot-pillot</h1>
    </el-header>
    <el-main>
      <el-card header="招新管理控制台">
        <p>
          API 健康检查：
          <el-tag :type="health.startsWith('ok') ? 'success' : 'danger'">
            {{ health }}
          </el-tag>
        </p>
        <el-alert
          title="脚手架已就绪"
          type="success"
          :closable="false"
          description="Vue 3 + Element Plus + TypeScript + Vite — 通过 /api 代理与后端通信"
        />
      </el-card>
    </el-main>
  </el-container>
</template>

<style scoped>
.home {
  min-height: 100vh;
  background: #f5f7fa;
}
h1 {
  margin: 0;
  line-height: 60px;
}
</style>
