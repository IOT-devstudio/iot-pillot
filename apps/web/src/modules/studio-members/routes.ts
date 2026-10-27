/**
 * 工作室成员名录的路由（#195）。
 *
 * 沿用仓库既有的「扁平路由 + meta.adminShell + AdminLayout」风格。
 * allowRoles 必须是 MEMBER_ONLY（= ["admin"]）：工作室运营首期**仅 admin**
 * （ADR 0001 §权限拍板），member 只读与游客公开接口都不在本轮范围。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import StudioMembersView from "./views/StudioMembersView.vue";

export const studioMembersRoutes: RouteRecordRaw[] = [
  {
    path: "/studio/members",
    name: "studio-members",
    component: StudioMembersView,
    meta: { title: "成员名录", allowRoles: MEMBER_ONLY, adminShell: true },
  },
];
