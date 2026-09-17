/**
 * opener 模块的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 */
import type { RouteRecordRaw } from "vue-router";

import StudioOpener from "./views/StudioOpener.vue";

export const openerRoutes: RouteRecordRaw[] = [
  // 3D 开屏（登录/注册面板浮在 Three.js 场景上）。根路径让给内容首页后，挪到 /opener。
  {
    path: "/opener",
    name: "opener",
    component: StudioOpener,
    meta: { title: "开屏" },
  },
];
