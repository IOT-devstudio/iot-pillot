<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";

import UserMenu from "@/components/common/UserMenu.vue";

interface AdminNavItem {
  label: string;
  to: string;
  match?: string;
  exact?: boolean;
  icon: string;
}

interface AdminNavGroup {
  label: string;
  items: AdminNavItem[];
}

const navGroups: AdminNavGroup[] = [
  {
    label: "工作台",
    items: [
      {
        label: "控制台",
        to: "/dashboard",
        exact: true,
        icon: "M3.75 4.75h6.5v6.5h-6.5z M13.75 4.75h6.5v4h-6.5z M13.75 11.75h6.5v8h-6.5z M3.75 14.25h6.5v5.5h-6.5z",
      },
      {
        label: "意向成员",
        to: "/recruitment/prospects",
        match: "/recruitment",
        icon: "M16 20v-1.5a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4V20 M9.5 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z M17 4.5a3.5 3.5 0 0 1 0 6.8 M21 20v-1.5a4 4 0 0 0-3-3.87",
      },
      {
        label: "表单管理",
        to: "/forms",
        icon: "M6 3.75h9l4 4V20.25H6z M15 3.75v4h4 M9 12h7 M9 16h7",
      },
      {
        label: "邮件中心",
        to: "/templates",
        icon: "M3.5 5.75h17v12.5h-17z M4 6.5l8 6 8-6",
      },
    ],
  },
  {
    label: "管理",
    items: [
      {
        label: "用户与权限",
        to: "/dashboard/admins",
        exact: true,
        icon: "M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M4.5 20a7.5 7.5 0 0 1 15 0 M18 8h3 M19.5 6.5v3",
      },
      {
        label: "系统设置",
        to: "/settings",
        icon: "M12 8.25a3.75 3.75 0 1 0 0 7.5 3.75 3.75 0 0 0 0-7.5z M19.4 13.5a7.8 7.8 0 0 0 0-3l1.45-1.1-1.5-2.6-1.75.7a7.8 7.8 0 0 0-2.6-1.5l-.3-1.9h-3l-.3 1.9a7.8 7.8 0 0 0-2.6 1.5l-1.75-.7-1.5 2.6L5.5 10.5a7.8 7.8 0 0 0 0 3l-1.45 1.1 1.5 2.6 1.75-.7a7.8 7.8 0 0 0 2.6 1.5l.3 1.9h3l.3-1.9a7.8 7.8 0 0 0 2.6-1.5l1.75.7 1.5-2.6z",
      },
    ],
  },
];

const route = useRoute();
const collapsed = ref(false);
const mobileOpen = ref(false);
const isMobile = ref(false);
const mobileMenuButton = ref<HTMLButtonElement | null>(null);
const mobileCloseButton = ref<HTMLButtonElement | null>(null);
const currentTitle = computed(() => String(route.meta.title ?? "控制台"));
const isProspectDetail = computed(
  () => route.name === "recruitment-prospect-detail",
);

function isActive(item: AdminNavItem): boolean {
  if (item.exact) return route.path === item.to;
  return route.path.startsWith(item.match ?? item.to);
}

function closeMobileMenu(restoreFocus = false): void {
  const shouldRestoreFocus =
    restoreFocus && mobileOpen.value && window.matchMedia("(max-width: 680px)").matches;
  mobileOpen.value = false;
  if (shouldRestoreFocus) {
    void nextTick(() => mobileMenuButton.value?.focus());
  }
}

function openMobileMenu(): void {
  mobileOpen.value = true;
  void nextTick(() => mobileCloseButton.value?.focus());
}

function closeOnEscape(event: KeyboardEvent): void {
  if (event.key === "Escape") closeMobileMenu(true);
}

watch(
  () => route.fullPath,
  () => closeMobileMenu(true),
);

let mobileMediaQuery: MediaQueryList | null = null;
function updateMobileViewport(event?: MediaQueryList | MediaQueryListEvent): void {
  isMobile.value = event?.matches ?? window.matchMedia("(max-width: 680px)").matches;
  if (!isMobile.value) mobileOpen.value = false;
}

onMounted(() => {
  window.addEventListener("keydown", closeOnEscape);
  mobileMediaQuery = window.matchMedia("(max-width: 680px)");
  updateMobileViewport(mobileMediaQuery);
  mobileMediaQuery.addEventListener("change", updateMobileViewport);
});
onBeforeUnmount(() => {
  window.removeEventListener("keydown", closeOnEscape);
  mobileMediaQuery?.removeEventListener("change", updateMobileViewport);
});
</script>

