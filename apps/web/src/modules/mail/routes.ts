/**
 * mail 模块（邮件中心）的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 * 邮件模板与发信记录含招新对象的联系方式，必须显式锁成员角色——
 * 守卫把「没写 allowRoles」当成公开路由放行（routes.test.ts 锁住这一点）。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import MailCenterView from "./views/MailCenterView.vue";

export const mailRoutes: RouteRecordRaw[] = [
  {
    path: "/templates",
    name: "templates",
    component: MailCenterView,
    meta: { title: "邮件中心", allowRoles: MEMBER_ONLY, adminShell: true },
  },
];
