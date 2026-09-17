import type { RouteRecordRaw } from "vue-router";
import { aboutRoutes } from "@/modules/about/routes";
import { homeRoutes } from "@/modules/home/routes";
import { openerRoutes } from "@/modules/opener/routes";
import { openerEditorRoutes } from "@/modules/opener-editor/routes";
import AuthView from "@/views/AuthView.vue";
import Home from "@/views/Home.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import { MEMBER_ONLY, placeholderRouteSpecs } from "./route-specs";

export const routes: RouteRecordRaw[] = [
  // 根路径 = 3D 开屏（opener 模块自带路由，在这里汇总）
  ...openerRoutes,
  // 管理端：楼栋布局可视化编辑器（meta.allowRoles 由 router/guards.ts 把关）
  ...openerEditorRoutes,
  // 用户侧（与管理侧同一个 router，靠 /user 与 /admin 前缀区分）
  ...homeRoutes,
  ...aboutRoutes,
  // 原根路径的健康检查页挪到 /home，实现保持不变
  { path: "/home", name: "home", component: Home },
  // 二维后备登录页：正常路径到不了这里，只有 3D 开屏渲染失败时由它转投。
  // 守卫的登录落点是 "/"（见 guards.ts），所以这里不需要任何链接入口。
  { path: "/login", name: "login", component: AuthView, meta: { title: "登录" } },
  // 后台管理页统一为成员专属；普通用户会被守卫带到 /user/home
  ...placeholderRouteSpecs.map(({ path, name, title }) => ({
    path,
    name,
    component: PlaceholderView,
    meta: { title, allowRoles: MEMBER_ONLY },
  })),
];
