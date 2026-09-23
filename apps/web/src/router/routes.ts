import type { RouteRecordRaw } from "vue-router";
import { aboutRoutes } from "@/modules/about/routes";
import { dashboardRoutes } from "@/modules/dashboard/routes";
import { homeRoutes } from "@/modules/home/routes";
import { mailRoutes } from "@/modules/mail/routes";
import { openerRoutes } from "@/modules/opener/routes";
import { openerEditorRoutes } from "@/modules/opener-editor/routes";
import { recruitmentRoutes } from "@/modules/recruitment/routes";
import AuthView from "@/views/AuthView.vue";
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
  // 招新流程（意向成员 / 详情），模块自带 meta.allowRoles
  ...recruitmentRoutes,
  // 管理台（dashboard 模块自带 meta.allowRoles）
  ...dashboardRoutes,
  // 邮件中心（模板 + 发信记录，mail 模块自带 meta.allowRoles）
  ...mailRoutes,
  // 内容首页已并入管理台，保留旧路径重定向，避免收藏夹与旧链接 404
  { path: "/home", redirect: "/dashboard" },
  // 二维后备登录页：正常路径到不了这里，只有 3D 开屏渲染失败时由它转投。
  // 守卫的登录落点是 "/"（见 guards.ts），所以这里不需要任何链接入口。
  { path: "/login", name: "login", component: AuthView, meta: { title: "登录" } },
  // 后台管理页统一为成员专属；普通用户会被守卫带到 /user/home。
  // /dashboard 已有真实页面，必须从占位清单里剔除。
  ...placeholderRouteSpecs
    .filter(({ path }) => path !== "/dashboard")
    .map(({ path, name, title }) => ({
    path,
    name,
    component: PlaceholderView,
    meta: { title, allowRoles: MEMBER_ONLY, adminShell: true },
    })),
];
