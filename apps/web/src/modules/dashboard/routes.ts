/**
 * dashboard 模块（管理台）的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 *
 * meta.allowRoles 由 router/guards.ts 把关；守卫把「没写 allowRoles」当成
 * 公开路由放行，而管理台含后端健康状态与用户名单，必须显式锁上成员角色。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import AdminMembersView from "./views/AdminMembersView.vue";
import DashboardView from "./views/DashboardView.vue";

export const dashboardRoutes: RouteRecordRaw[] = [
  {
    path: "/dashboard",
    name: "dashboard",
    component: DashboardView,
    meta: { title: "控制台", allowRoles: MEMBER_ONLY },
  },
  // 「用户与权限」独立成页而不是塞进控制台：控制台只留健康状态与入口，
  // 管理动作集中到自己的页面，职责更清。同样是成员专属。
  {
    path: "/dashboard/admins",
    name: "dashboard-admins",
    component: AdminMembersView,
    meta: { title: "用户与权限", allowRoles: MEMBER_ONLY },
  },
];
