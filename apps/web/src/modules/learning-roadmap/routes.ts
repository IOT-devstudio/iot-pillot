/**
 * 学习路线管理端路由（#223）。
 *
 * 沿用仓库既有的「扁平路由 + meta.adminShell + AdminLayout」风格。
 * allowRoles 必须是 MEMBER_ONLY（= ["admin"]）：路线维护是运营动作，
 * 用户端只读入口在 #224 落地（路径 /user/learning-paths）。
 */
import type { RouteRecordRaw } from "vue-router";

import { MEMBER_ONLY } from "@/router/route-specs";
import LearningRoadmapView from "./views/LearningRoadmapView.vue";

export const learningRoadmapRoutes: RouteRecordRaw[] = [
  {
    path: "/studio/roadmaps",
    name: "studio-roadmaps",
    component: LearningRoadmapView,
    meta: { title: "学习路线", allowRoles: MEMBER_ONLY, adminShell: true },
  },
];
