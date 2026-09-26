/**
 * 当前登录用户的详细资料假数据。
 *
 * 现状：后端 #57 还没合并，`GET /api/v1/me` 只透 user_id / username / role，
 * 不返 detail。前端为了让 home 页的「我的信息」能展示方向（issue #58），
 * 按 user_id 派生 fixture。#57 落地后这两个函数应当整体移除，
 * 后端 /me 直接返 detail。
 *
 * 删除条件 = 后端 /me 已包含 detail 字段（见 #57 验收）。
 *
 * 与 dashboard/adminUserFixtures.ts 的关系：派生规则同源（user_id % 3 / % 7），
 * 但**分两个文件维护**——adminUserFixtures 处理的是「管理员列表行」，入参是
 * AdminUser；这里处理的是「当前登录用户自己」，入参是 CurrentUser。共享会让两边
 * 都难懂，且 home 模块不应该反向依赖 dashboard 模块。
 */
import type { CurrentUser } from "@/api/admin";
import type { MyProfile } from "@/api/profile";

const FRONT_END: MyProfile["detail"] = {
  class: "物联网工程 2301",
  student_id: 2023114514,
  qq: "1044696157",
  direction: "front-end",
  email: "liao.wenxuan@example.edu.cn",
};

const BACK_END: MyProfile["detail"] = {
  class: "计算机科学与技术 2202",
  student_id: 2022110033,
  qq: "99887766",
  direction: "back-end",
  email: "former200715@example.edu.cn",
};

const AGENT: MyProfile["detail"] = {
  class: "软件工程 2401",
  student_id: 2024099888,
  qq: "55556666",
  direction: "agent",
  email: "agent.dev@example.edu.cn",
};

/**
 * 「未填」档位：用于刚注册 / 没补资料的账号。等 #57 落地后这种 case
 * 后端会真实给空字符串 / null —— 现在先用 user_id 命中特殊值来演示。
 */
const FALLBACK: MyProfile["detail"] = {
  class: "",
  student_id: null,
  qq: "",
  direction: null,
  email: "",
};

/**
 * 给当前用户套上 fixture 的 detail：演示模式下避免卡片里空空荡荡。
 *
 * 派生规则按 user_id 取模给一个稳定的方向（同一用户每次返回一样），
 * 方便 review diff 时不跳来跳去。
 */
export function fillDemoProfile(user: CurrentUser): MyProfile {
  const slot = user.user_id % 3;
  const detail: MyProfile["detail"] =
    slot === 0 ? { ...FRONT_END } : slot === 1 ? { ...BACK_END } : { ...AGENT };
  return {
    user_id: user.user_id,
    username: user.username,
    role: user.role,
    detail,
  };
}

/**
 * 「未填」档位：用于刚注册 / 没补资料的账号。等 #57 落地后这种 case
 * 后端会真实给空字符串 / null —— 现在先用 user_id 命中特殊值来演示。
 */
export function fillDemoProfileWithFallbacks(user: CurrentUser): MyProfile {
  if (user.user_id % 7 === 0) {
    return { ...fillDemoProfile(user), detail: { ...FALLBACK } };
  }
  return fillDemoProfile(user);
}