<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute } from "vue-router";

import AdminLayout from "@/components/common/AdminLayout.vue";
import SignalShell from "@/modules/swiss-signal/SignalShell.vue";
import ProfileCompletionNotice from "@/modules/user-settings/components/ProfileCompletionNotice.vue";

const route = useRoute();
const signalTransitioning = ref(false);
const usesAdminLayout = computed(() => route.meta.adminShell === true);
const usesSignal = computed(() =>
  ["/", "/login", "/user/home", "/user/settings", "/user/about"].includes(
    route.path,
  ),
);
</script>

<template>
  <el-config-provider>
    <ProfileCompletionNotice v-if="Array.isArray(route.meta.allowRoles)" />
    <AdminLayout v-if="usesAdminLayout">
      <router-view />
    </AdminLayout>
    <SignalShell
      v-if="usesSignal || signalTransitioning"
      :transition-only="!usesSignal"
      @transition-active="signalTransitioning = $event"
    ><router-view v-if="usesSignal" /></SignalShell>
    <router-view v-if="!usesAdminLayout && !usesSignal" />
  </el-config-provider>
</template>
