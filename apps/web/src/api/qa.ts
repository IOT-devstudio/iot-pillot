/**
 * 招新问答 API（#219，只读）。
 *
 * 后端契约：/api/v1/forum/posts（登录用户可读招新问答）。
 * 网络层保持后端 snake_case。本期只有读：列表 + 详情，无写接口。
 *
 * 两个内容域（#221）用同一套函数，靠 scope 参数切路径前缀：
 *   recruitment → /api/v1/forum/...        （登录即可）
 *   studio      → /api/v1/studio/forum/... （仅管理员，普通成员 404）
 * 不拆成两份 API 文件：接口形状完全一致，拆开只会让页面多一个分支；
 * 域差异只有「路径前缀」和「谁能进」，前者集中在这里，后者由路由 meta 与后端把关。
 */
import type {
  QaCreatePostReq,
  QaPostDetail,
  QaPostListResp,
  QaPostQuery,
  QaPostSummary,
  QaReply,
  QaReplyReq,
  QaUpdatePostReq,
} from "@iot-pillot/shared-types";
import { requestMessageWithSession, requestWithSession } from "./session-request";

/** 内容域（与路由 routes.ts 的 ForumScope 同值域）。 */
export type QaScope = "recruitment" | "studio";

/**
 * 按内容域给出接口前缀。
 *
 * 后端两个域是**不同的路径**（/api/v1/forum 与 /api/v1/studio/forum），
 * 不是同一端点加参数，所以这里必须真切路径。
 */
function apiBase(scope: QaScope = "recruitment"): string {
  return scope === "studio" ? "/api/v1/studio/forum" : "/api/v1/forum";
}

// 直接复用共享包的契约类型，不在这里复制一份：
// 后端字段一旦变化，typecheck 会在这一层就报出来。
export type {
  QaCreatePostReq,
  QaPostDetail,
  QaPostListResp,
  QaPostQuery,
  QaPostSummary,
  QaReply,
  QaReplyReq,
  QaUpdatePostReq,
};

/**
 * 分页列出招新问答帖子。
 *
 * direction 三态（对齐后端「缺省=全部」语义）：
 *   undefined / null  = 不按方向筛选（空方向帖子也出现，见 ADR 0003 决策 7）；
 *   string            = 只看该方向（direction_code = ? 自然排除 NULL）。
 * 用 undefined/null 区分「没传」与「传了空串」：空串在 query 里与「没传」不可分，
 * 而这两件事必须分开。
 */
export function listQaPosts(
  query: QaPostQuery = {},
  scope: QaScope = "recruitment",
): Promise<QaPostListResp> {
  const params = new URLSearchParams({
    page: String(query.page ?? 1),
    page_size: String(query.page_size ?? 20),
  });
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.direction !== undefined && query.direction !== null) {
    params.set("direction", query.direction);
  }
  // 已解决三态：只有明确 true/false 才发参数，null 表示不筛选。
  if (query.resolved !== undefined && query.resolved !== null) {
    params.set("resolved", String(query.resolved));
  }
  return requestWithSession<QaPostListResp>(
    `${apiBase(scope)}/posts?${params.toString()}`,
  );
}

/** 读取单个帖子详情（正文 + 回复，官方回答已置顶）。 */
export function fetchQaPost(
  id: number,
  scope: QaScope = "recruitment",
): Promise<QaPostDetail> {
  return requestWithSession<QaPostDetail>(`${apiBase(scope)}/posts/${id}`);
}

/* ------------------------------------------------------------------ *
 * 写入（#220）
 * ------------------------------------------------------------------ */

/** 新建帖子。新建后回复列表为空，后端直接回详情结构。 */
export function createQaPost(
  payload: QaCreatePostReq,
  scope: QaScope = "recruitment",
): Promise<QaPostDetail> {
  return requestWithSession<QaPostDetail>(`${apiBase(scope)}/posts`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

/** 编辑帖子。仅作者本人或管理员可用；后端对无权与不存在一律回 404。 */
export function updateQaPost(
  id: number,
  payload: QaUpdatePostReq,
  scope: QaScope = "recruitment",
): Promise<QaPostDetail> {
  return requestWithSession<QaPostDetail>(`${apiBase(scope)}/posts/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

/**
 * 软删帖子。
 *
 * 后端成功时 data 为 null（Result 的 omitempty 会丢掉 data 字段），
 * 走 message 出口而不是 dataOf：按 data 缺失当失败会弹出假错误。
 */
export function deleteQaPost(
  id: number,
  scope: QaScope = "recruitment",
): Promise<string> {
  return requestMessageWithSession(`${apiBase(scope)}/posts/${id}`, {
    method: "DELETE",
  });
}

/** 新建回复。 */
export function createQaReply(
  postId: number,
  payload: QaReplyReq,
  scope: QaScope = "recruitment",
): Promise<QaReply> {
  return requestWithSession<QaReply>(`${apiBase(scope)}/posts/${postId}/replies`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

/** 编辑回复。编辑不触发邮件通知（ADR 0007）。 */
export function updateQaReply(
  replyId: number,
  payload: QaReplyReq,
  scope: QaScope = "recruitment",
): Promise<QaReply> {
  return requestWithSession<QaReply>(`${apiBase(scope)}/replies/${replyId}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

/** 软删回复。同样走 message 出口（成功时 data 为 null）。 */
export function deleteQaReply(
  replyId: number,
  scope: QaScope = "recruitment",
): Promise<string> {
  return requestMessageWithSession(`${apiBase(scope)}/replies/${replyId}`, {
    method: "DELETE",
  });
}

/* ------------------------------------------------------------------ *
 * 已解决标记（#221）
 * ------------------------------------------------------------------ */

/**
 * 标记或取消「已解决」。
 *
 * 后端回完整详情（含回复），前端可直接替换页面状态；
 * 幂等：重复标记同一方向不报错。
 */
export function resolveQaPost(
  id: number,
  resolved: boolean,
  scope: QaScope = "recruitment",
): Promise<QaPostDetail> {
  return requestWithSession<QaPostDetail>(`${apiBase(scope)}/posts/${id}/resolve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ resolved }),
  });
}
