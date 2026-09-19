/**
 * home 模块（用户侧）的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 * 用户侧统一走 /user/* 前缀，与管理侧 /admin/* 对称。
 *
 * meta.allowRoles 由 router/guards.ts 把关；这里两个角色都列上，因为
 * 工作室成员也能访问用户侧页面（访问规则是单向的）。
 */
import type { RouteRecordRaw } from "vue-router";

import { ANY_SIGNED_IN } from "@/router/route-specs";
import UserHomeView from "./views/UserHomeView.vue";

export const homeRoutes: RouteRecordRaw[] = [
  {
    path: "/user/home",
    name: "user-home",
    component: UserHomeView,
    meta: { title: "首页", allowRoles: ANY_SIGNED_IN },
  },
];
