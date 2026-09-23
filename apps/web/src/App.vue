<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/common/AdminLayout.vue";

const route = useRoute();
const usesAdminLayout = computed(() => route.meta.adminShell === true);
</script>

<template>
  <el-config-provider>
    <AdminLayout v-if="usesAdminLayout">
      <router-view />
    </AdminLayout>
    <router-view v-else />
  </el-config-provider>
</template>
