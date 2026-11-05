/**
 * 学习路线路由（#223 管理端 + #224 用户端）。
 *
 * 管理端沿用「扁平路由 + meta.adminShell + AdminLayout」风格，
 * allowRoles = MEMBER_ONLY（= ["admin"]）：路线维护是运营动作。
 *
 * 用户端两个路由都标 ANY_SIGNED_IN（登录即可，含 admin）。
 * **不新增任何公开路由与未登录分支**（AC 明确要求）：游客从首页点进来
 * 直接被全局守卫拦去登录页，与 /user/settings、/user/qa 完全一致。
 */
import type { RouteRecordRaw } from "vue-router";

import { ANY_SIGNED_IN, MEMBER_ONLY } from "@/router/route-specs";
import LearningPathDetailView from "./views/LearningPathDetailView.vue";
import LearningPathsView from "./views/LearningPathsView.vue";
import LearningRoadmapView from "./views/LearningRoadmapView.vue";

export const learningRoadmapRoutes: RouteRecordRaw[] = [
  {
    path: "/studio/roadmaps",
    name: "studio-roadmaps",
    component: LearningRoadmapView,
    meta: { title: "学习路线", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/user/learning-paths",
    name: "learning-paths",
    component: LearningPathsView,
    meta: { title: "学习路线", allowRoles: ANY_SIGNED_IN },
  },
  {
    path: "/user/learning-paths/:id",
    name: "learning-path-detail",
    component: LearningPathDetailView,
    meta: { title: "学习路线详情", allowRoles: ANY_SIGNED_IN },
  },
];
