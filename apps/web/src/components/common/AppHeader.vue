<script setup lang="ts">
/**
 * 全局顶栏：品牌词标 + 后台导航 + 身份菜单。
 *
 * 只做导航与品牌展示，不含任何鉴权判断（那是 router/guards.ts 的职责）。
 * 隐藏菜单 ≠ 权限控制，敏感操作仍由后端鉴权。
 *
 * 身份菜单由 UserMenu 提供（issue #58）：把散在各页 PageHeader#actions 里
 * 的 SignOutButton 收进同一个下拉入口。顶栏导航项数量保持不变。
 */
import { useRoute } from "vue-router";

import UserMenu from "./UserMenu.vue";

interface NavItem {
  label: string;
  to: string;
  /** 可选：前缀匹配路径，用于让详情页也点亮父级导航 */
  match?: string;
  /**
   * 精确匹配。用于**有子页面的项**：控制台是 /dashboard，它的子页面
   * /dashboard/admins 也在导航里，若按前缀匹配会让两项同时点亮。
   */
  exact?: boolean;
}

const nav: NavItem[] = [
  // 控制台 = 管理台 /dashboard（管理员登录落点）；根路径 / 是 3D 登录开屏，
  // /home 是独立的内容首页，都不在这套导航里。
  { label: "控制台", to: "/dashboard", exact: true },
  { label: "意向成员", to: "/recruitment/prospects", match: "/recruitment" },
  { label: "表单管理", to: "/forms" },
  { label: "邮件模板", to: "/templates" },
  // 用户与权限归在设置类，排在系统设置之前（其余业务模块之后）。
  { label: "用户与权限", to: "/dashboard/admins" },
  { label: "系统设置", to: "/settings" },
];

const route = useRoute();

function isActive(item: NavItem): boolean {
  if (item.exact) return route.path === item.to;
  const probe = item.match ?? item.to;
  if (probe === "/") return route.path === "/";
  return route.path.startsWith(probe);
}
</script>

<template>
  <header class="topbar">
    <router-link to="/" class="topbar__brand" aria-label="iot-pillot 首页">
      <span class="topbar__mark" aria-hidden="true">i/o</span>
      <span class="topbar__name">iot-pillot</span>
    </router-link>

    <nav class="topbar__nav" aria-label="后台导航">
      <router-link
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        class="topbar__link"
        :class="{ 'is-active': isActive(item) }"
      >
        {{ item.label }}
      </router-link>
    </nav>

    <span class="topbar__edition">2026年招新</span>
    <UserMenu class="topbar__user-menu" />
  </header>
</template>

<style scoped>
.topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: clamp(20px, 4vw, 48px);
  height: 58px;
  padding: 0 clamp(20px, 4vw, 48px);
  border-bottom: 1px solid var(--line);
  background: rgb(255 253 247 / 88%);
  backdrop-filter: blur(12px);
}

.topbar__brand {
  display: inline-flex;
  gap: 10px;
  align-items: center;
  color: var(--ink);
  text-decoration: none;
}

.topbar__mark {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid var(--blue);
  border-radius: 50%;
  color: var(--blue);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.topbar__name {
  font-family: var(--font-serif);
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.topbar__nav {
  display: flex;
  gap: 2px;
  align-self: stretch;
}

.topbar__link {
  position: relative;
  display: grid;
  padding: 0 16px;
  place-items: center;
  color: var(--ink-soft);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-decoration: none;
  transition: color 160ms ease;
}

.topbar__link::after {
  position: absolute;
  right: 12px;
  bottom: 0;
  left: 12px;
  height: 2px;
  background: var(--blue);
  content: "";
  opacity: 0;
  transform: scaleX(0.4);
  transition: opacity 180ms ease, transform 180ms ease;
}

.topbar__link:hover {
  color: var(--ink);
}

.topbar__link.is-active {
  color: var(--blue);
}

.topbar__link.is-active::after {
  opacity: 1;
  transform: scaleX(1);
}

.topbar__edition {
  margin-left: auto;
  padding-bottom: 3px;
  color: var(--ink-faint);
  border-bottom: 1px solid var(--line);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  white-space: nowrap;
}

/* 身份菜单在 edition 之后贴右；与 edition 共享「右半区」 */
.topbar__user-menu {
  flex: 0 0 auto;
}

@media (max-width: 860px) {
  .topbar {
    gap: 12px;
    height: auto;
    min-height: 58px;
    flex-wrap: wrap;
    padding: 10px 20px;
  }

  .topbar__nav {
    order: 3;
    width: 100%;
    overflow-x: auto;
  }

  .topbar__link {
    padding: 10px 14px;
  }

  .topbar__edition {
    margin-left: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .topbar__link::after {
    transition: none;
  }
}
</style>
