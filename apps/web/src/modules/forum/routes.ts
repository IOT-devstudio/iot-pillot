/**
 * 招新问答的路由（#219 只读、#220 写入）。
 *
 * 用户侧页面：登录用户都能进（招新问答面向全体成员/报名者，ANY_SIGNED_IN）。
 * 沿用仓库「扁平路由 + meta.allowRoles + 用户侧 /user 前缀」风格。
 *
 * 内部论坛（studio scope）仅管理员可见，前端本模块不接入。
 *
 * 路由顺序：`/user/qa/new` 必须排 `/user/qa/:id` **之前**，
 * 否则 "new" 会被当成 id 匹配进详情页。
 */
import type { RouteRecordRaw } from "vue-router";

import { ANY_SIGNED_IN } from "@/router/route-specs";
import QaListView from "./views/QaListView.vue";
import QaDetailView from "./views/QaDetailView.vue";
import QaComposeView from "./views/QaComposeView.vue";

export const forumRoutes: RouteRecordRaw[] = [
  {
    path: "/user/qa",
    name: "qa-list",
    component: QaListView,
    meta: { title: "招新问答", allowRoles: ANY_SIGNED_IN },
  },
  {
    path: "/user/qa/new",
    name: "qa-new",
    component: QaComposeView,
    meta: { title: "提问", allowRoles: ANY_SIGNED_IN },
  },
  {
    path: "/user/qa/:id/edit",
    name: "qa-edit",
    component: QaComposeView,
    meta: { title: "编辑帖子", allowRoles: ANY_SIGNED_IN },
  },
  {
    path: "/user/qa/:id",
    name: "qa-detail",
    component: QaDetailView,
    meta: { title: "问答详情", allowRoles: ANY_SIGNED_IN },
  },
];
