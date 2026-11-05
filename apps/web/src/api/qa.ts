/**
 * 招新问答 API（#219，只读）。
 *
 * 后端契约：/api/v1/forum/posts（登录用户可读招新问答）。
 * 网络层保持后端 snake_case。本期只有读：列表 + 详情，无写接口。
 *
 * 内部论坛（studio scope）只挂管理端分组，前端本模块不接入（ADR 0003 决策 3：
 * 开放给成员只需把路由挪分组，不在本期范围）。
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
export function listQaPosts(query: QaPostQuery = {}): Promise<QaPostListResp> {
  const params = new URLSearchParams({
    page: String(query.page ?? 1),
    page_size: String(query.page_size ?? 20),
  });
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.direction !== undefined && query.direction !== null) {
    params.set("direction", query.direction);
  }
  return requestWithSession<QaPostListResp>(
    `/api/v1/forum/posts?${params.toString()}`,
  );
}

/** 读取单个帖子详情（正文 + 回复，官方回答已置顶）。 */
export function fetchQaPost(id: number): Promise<QaPostDetail> {
  return requestWithSession<QaPostDetail>(`/api/v1/forum/posts/${id}`);
}

/* ------------------------------------------------------------------ *
 * 写入（#220）
 * ------------------------------------------------------------------ */

/** 新建帖子。新建后回复列表为空，后端直接回详情结构。 */
export function createQaPost(payload: QaCreatePostReq): Promise<QaPostDetail> {
  return requestWithSession<QaPostDetail>("/api/v1/forum/posts", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

/** 编辑帖子。仅作者本人或管理员可用；后端对无权与不存在一律回 404。 */
export function updateQaPost(
  id: number,
  payload: QaUpdatePostReq,
): Promise<QaPostDetail> {
  return requestWithSession<QaPostDetail>(`/api/v1/forum/posts/${id}`, {
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
export function deleteQaPost(id: number): Promise<string> {
  return requestMessageWithSession(`/api/v1/forum/posts/${id}`, {
    method: "DELETE",
  });
}

/** 新建回复。 */
export function createQaReply(
  postId: number,
  payload: QaReplyReq,
): Promise<QaReply> {
  return requestWithSession<QaReply>(`/api/v1/forum/posts/${postId}/replies`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

/** 编辑回复。编辑不触发邮件通知（ADR 0007）。 */
export function updateQaReply(
  replyId: number,
  payload: QaReplyReq,
): Promise<QaReply> {
  return requestWithSession<QaReply>(`/api/v1/forum/replies/${replyId}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

/** 软删回复。同样走 message 出口（成功时 data 为 null）。 */
export function deleteQaReply(replyId: number): Promise<string> {
  return requestMessageWithSession(`/api/v1/forum/replies/${replyId}`, {
    method: "DELETE",
  });
}
