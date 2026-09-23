<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import PageHeader from "@/components/common/PageHeader.vue";

const route = useRoute();
const pageTitle = computed(() => String(route.meta.title ?? "页面"));
</script>

<template>
  <div class="page">
    <main class="page__body">
      <PageHeader eyebrow="iot-pillot · 招新管理" :title="pageTitle" />

      <section class="panel panel--pad placeholder-card">
        <p class="placeholder-card__index" aria-hidden="true">待开发</p>
      </section>
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: calc(100dvh - 60px);
}

.page__body {
  display: flex;
  flex-direction: column;
  gap: 28px;
  width: 100%;
  margin: 0;
  padding: 29px 0 44px;
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
