/**
 * recruitment 模块的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 */
import type { RouteRecordRaw } from "vue-router";

import ProspectDetailView from "./views/ProspectDetailView.vue";
import ProspectsView from "./views/ProspectsView.vue";

export const recruitmentRoutes: RouteRecordRaw[] = [
  { path: "/recruitment", redirect: "/recruitment/prospects" },
  { path: "/recruitment/prospects", name: "recruitment-prospects", component: ProspectsView, meta: { title: "意向成员" } },
  { path: "/recruitment/prospects/:id", name: "recruitment-prospect-detail", component: ProspectDetailView, meta: { title: "意向成员详情" } },
];
