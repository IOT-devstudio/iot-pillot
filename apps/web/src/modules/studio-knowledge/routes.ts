/**
 * 工作室知识库的路由（#196）。
 *
 * 沿用仓库既有的「扁平路由 + meta.adminShell + AdminLayout」风格。
 * allowRoles 必须是 MEMBER_ONLY（= ["admin"]）：知识库首期**仅 admin**
 * （ADR 0001 §3），permission 字段可写但不为 member/public 挂读取入口。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import StudioKnowledgeView from "./views/StudioKnowledgeView.vue";

export const studioKnowledgeRoutes: RouteRecordRaw[] = [
  {
    path: "/studio/knowledge",
    name: "studio-knowledge",
    component: StudioKnowledgeView,
    meta: { title: "知识库", allowRoles: MEMBER_ONLY, adminShell: true },
  },
];
