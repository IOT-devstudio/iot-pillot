/**
 * 工作室项目管理的路由（#198）。
 *
 * 沿用仓库既有的「扁平路由 + meta.adminShell + AdminLayout」风格。
 * allowRoles 必须是 MEMBER_ONLY（= ["admin"]）：工作室运营首期**仅 admin**
 * （ADR 0001 §权限拍板），projects 也没有 member 只读入口。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import StudioProjectsView from "./views/StudioProjectsView.vue";

export const studioProjectRoutes: RouteRecordRaw[] = [
  {
    path: "/studio/projects",
    name: "studio-projects",
    component: StudioProjectsView,
    meta: { title: "项目管理", allowRoles: MEMBER_ONLY, adminShell: true },
  },
];
