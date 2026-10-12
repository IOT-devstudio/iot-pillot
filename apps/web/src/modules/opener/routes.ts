/**
 * opener 模块的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 */
import type { RouteRecordRaw } from "vue-router";

import AuthView from "@/views/AuthView.vue";

export const openerRoutes: RouteRecordRaw[] = [
  // 根路径与 /login 共用瑞士信号 2D 认证界面（Issue #81）
  {
    path: "/",
    name: "opener",
    component: AuthView,
    meta: { title: "登录" },
  },
];
