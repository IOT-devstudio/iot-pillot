<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/common/AdminLayout.vue";
import SignalShell from "@/modules/swiss-signal/SignalShell.vue";

const route = useRoute();
const usesAdminLayout = computed(() => route.meta.adminShell === true);
const usesSignal = computed(() =>
  ["/", "/login", "/user/home", "/user/settings"].includes(route.path),
);
</script>

<template>
  <el-config-provider>
    <AdminLayout v-if="usesAdminLayout">
      <router-view />
    </AdminLayout>
    <SignalShell v-else-if="usesSignal"><router-view /></SignalShell>
    <router-view v-else />
  </el-config-provider>
</template>
