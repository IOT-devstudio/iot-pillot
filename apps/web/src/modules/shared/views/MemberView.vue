<script setup lang="ts">
/**
 * 普通用户（member）的登录落点。
 *
 * 本期是占位页：真正的内容要等对应的业务模块开工后再填。
 * 放在这里的作用是让角色分流有明确落点，也方便验证守卫行为。
 */
</script>

<template>
  <el-container class="member-page">
    <el-main>
      <el-card class="member-card">
        <h1>个人中心</h1>
        <p>待开发</p>
      </el-card>
    </el-main>
  </el-container>
</template>

<style scoped>
.member-page {
  min-height: 100vh;
  background: #f5f7fa;
}

.member-card {
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
