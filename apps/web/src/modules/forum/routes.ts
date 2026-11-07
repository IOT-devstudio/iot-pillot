/**
 * 招新问答的路由（#219 只读、#220 写入、#221 内部论坛分区、#242 管理端入口）。
 *
 * 三组入口共用同一批组件：
 *   - 用户端 `/user/qa*`：登录用户都能进（ANY_SIGNED_IN），套瑞士信号壳（signalShell），
 *     提问、回答、编辑、删除、标记已解决都在这里；
 *   - 管理端 `/recruitment/qa*`：MEMBER_ONLY + adminShell，只负责「回答问题」，
 *     列表页不提供「提问」入口（管理员要发帖走用户端）；
 *   - 内部论坛 `/studio/forum*`：MEMBER_ONLY + adminShell，普通成员后端一律 404。
 *
 * **组件不认路径**：`useForumScope()` 从 meta 读内容域（forumScope）与路径前缀
 * （forumBase）。#242 之前路径前缀是按域硬编码的，管理端详情页的
 * 「返回列表 / 编辑后跳转 / 发布后跳转」会把管理员送回 /user/qa，跳出管理台外壳；
 * 现在由 forumBase 显式声明，同一域可以有多组前缀。
 *
 * 路由顺序：`.../new` 必须排 `/:id` **之前**，否则 "new" 会被当成 id 匹配进详情页。
 */
import type { RouteRecordRaw } from "vue-router";

import { ANY_SIGNED_IN, MEMBER_ONLY } from "@/router/route-specs";
import { FORUM_BASE_META, FORUM_SCOPE_META } from "./scope-meta";
import QaListView from "./views/QaListView.vue";
import QaDetailView from "./views/QaDetailView.vue";
import QaComposeView from "./views/QaComposeView.vue";

// meta 键与内容域类型的唯一真源在 scope-meta.ts（避免纯逻辑单测被视图链拖累）；
// 这里原样再导出，保持 routes.ts 作为「本模块路由契约」的既有入口。
export { FORUM_BASE_META, FORUM_SCOPE_META } from "./scope-meta";
export type { ForumScope } from "./scope-meta";

/** 用户端招新问答前缀：登录即可，套瑞士信号壳。 */
const USER_QA_BASE = "/user/qa";
/** 管理端招新问答前缀：只回答问题，不提供发帖入口。 */
const ADMIN_QA_BASE = "/recruitment/qa";
/** 内部论坛前缀：仅管理员，留在管理台外壳里。 */
const STUDIO_FORUM_BASE = "/studio/forum";

export const forumRoutes: RouteRecordRaw[] = [
  // ── 用户端招新问答（#219/#220）：提问与查阅 ──
  {
    path: USER_QA_BASE,
    name: "qa-list",
    component: QaListView,
    meta: {
      title: "招新问答",
      allowRoles: ANY_SIGNED_IN,
      // #242：与 /user/home、/user/settings 用同一套壳与配色，
      // 不再裸渲染成「后台灰蓝卡片」。
      signalShell: true,
      [FORUM_SCOPE_META]: "recruitment",
      [FORUM_BASE_META]: USER_QA_BASE,
    },
  },
  {
    path: `${USER_QA_BASE}/new`,
    name: "qa-new",
    component: QaComposeView,
    meta: {
      title: "提问",
      allowRoles: ANY_SIGNED_IN,
      signalShell: true,
      [FORUM_SCOPE_META]: "recruitment",
      [FORUM_BASE_META]: USER_QA_BASE,
    },
  },
  {
    path: `${USER_QA_BASE}/:id/edit`,
    name: "qa-edit",
    component: QaComposeView,
    meta: {
      title: "编辑帖子",
      allowRoles: ANY_SIGNED_IN,
      signalShell: true,
      [FORUM_SCOPE_META]: "recruitment",
      [FORUM_BASE_META]: USER_QA_BASE,
    },
  },
  {
    path: `${USER_QA_BASE}/:id`,
    name: "qa-detail",
    component: QaDetailView,
    meta: {
      title: "问答详情",
      allowRoles: ANY_SIGNED_IN,
      signalShell: true,
      [FORUM_SCOPE_META]: "recruitment",
      [FORUM_BASE_META]: USER_QA_BASE,
    },
  },

  // ── 管理端招新问答（#242）：只回答问题 ──
  // 复用同一批组件（与 #221 同口径），靠 meta 切外壳与路径；
  // 列表页隐藏「提问」按钮：该页职责是回答，发帖走用户端。
  {
    path: ADMIN_QA_BASE,
    name: "recruitment-qa-list",
    component: QaListView,
    meta: {
      title: "招新问答",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "recruitment",
      [FORUM_BASE_META]: ADMIN_QA_BASE,
    },
  },
  // 管理端不需要「新建」入口，但详情 / 编辑仍要有：管理员在管理台里
  // 编辑自己的帖子后应当留在管理台，而不是被送进用户端。
  {
    path: `${ADMIN_QA_BASE}/:id/edit`,
    name: "recruitment-qa-edit",
    component: QaComposeView,
    meta: {
      title: "编辑帖子",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "recruitment",
      [FORUM_BASE_META]: ADMIN_QA_BASE,
    },
  },
  {
    path: `${ADMIN_QA_BASE}/:id`,
    name: "recruitment-qa-detail",
    component: QaDetailView,
    meta: {
      title: "问答详情",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "recruitment",
      [FORUM_BASE_META]: ADMIN_QA_BASE,
    },
  },

  // ── 内部论坛（#221）：同一批组件，仅管理员可见 ──
  // 用独立的 /studio/forum 前缀而不是 /user/qa?scope=studio：
  // 路径本身能让人一眼看出这是内部区，收藏与分享也不会把两种域混在一起。
  // 隐藏菜单不等于鉴权——后端对普通成员一律 404。
  {
    path: STUDIO_FORUM_BASE,
    name: "studio-forum-list",
    component: QaListView,
    meta: {
      title: "内部论坛",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
      [FORUM_BASE_META]: STUDIO_FORUM_BASE,
    },
  },
  {
    path: `${STUDIO_FORUM_BASE}/new`,
    name: "studio-forum-new",
    component: QaComposeView,
    meta: {
      title: "发内部帖",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
      [FORUM_BASE_META]: STUDIO_FORUM_BASE,
    },
  },
  {
    path: `${STUDIO_FORUM_BASE}/:id/edit`,
    name: "studio-forum-edit",
    component: QaComposeView,
    meta: {
      title: "编辑内部帖",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
      [FORUM_BASE_META]: STUDIO_FORUM_BASE,
    },
  },
  {
    path: `${STUDIO_FORUM_BASE}/:id`,
    name: "studio-forum-detail",
    component: QaDetailView,
    meta: {
      title: "内部帖详情",
      allowRoles: MEMBER_ONLY,
      adminShell: true,
      [FORUM_SCOPE_META]: "studio",
      [FORUM_BASE_META]: STUDIO_FORUM_BASE,
    },
  },
];
