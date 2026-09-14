# Route Placeholder Pages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Vue 3 前端注册登录、控制台、表单管理、招聘管理、邮件模板和系统设置六个占位页面路由。

**Architecture:** 保留现有首页和健康检查逻辑。将新路由集中声明在独立的路由记录文件中，统一指向一个通过 `route.meta.title` 显示页面名称的 `PlaceholderView`，再由 `router/index.ts` 创建 Vue Router 实例。

**Tech Stack:** Vue 3 `<script setup>`、TypeScript、Vue Router 4、Element Plus、Vite、Vitest。

## Global Constraints

- 新页面只显示中文页面名称和“待开发”，不请求 API、不实现业务逻辑。
- 保留 `/` 首页路由及其 API 健康检查行为。
- 路由必须使用 Vue Router 4 的 `createWebHistory`。
- 页面标题通过路由 `meta.title` 传递，六个占位路由共用一个视图组件。
- 不增加后台布局、导航、路由守卫、后端路由或共享 DTO。
- 遵循仓库约定：前端命令从仓库根目录执行，提交使用 Conventional Commits。

## 文件结构

- Modify: `apps/web/package.json` — 增加 `test` 脚本和前端路由单测所需的开发依赖。
- Modify: `pnpm-lock.yaml` — 由 pnpm 安装测试依赖时更新；保留工作区中已有的用户改动。
- Create: `apps/web/src/router/route-specs.ts` — 导出不依赖 UI 的六个占位路由规格，供路由实现与测试共用。
- Create: `apps/web/src/router/routes.ts` — 导出首页和六个占位页面的 `RouteRecordRaw[]`。
- Modify: `apps/web/src/router/index.ts` — 使用集中声明的路由记录创建 Router 实例。
- Create: `apps/web/src/views/PlaceholderView.vue` — 读取当前路由元信息并渲染统一占位页。
- Create: `apps/web/src/router/routes.test.ts` — 验证目标路径、路由名和页面标题。

### Task 1: Add a failing route contract test

**Files:**
- Modify: `apps/web/package.json`
- Modify: `pnpm-lock.yaml`
- Create: `apps/web/src/router/routes.test.ts`

**Interfaces:**
- Consumes: future `apps/web/src/router/routes.ts` export `routes: RouteRecordRaw[]`.
- Produces: a runnable `pnpm --filter @iot-pillot/web test` command and an executable contract for all target routes.

- [ ] **Step 1: Add the test command and Vitest dev dependency**

Add the following entries to `apps/web/package.json` without changing existing scripts or dependencies:

```json
{
  "scripts": {
    "test": "vitest run"
  },
  "devDependencies": {
    "vitest": "^2.1.9"
  }
}
```

Run `pnpm install` from the repository root so `pnpm-lock.yaml` records the workspace dependency.

- [ ] **Step 2: Write the failing test**

Create `apps/web/src/router/routes.test.ts` with this contract:

```ts
import { describe, expect, it } from "vitest";
import { routes } from "./routes";

describe("application routes", () => {
  it("registers each planned placeholder page with its title", () => {
    const placeholderRoutes = routes
      .filter((route) => route.path !== "/")
      .map((route) => ({
        path: route.path,
        name: route.name,
        title: route.meta?.title,
      }));

    expect(placeholderRoutes).toEqual([
      { path: "/login", name: "login", title: "登录" },
      { path: "/dashboard", name: "dashboard", title: "控制台" },
      { path: "/forms", name: "forms", title: "表单管理" },
      { path: "/recruitment", name: "recruitment", title: "招聘管理" },
      { path: "/templates", name: "templates", title: "邮件模板" },
      { path: "/settings", name: "settings", title: "系统设置" },
    ]);
  });
});
```

- [ ] **Step 3: Run the test and verify the expected RED failure**

Run:

```bash
pnpm --filter @iot-pillot/web test -- src/router/routes.test.ts
```

Expected: Vitest starts but fails because `apps/web/src/router/routes.ts` does not exist yet. This confirms the test is exercising the missing route contract rather than passing accidentally.