<template>
  <div class="admin-shell" :class="{ 'admin-shell--collapsed': collapsed }">
    <button
      v-if="mobileOpen"
      class="admin-shell__scrim"
      type="button"
      aria-label="关闭导航菜单"
      @click="closeMobileMenu(true)"
    />

    <aside
      id="admin-sidebar"
      class="admin-sidebar"
      :class="{ 'admin-sidebar--open': mobileOpen }"
      :inert="isMobile && !mobileOpen"
      aria-label="管理端侧边栏"
    >
      <div class="admin-sidebar__head">
        <router-link to="/dashboard" class="admin-brand" aria-label="iot-pillot 控制台">
          <span class="admin-brand__mark" aria-hidden="true">i/o</span>
          <span class="admin-brand__name">iot-pillot</span>
        </router-link>
        <button
          class="admin-sidebar__collapse"
          type="button"
          :aria-label="collapsed ? '展开侧边栏' : '折叠侧边栏'"
          :aria-expanded="!collapsed"
          title="折叠或展开侧边栏"
          @click="collapsed = !collapsed"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M9 5 16 12 9 19" />
          </svg>
        </button>
        <button
          class="admin-sidebar__close"
          ref="mobileCloseButton"
          type="button"
          aria-label="关闭导航菜单"
          @click="closeMobileMenu(true)"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="m6 6 12 12M18 6 6 18" />
          </svg>
        </button>
      </div>

      <div class="admin-sidebar__rule" />

      <nav class="admin-navigation" aria-label="后台导航">
        <div v-for="(group, index) in navGroups" :key="group.label" class="admin-navigation__group">
          <p class="admin-navigation__heading">{{ group.label }}</p>
          <div class="admin-navigation__items">
            <router-link
              v-for="item in group.items"
              :key="item.to"
              :to="item.to"
              class="admin-navigation__link"
              :class="{ 'is-active': isActive(item) }"
              :aria-current="isActive(item) ? 'page' : undefined"
              :aria-label="collapsed ? item.label : undefined"
              :title="collapsed ? item.label : undefined"
            >
              <svg class="admin-navigation__icon" viewBox="0 0 24 24" aria-hidden="true">
                <path :d="item.icon" />
              </svg>
              <span class="admin-navigation__label">{{ item.label }}</span>
            </router-link>
          </div>
          <div v-if="index < navGroups.length - 1" class="admin-navigation__separator" />
        </div>
      </nav>

      <footer class="admin-sidebar__footer">
        <span class="admin-sidebar__season">2026 年招新</span>
        <span class="admin-sidebar__signature">IOT-PILLOT · ADMIN CONSOLE</span>
      </footer>
    </aside>

    <div class="admin-main">
      <div class="admin-main__mobilebar">
        <button
          ref="mobileMenuButton"
          class="admin-main__menu"
          type="button"
          aria-label="打开导航菜单"
          aria-controls="admin-sidebar"
          :aria-expanded="mobileOpen"
          @click="openMobileMenu"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M4 7h16M4 12h16M4 17h16" />
          </svg>
        </button>
        <router-link to="/dashboard" class="admin-main__mobile-brand">
          <span class="admin-brand__mark" aria-hidden="true">i/o</span>
          <span>iot-pillot</span>
        </router-link>
      </div>

      <div class="admin-main__inner">
        <header class="admin-utility">
          <div class="admin-utility__crumb" aria-label="当前位置">
            <span>招新管理</span>
            <span aria-hidden="true">/</span>
            <strong>{{ isProspectDetail ? "意向成员" : currentTitle }}</strong>
            <template v-if="isProspectDetail">
              <span aria-hidden="true">/</span>
              <span>详情</span>
            </template>
          </div>
          <UserMenu />
        </header>
        <slot />
      </div>
    </div>
  </div>
</template>

<style scoped>
.admin-shell {
  --sidebar-size: 246px;
  display: grid;
  grid-template-columns: var(--sidebar-size) minmax(0, 1fr);
  min-height: 100dvh;
  transition: grid-template-columns 200ms ease;
}

.admin-shell--collapsed {
  --sidebar-size: 76px;
}

.admin-sidebar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  flex-direction: column;
  width: var(--sidebar-size);
  height: 100dvh;
  padding: 25px 16px 21px;
  overflow: hidden;
  border-right: 1px solid var(--line);
  background: #faf8f1;
  transition: width 200ms ease, padding 200ms ease;
}

