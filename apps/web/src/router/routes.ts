import type { RouteRecordRaw } from "vue-router";
import { openerRoutes } from "@/modules/opener/routes";
import { openerEditorRoutes } from "@/modules/opener-editor/routes";
import AuthView from "@/views/AuthView.vue";
import Home from "@/views/Home.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import { placeholderRouteSpecs } from "./route-specs";

export const routes: RouteRecordRaw[] = [
  // 根路径 = 3D 开屏（opener 模块自带路由，在这里汇总）
  ...openerRoutes,
  // 管理端：楼栋布局可视化编辑器（meta.requiresAdmin 由 router/guards.ts 把关）
  ...openerEditorRoutes,
  // 原根路径的健康检查页挪到 /home，实现保持不变
  { path: "/home", name: "home", component: Home },
  // 纯表单版登录页保留为后备入口（3D 不可用或想看对照时用）
  { path: "/login", name: "login", component: AuthView, meta: { title: "登录" } },
  ...placeholderRouteSpecs
    .filter(({ path }) => path !== "/login")
    .map(({ path, name, title }) => ({
      path,
      name,
      component: PlaceholderView,
      meta: { title },
    })),
];
