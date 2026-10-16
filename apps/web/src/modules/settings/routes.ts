/**
 * settings 模块（管理设置中心）的路由（issue #59）。
 *
 * IA 结论（见 #58 评论）：「用户与权限」管的是谁能管理系统（RBAC），
 * 本质上是系统级设置的一节，不是独立内容模块——并入设置中心，
 * 顶栏不再为它单开一项。
 *
 * 路由结构沿用仓库既有的「扁平路由 + meta.adminShell + AdminLayout」
 * 风格（全项目无嵌套 children 先例，票内方案也注明可调整）：
 *   /settings        → 索引重定向到第一个真实区块（避免空落点）
 *   /settings/users  → 用户与权限（现 AdminMembersView，组件不动只改路径）
 * 后续区块（SMTP 发件配置、注册开关……）等各自的内容 issue 落地时
 * 在这里追加，**不做占位导航**（吸取「待开发卡片当导航噪音」的教训）。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
// 组件暂留 dashboard 模块（#59 只迁路由不迁文件，减小 diff）；
// 迁移组件属纯搬运，等设置中心第二个区块落地时一并整理。
import AdminMembersView from "@/modules/dashboard/views/AdminMembersView.vue";

export const settingsRoutes: RouteRecordRaw[] = [
  // 索引落到第一个真实区块；将来新区块就位后再调整「第一个」是谁
  { path: "/settings", redirect: "/settings/users" },
  {
    path: "/settings/users",
    name: "settings-users",
    component: AdminMembersView,
    // 漏标 allowRoles = 门禁静默失效（routes.test 锁住）；
    // adminShell 让页面挂进 AdminLayout 外壳，不重新引入独立顶栏。
    meta: { title: "用户与权限", allowRoles: MEMBER_ONLY, adminShell: true },
  },
];
