/**
 * 招新问答的路由（#219）。
 *
 * 用户侧页面：登录用户都能进（招新问答面向全体成员/报名者，ANY_SIGNED_IN）。
 * 沿用仓库「扁平路由 + meta.allowRoles + 用户侧 /user 前缀」风格。
 *
 * 内部论坛（studio scope）仅管理员可见，前端本模块不接入。
 */
import type { RouteRecordRaw } from "vue-router";

import { ANY_SIGNED_IN } from "@/router/route-specs";
import QaListView from "./views/QaListView.vue";
import QaDetailView from "./views/QaDetailView.vue";

export const forumRoutes: RouteRecordRaw[] = [
  {
    path: "/user/qa",
    name: "qa-list",
    component: QaListView,
    meta: { title: "招新问答", allowRoles: ANY_SIGNED_IN },
  },
  {
    path: "/user/qa/:id",
    name: "qa-detail",
    component: QaDetailView,
    meta: { title: "问答详情", allowRoles: ANY_SIGNED_IN },
  },
];