.admin-sidebar__head {
  display: flex;
  min-height: 36px;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 0 8px;
}

.admin-brand,
.admin-main__mobile-brand {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 11px;
  color: var(--ink);
  text-decoration: none;
}

.admin-brand__mark {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--blue);
  border-radius: 50%;
  color: var(--blue);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.04em;
}

.admin-brand__name,
.admin-main__mobile-brand > span:last-child {
  overflow: hidden;
  font-family: var(--font-serif);
  font-size: 19px;
  font-weight: 700;
  letter-spacing: -0.03em;
  white-space: nowrap;
}

.admin-sidebar__collapse,
.admin-sidebar__close,
.admin-main__menu {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  place-items: center;
  padding: 0;
  color: var(--ink-soft);
  border: 1px solid transparent;
  border-radius: 3px;
  background: transparent;
  cursor: pointer;
  transition: color 160ms ease, border-color 160ms ease, background 160ms ease;
}

.admin-sidebar__collapse svg,
.admin-sidebar__close svg,
.admin-main__menu svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.7;
}

.admin-sidebar__collapse:hover,
.admin-sidebar__close:hover,
.admin-main__menu:hover {
  color: var(--blue);
  border-color: var(--line);
  background: rgb(22 77 128 / 5%);
}

.admin-sidebar__collapse:focus-visible,
.admin-sidebar__close:focus-visible,
.admin-main__menu:focus-visible,
.admin-brand:focus-visible,
.admin-navigation__link:focus-visible,
.admin-main__mobile-brand:focus-visible {
  outline: 2px solid var(--blue-bright);
  outline-offset: 3px;
}

.admin-sidebar__close,
.admin-main__mobilebar {
  display: none;
}

.admin-sidebar__rule {
  height: 1px;
  flex: 0 0 auto;
  margin: 27px 10px 22px;
  background: var(--line);
}

