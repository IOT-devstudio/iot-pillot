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
  QaPostDetail,
  QaPostListResp,
  QaPostQuery,
  QaPostSummary,
} from "@iot-pillot/shared-types";
import { requestWithSession } from "./session-request";

// 直接复用共享包的契约类型，不在这里复制一份：
// 后端字段一旦变化，typecheck 会在这一层就报出来。
export type {
  QaPostDetail,
  QaPostListResp,
  QaPostQuery,
  QaPostSummary,
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
