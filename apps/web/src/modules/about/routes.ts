/**
 * about 模块（用户侧）的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 * 用户侧统一走 /user/* 前缀，与管理侧 /admin/* 对称。
 */
import type { RouteRecordRaw } from "vue-router";

import AboutView from "./views/AboutView.vue";

export const aboutRoutes: RouteRecordRaw[] = [
  {
    path: "/user/about",
    name: "about",
    component: AboutView,
    meta: { title: "关于" },
  },
];