### Task 2: Implement the shared placeholder route records

**Files:**
- Create: `apps/web/src/router/route-specs.ts`
- Create: `apps/web/src/router/routes.ts`
- Create: `apps/web/src/views/PlaceholderView.vue`

**Interfaces:**
- Consumes: the route specifications from `apps/web/src/router/route-specs.ts`, `route.meta.title` from Vue Router 4, and the existing Element Plus auto-registration.
- Produces: `routes: RouteRecordRaw[]`, containing `/` plus the six named placeholder records, and a reusable view for all six records.

- [ ] **Step 1: Add the minimal placeholder view**

Create `apps/web/src/views/PlaceholderView.vue`:

```vue
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
```

- [ ] **Step 2: Declare the route specifications**

Create `apps/web/src/router/route-specs.ts`:

```ts
export const placeholderRouteSpecs = [
  { path: "/login", name: "login", title: "登录" },
  { path: "/dashboard", name: "dashboard", title: "控制台" },
  { path: "/forms", name: "forms", title: "表单管理" },
  { path: "/recruitment", name: "recruitment", title: "招聘管理" },
  { path: "/templates", name: "templates", title: "邮件模板" },
  { path: "/settings", name: "settings", title: "系统设置" },
] as const;
```

- [ ] **Step 3: Declare the route records**

Create `apps/web/src/router/routes.ts`:

```ts
import type { RouteRecordRaw } from "vue-router";
import Home from "@/views/Home.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import { placeholderRouteSpecs } from "./route-specs";

export const routes: RouteRecordRaw[] = [
  { path: "/", name: "home", component: Home },
  ...placeholderRouteSpecs.map(({ path, name, title }) => ({
    path,
    name,
    component: PlaceholderView,
    meta: { title },
  })),
];
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```bash
pnpm --filter @iot-pillot/web test -- src/router/routes.test.ts
```

Expected: one test passes and Vitest reports no failures.

### Task 3: Connect the router instance to the shared records

**Files:**
- Modify: `apps/web/src/router/index.ts`

**Interfaces:**
- Consumes: `routes` from `apps/web/src/router/routes.ts`.
- Produces: the same default Router export consumed by `apps/web/src/main.ts`.

- [ ] **Step 1: Replace inline records with the shared route list**

Update `apps/web/src/router/index.ts` to:

```ts
import { createRouter, createWebHistory } from "vue-router";
import { routes } from "./routes";

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
```

- [ ] **Step 2: Run the focused route test again**

Run:

```bash
pnpm --filter @iot-pillot/web test -- src/router/routes.test.ts
```

Expected: the route contract remains green after the router instance starts consuming the shared records.

### Task 4: Run project verification

**Files:**
- No additional files; verify all files from Tasks 1–3.

**Interfaces:**
- Consumes: the completed frontend route implementation.
- Produces: verified type-safe and production-buildable frontend output.

- [ ] **Step 1: Run the complete frontend test command**

Run `pnpm --filter @iot-pillot/web test`.

Expected: Vitest completes successfully with one passing test and no warnings.

- [ ] **Step 2: Run frontend type checking**

Run `pnpm --filter @iot-pillot/web typecheck`.

Expected: `vue-tsc` exits with code 0 and reports no type errors.

- [ ] **Step 3: Run the production build**

Run `pnpm --filter @iot-pillot/web build`.

Expected: `vue-tsc -b && vite build` exits with code 0 and emits the frontend build output.

- [ ] **Step 4: Review the final diff**

Run `git diff --check` and `git status --short`.

Expected: no whitespace errors; only the planned frontend files and dependency lock changes are present, in addition to any pre-existing user modifications.

- [ ] **Step 5: Commit the implementation**

After verification passes, stage only the implementation files and dependency lock changes, then commit:

```bash
git add apps/web/package.json apps/web/src/router/routes.test.ts apps/web/src/router/route-specs.ts apps/web/src/router/routes.ts apps/web/src/router/index.ts apps/web/src/views/PlaceholderView.vue pnpm-lock.yaml
git commit -m "feat: add placeholder page routes"
```
