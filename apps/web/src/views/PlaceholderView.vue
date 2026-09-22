<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import AppHeader from "@/components/common/AppHeader.vue";
import BackToDashboard from "@/components/common/BackToDashboard.vue";
import PageHeader from "@/components/common/PageHeader.vue";

const route = useRoute();
const pageTitle = computed(() => String(route.meta.title ?? "页面"));
</script>

<template>
  <div class="page">
    <AppHeader />

    <main class="page__body">
      <BackToDashboard />

      <PageHeader eyebrow="iot-pillot · 招新管理" :title="pageTitle" />

      <section class="panel panel--pad placeholder-card">
        <p class="placeholder-card__index" aria-hidden="true">待开发</p>
      </section>
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
}

.page__body {
  display: flex;
  flex-direction: column;
  gap: 28px;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

.placeholder-card {
  min-height: 220px;
}

.placeholder-card__index {
  margin: 0 0 10px;
  color: var(--red);
  font-family: var(--font-kai);
  font-size: 18px;
}
</style>
