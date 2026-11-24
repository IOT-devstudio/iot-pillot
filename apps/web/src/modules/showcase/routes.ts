/**
 * 作品审阅的管理端路由。
 *
 * 沿用「扁平路由 + meta.adminShell + AdminLayout」风格；
 * allowRoles = MEMBER_ONLY（= ["admin"]），审阅是管理员专属动作，
 * 后端同样在 admin 分组下做 403 拦截。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import AdminShowcaseView from "./views/AdminShowcaseView.vue";

export const adminShowcaseRoutes: RouteRecordRaw[] = [
  {
    path: "/studio/showcase",
    name: "studio-showcase",
    component: AdminShowcaseView,
    meta: { title: "作品审阅", allowRoles: MEMBER_ONLY, adminShell: true },
  },
];
