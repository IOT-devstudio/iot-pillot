/**
 * user-settings 模块（用户侧）的路由。
 *
 * 由 router/routes.ts 汇总，模块自身不直接改全局路由表。
 * 用户侧统一走 /user/* 前缀，与管理侧 /admin/* 对称。
 *
 * meta.allowRoles 由 router/guards.ts 把关。这里两个角色都列上，因为
 * 工作室成员也能访问用户侧页面（访问规则是单向的）。个人设置是对所有人
 * 开放的：用户填自己的资料、管理员也可以填自己的资料；管理员「代改他人
 * 资料」不在本期范围。
 *
 * 前置依赖：#57 后端 PUT /me 落地。本 issue 仅完成前端路由与占位视图，
 * 后续 commit 在本分支上继续叠加。
 */
import type { RouteRecordRaw } from "vue-router";

import { ANY_SIGNED_IN } from "@/router/route-specs";
import UserSettingsView from "./views/UserSettingsView.vue";

export const userSettingsRoutes: RouteRecordRaw[] = [
  {
    path: "/user/settings",
    name: "user-settings",
    component: UserSettingsView,
    meta: { title: "个人设置", allowRoles: ANY_SIGNED_IN },
  },
];