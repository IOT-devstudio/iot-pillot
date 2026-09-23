/**
 * 管理员用户列表的 detail 字段假数据。
 *
 * 现状：后端 #57 还没合并，`GET /admin/users` 的 items 不带 detail。
 * 前端为了让「详情弹窗」（issue #58）能展示完整字段，按 user_id 派生
 * fixture 数据。#57 落地后这两个函数应当整体移除，items 自带 detail。
 *
 * 注意：本文件**不是**用来在生产里造数据的；只是开发/演示阶段让
 * 弹窗有东西可看。删除条件 = 后端 `/admin/users` 与 `/admin/admins`
 * 的 items 已包含 detail 字段（见 #57 验收）。
 */
import type { AdminUser } from "@/api/admin";

const FRONT_END: AdminUser["detail"] = {
  class: "物联网工程 2301",
  student_id: 2023114514,
  qq: "1044696157",
  direction: "front-end",
  email: "liao.wenxuan@example.edu.cn",
};

const BACK_END: AdminUser["detail"] = {
  class: "计算机科学与技术 2202",
  student_id: 2022110033,
  qq: "99887766",
  direction: "back-end",
  email: "former200715@example.edu.cn",
};

const AGENT: AdminUser["detail"] = {
  class: "软件工程 2401",
  student_id: 2024099888,
  qq: "55556666",
  direction: "agent",
  email: "agent.dev@example.edu.cn",
};

const FALLBACK: AdminUser["detail"] = {
  class: "",
  student_id: undefined,
  qq: "",
  direction: null,
  email: "",
};

/**
 * 给一行用户补 detail：演示模式下避免弹窗里空空荡荡。
 *
 * 派生规则不是凭身份猜内容，是按 user_id 取模给一个稳定的方向
 * （同一行每次返回一样），方便 review diff 时不跳来跳去。整页「清一色」
 * 看起来比 mock 一个看上去像真人的字段更诚实。
 */
export function fillDemoDetail(user: AdminUser): AdminUser {
  if (user.detail !== undefined) {
    return user;
  }
  const slot = user.user_id % 3;
  let detail: AdminUser["detail"];
  if (slot === 0) {
    detail = FRONT_END;
  } else if (slot === 1) {
    detail = BACK_END;
  } else {
    detail = AGENT;
  }
  // 演示模式下把整页 detail 都返回：避免「一半有、一半空」的诡异状态。
  if (user.detail === undefined) {
    return { ...user, detail };
  }
  return user;
}

/**
 * 「未填」档位：用于刚注册 / 没补资料的账号。等 #57 落地后这种 case
 * 后端会真实给空字符串 / null —— 现在先用 user_id 命中特殊值来演示。
 */
export function fillDemoDetailWithFallbacks(user: AdminUser): AdminUser {
  if (user.detail !== undefined) {
    return user;
  }
  if (user.user_id % 7 === 0) {
    return { ...user, detail: FALLBACK };
  }
  return fillDemoDetail(user);
}