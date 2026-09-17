/**
 * home 模块（用户侧）的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 * 用户侧统一走 /user/* 前缀，与管理侧 /admin/* 对称。
 */
import type { RouteRecordRaw } from "vue-router";

import UserHomeView from "./views/UserHomeView.vue";

export const homeRoutes: RouteRecordRaw[] = [
  {
    path: "/user/home",
    name: "user-home",
    component: UserHomeView,
    meta: { title: "首页" },
  },
];
