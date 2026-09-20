/**
 * recruitment 模块的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 * 招新档案属于管理侧功能区，与 /user/* 用户侧对称。
 *
 * meta.allowRoles 由 router/guards.ts 把关；守卫把「没写 allowRoles」当成
 * 公开路由放行，所以这里每一条真实页面都必须显式锁上成员角色。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import ProspectDetailView from "./views/ProspectDetailView.vue";
import ProspectsView from "./views/ProspectsView.vue";

export const recruitmentRoutes: RouteRecordRaw[] = [
  { path: "/recruitment", redirect: "/recruitment/prospects" },
  {
    path: "/recruitment/prospects",
    name: "recruitment-prospects",
    component: ProspectsView,
    meta: { title: "意向成员", allowRoles: MEMBER_ONLY },
  },
  {
    path: "/recruitment/prospects/:id",
    name: "recruitment-prospect-detail",
    component: ProspectDetailView,
    meta: { title: "意向成员详情", allowRoles: MEMBER_ONLY },
  },
];
