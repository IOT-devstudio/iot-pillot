import type { RouteRecordRaw } from "vue-router";
import { openerRoutes } from "@/modules/opener/routes";
import { openerEditorRoutes } from "@/modules/opener-editor/routes";
import { recruitmentRoutes } from "@/modules/recruitment/routes";
import AuthView from "@/views/AuthView.vue";
import Home from "@/views/Home.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import { placeholderRouteSpecs } from "./route-specs";

export const routes: RouteRecordRaw[] = [
  // 根路径 = 内容首页（健康检查 + 控制台入口），3D 开屏改到 /opener
  { path: "/", name: "home", component: Home },
  // 3D 开屏（opener 模块自带路由，在这里汇总）
  ...openerRoutes,
  // 管理端：楼栋布局可视化编辑器（meta.requiresAdmin 由 router/guards.ts 把关）
  ...openerEditorRoutes,
  // 招新流程（意向成员 / 候选人 / 详情）
  ...recruitmentRoutes,
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
