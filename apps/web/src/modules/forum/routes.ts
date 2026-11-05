/**
 * 招新问答的路由（#219 只读、#220 写入、#221 内部论坛分区）。
 *
 * 用户侧页面：登录用户都能进（招新问答面向全体成员/报名者，ANY_SIGNED_IN）。
 * 沿用仓库「扁平路由 + meta.allowRoles + 用户侧 /user 前缀」风格。
 *
 * 内部论坛（#221）**复用同一批组件**，靠路由 meta 切换内容域，不复制一份实现
 * （AC 明确要求）。组件通过 `useForumScope()` 读当前域，不认路径。
 *
 * 路由顺序：`/user/qa/new` 必须排 `/user/qa/:id` **之前**，
 * 否则 "new" 会被当成 id 匹配进详情页。
 */
import type { RouteRecordRaw } from "vue-router";

import { ANY_SIGNED_IN, MEMBER_ONLY } from "@/router/route-specs";
import QaListView from "./views/QaListView.vue";
import QaDetailView from "./views/QaDetailView.vue";
import QaComposeView from "./views/QaComposeView.vue";

/** 路由 meta 里标记内容域的键。 */
export const FORUM_SCOPE_META = "forumScope";

/** 内容域：招新问答 / 工作室内部论坛。 */
export type ForumScope = "recruitment" | "studio";

export const forumRoutes: RouteRecordRaw[] = [
  {
    path: "/user/qa",
    name: "qa-list",
    component: QaListView,
    meta: {
      title: "招新问答",
      allowRoles: ANY_SIGNED_IN,
      [FORUM_SCOPE_META]: "recruitment",
    },
  },
  {
    path: "/user/qa/new",
    name: "qa-new",
    component: QaComposeView,
    meta: {
      title: "提问",
      allowRoles: ANY_SIGNED_IN,
      [FORUM_SCOPE_META]: "recruitment",
    },
  },
  {
    path: "/user/qa/:id/edit",
    name: "qa-edit",
    component: QaComposeView,
    meta: {
      title: "编辑帖子",
      allowRoles: ANY_SIGNED_IN,
      [FORUM_SCOPE_META]: "recruitment",
    },
  },
  {
    path: "/user/qa/:id",
    name: "qa-detail",
    component: QaDetailView,
    meta: {
      title: "问答详情",
      allowRoles: ANY_SIGNED_IN,
      [FORUM_SCOPE_META]: "recruitment",
    },
  },

  // ── 内部论坛（#221）：同一批组件，仅管理员可见 ──
  // 用独立的 /studio/forum 前缀而不是 /user/qa?scope=studio：
  // 路径本身能让人一眼看出这是内部区，收藏与分享也不会把两种域混在一起。
  // 隐藏菜单不等于鉴权——后端对普通成员一律 404。
  {
    path: "/studio/forum",
    name: "studio-forum-list",
    component: QaListView,
    meta: {
      title: "内部论坛",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
    },
  },
  {
    path: "/studio/forum/new",
    name: "studio-forum-new",
    component: QaComposeView,
    meta: {
      title: "发内部帖",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
    },
  },
  {
    path: "/studio/forum/:id/edit",
    name: "studio-forum-edit",
    component: QaComposeView,
    meta: {
      title: "编辑内部帖",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
    },
  },
  {
    path: "/studio/forum/:id",
    name: "studio-forum-detail",
    component: QaDetailView,
    meta: {
      title: "内部帖详情",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
    },
  },
];
