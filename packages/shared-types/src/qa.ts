// 招新问答 / 内部论坛（#219）的共享 DTO。
//
// 本期只读：列表 + 详情 + 方向筛选 + 分页。写入（发帖/回复/软删/标记已解决）
// 属于 #220/#221，本文件不定义任何写请求体。
//
// 命名规则：网络层保持后端 snake_case（与 recruitment.ts / studio.ts 一致）；
// ID 用 number（后端已落地，未迁 UUID，见 CLAUDE.md「前端开发计划」M0 约定）。
// 时间戳统一 ISO-8601 字符串。

import type { ID, ISODateTime } from "./common";

/** 内容域：招新问答（recruitment）或内部论坛（studio）。 */
export type ForumScope = "recruitment" | "studio";

/** 列表项摘要（不渲染正文，只给元信息与摘要）。 */
export interface QaPostSummary {
  id: number;
  scope: ForumScope;
  title: string;
  author_id: number;
  author_name: string;
  /** 关联招新方向码；空方向的帖子该字段为 null（ADR 0003 决策 7：NULL 与空方向不同）。 */
  direction_code: string | null;
  /** 正文全文，下发给前端由 markdownExcerpt 生成纯文本预览（列表不渲染 Markdown VNode）。 */
  body: string;
  reply_count: number;
  /** 正文是否含图片（列表里用图标提示，不渲染图）。 */
  has_image: boolean;
  is_resolved: boolean;
  /** 最后活跃时间：最新回复时间，没有回复则取发帖时间。 */
  last_active_at: ISODateTime;
  created_at: ISODateTime;
}

/** 分页列表响应：分页信息在 data 内（与后端 Result 无 meta 约定一致）。 */
export interface QaPostListResp {
  items: QaPostSummary[];
  total: number;
  page: number;
  page_size: number;
}

/** 一条回复。 */
export interface QaReply {
  id: number;
  post_id: number;
  author_id: number;
  body: string;
  /** 是否官方回答：作者为管理员时由后端实时置顶（ADR 0003 决策 8）。 */
  is_official: boolean;
  edited_at: ISODateTime | null;
  created_at: ISODateTime;
}

/** 帖子详情：正文 + 回复（官方回答已置顶）。 */
export interface QaPostDetail {
  id: number;
  scope: ForumScope;
  title: string;
  author_id: number;
  direction_code: string | null;
  body: string;
  is_resolved: boolean;
  resolved_at: ISODateTime | null;
  edited_at: ISODateTime | null;
  created_at: ISODateTime;
  replies: QaReply[];
}

/** 列表筛选参数（前端 → 查询串）。 */
export interface QaPostQuery {
  q?: string;
  /** 方向码筛选；缺省为全部（含空方向），不传 direction 参数。 */
  direction?: string | null;
  page?: number;
  page_size?: number;
}

// 复用既有 ID / ISODateTime 导出，避免消费方再多 import 一层。
export type { ID, ISODateTime };
