/**
 * shared 模块的路由：所有登录用户都能访问的功能区。
 *
 * 与成员页的区别在 meta.allowRoles：这里同时列出 admin 和 member，
 * 因为访问规则是单向的——工作室成员也可以进入普通用户页面。
 */
import type { RouteRecordRaw } from "vue-router";

import MemberView from "./views/MemberView.vue";

/** 个人页允许的角色。成员能进普通用户页，所以两个角色都列上。 */
export const ANY_SIGNED_IN = ["admin", "member"] as const;

export const sharedRoutes: RouteRecordRaw[] = [
  {
    path: "/member",
    name: "member",
    component: MemberView,
    meta: { title: "个人中心", allowRoles: ANY_SIGNED_IN },
  },
];