.admin-navigation {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.admin-navigation__heading {
  margin: 0 11px 9px;
  color: #858f99;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  white-space: nowrap;
}

.admin-navigation__items {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.admin-navigation__link {
  position: relative;
  display: flex;
  min-height: 44px;
  align-items: center;
  gap: 12px;
  padding: 0 12px;
  overflow: hidden;
  color: var(--ink-soft);
  border-left: 2px solid transparent;
  text-decoration: none;
  transition: color 160ms ease, background 160ms ease, border-color 160ms ease;
}

.admin-navigation__link:hover {
  color: var(--ink);
  background: rgb(16 43 78 / 5%);
}

.admin-navigation__link.is-active {
  color: var(--blue);
  border-left-color: var(--blue);
  background: rgb(22 77 128 / 8%);
}

.admin-navigation__icon {
  width: 18px;
  height: 18px;
  flex: 0 0 auto;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.55;
}

.admin-navigation__label {
  overflow: hidden;
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: opacity 120ms ease, max-width 200ms ease;
}

.admin-navigation__separator {
  height: 1px;
  margin: 18px 10px 17px;
  background: var(--line-soft);
}

.admin-sidebar__footer {
  display: flex;
  flex-direction: column;
  gap: 7px;
  margin-top: auto;
  padding: 20px 11px 0;
  border-top: 1px solid var(--line);
  white-space: nowrap;
}

.admin-sidebar__season {
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 18px;
  font-weight: 600;
}

.admin-sidebar__signature {
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 9px;
  letter-spacing: 0.08em;
}

.admin-main {
  position: relative;
  min-width: 0;
  min-height: 100dvh;
  overflow: hidden;
  background-color: var(--paper);
  isolation: isolate;
}

.admin-main::before {
  position: absolute;
  z-index: -1;
  inset: 0;
  background-image:
    radial-gradient(circle at 88% 2%, rgb(22 77 128 / 7%), transparent 34%),
    linear-gradient(rgb(16 43 78 / 3%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(16 43 78 / 3%) 1px, transparent 1px);
  background-size: 100% 100%, 40px 40px, 40px 40px;
  content: "";
  pointer-events: none;
}

.admin-main::after {
  position: absolute;
  z-index: -1;
  top: -150px;
  right: -180px;
  width: min(42vw, 540px);
  aspect-ratio: 1;
  border: 1px solid rgb(22 77 128 / 5%);
  border-radius: 50%;
  box-shadow: 0 0 0 80px rgb(22 77 128 / 2%), 0 0 0 160px rgb(22 77 128 / 1%);
  content: "";
  pointer-events: none;
}

.admin-main__inner {
  position: relative;
  z-index: 1;
  width: min(100%, 1670px);
  margin: 0 auto;
  padding: 25px clamp(26px, 3.6vw, 68px) 48px;
}

.admin-utility {
  display: flex;
  min-height: 34px;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.admin-utility__crumb {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--ink-faint);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
}

.admin-utility__crumb strong {
  color: var(--blue);
  font-weight: 700;
}

.admin-shell__scrim {
  display: none;
}

.admin-shell--collapsed .admin-brand__name,
.admin-shell--collapsed .admin-navigation__heading,
.admin-shell--collapsed .admin-navigation__label,
.admin-shell--collapsed .admin-sidebar__footer {
  max-width: 0;
  opacity: 0;
  pointer-events: none;
}

.admin-shell--collapsed .admin-sidebar {
  padding-right: 12px;
  padding-left: 12px;
}

.admin-shell--collapsed .admin-sidebar__head {
  flex-direction: column;
  gap: 15px;
  padding: 0;
}

.admin-shell--collapsed .admin-sidebar__collapse svg {
  transform: rotate(180deg);
}

.admin-shell--collapsed .admin-navigation__link {
  justify-content: center;
  padding: 0;
}

.admin-shell--collapsed .admin-navigation__separator {
  margin-right: 4px;
  margin-left: 4px;
}

@media (max-width: 1100px) {
  .admin-main__inner {
    padding-right: 30px;
    padding-left: 30px;
  }
}

@media (max-width: 680px) {
  .admin-shell,
  .admin-shell--collapsed {
    display: block;
    min-height: 100dvh;
  }

  .admin-sidebar,
  .admin-shell--collapsed .admin-sidebar {
    position: fixed;
    top: 0;
    bottom: 0;
    left: 0;
    z-index: 42;
    width: min(82vw, 290px);
    height: 100dvh;
    padding: 19px 16px 20px;
    box-shadow: 12px 0 36px rgb(16 43 78 / 12%);
    transform: translateX(-105%);
    transition: transform 220ms ease;
  }

  .admin-sidebar--open {
    transform: translateX(0);
  }

  .admin-shell--collapsed .admin-sidebar__head {
    flex-direction: row;
    gap: 6px;
    padding: 0 8px;
  }

  .admin-shell--collapsed .admin-brand__name,
  .admin-shell--collapsed .admin-navigation__heading,
  .admin-shell--collapsed .admin-navigation__label,
  .admin-shell--collapsed .admin-sidebar__footer {
    max-width: 280px;
    opacity: 1;
    pointer-events: auto;
  }

  .admin-shell--collapsed .admin-sidebar__collapse {
    display: none;
  }

  .admin-sidebar__collapse,
  .admin-shell--collapsed .admin-sidebar__collapse {
    display: none;
  }

  .admin-sidebar__close {
    display: grid;
  }

  .admin-navigation__link,
  .admin-shell--collapsed .admin-navigation__link {
    justify-content: flex-start;
    gap: 12px;
    padding: 0 12px;
  }

  .admin-shell__scrim {
    position: fixed;
    z-index: 41;
    inset: 0;
    display: block;
    border: 0;
    background: rgb(16 32 48 / 35%);
    backdrop-filter: blur(2px);
  }

  .admin-main__mobilebar {
    position: relative;
    z-index: 2;
    display: flex;
    min-height: 59px;
    align-items: center;
    gap: 13px;
    padding: 0 19px;
    border-bottom: 1px solid var(--line);
    background: rgb(250 248 241 / 92%);
    backdrop-filter: blur(12px);
  }

  .admin-main__mobile-brand {
    gap: 8px;
  }

  .admin-main__mobile-brand .admin-brand__mark {
    width: 29px;
    height: 29px;
  }

  .admin-main__mobile-brand > span:last-child {
    font-size: 16px;
  }

  .admin-main__inner {
    padding: 15px 18px 34px;
  }

  .admin-utility {
    min-height: 30px;
    margin-bottom: 5px;
  }

  .admin-utility__crumb {
    display: none;
  }

  .admin-utility :deep(.sign-out) {
    padding: 6px 9px;
    font-size: 11px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .admin-shell,
  .admin-sidebar,
  .admin-sidebar__collapse,
  .admin-navigation__link,
  .admin-navigation__label {
    transition: none;
  }
}
</style>
