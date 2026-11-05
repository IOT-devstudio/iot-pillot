// 招新问答 / 内部论坛（#219 只读、#220 写入）的共享 DTO。
//
// 读：列表 + 详情 + 方向筛选 + 分页。
// 写：新建/编辑/软删帖子与回复（#220 已落地）。
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
  /**
   * 已解决筛选（#221）。三态：
   * null/undefined = 全部（不传参数），true = 只看已解决，false = 只看未解决。
   * 不能用 false 表示「不筛选」，否则「只看未解决」无法表达。
   */
  resolved?: boolean | null;
  page?: number;
  page_size?: number;
}

// 复用既有 ID / ISODateTime 导出，避免消费方再多 import 一层。
export type { ID, ISODateTime };

/* ------------------------------------------------------------------ *
 * 写入请求体（#220）
 * ------------------------------------------------------------------ */

/**
 * 新建帖子请求体。
 *
 * direction_code 三态语义与后端一致：
 *   省略 / null          → 没选方向，服务端按「资料方向 → 最新报名 → 留空」回落
 *   ""（空串）           → 视同没选，同样走回落
 *   "front-end" 等具体值 → 以用户选择为准，不做回落
 *
 * scope 不在请求体里：它由路由分组决定，放进请求体会让用户能把招新帖写进内部论坛。
 */
export interface QaCreatePostReq {
  title: string;
  body: string;
  direction_code?: string | null;
}

/** 编辑帖子请求体（方向不可改）。 */
export interface QaUpdatePostReq {
  title: string;
  body: string;
}

/** 新建 / 编辑回复请求体（两者字段相同）。 */
export interface QaReplyReq {
  body: string;
}

/** 附件上传成功的响应（ADR 0004：URL 是相对地址，换域名不用改正文）。 */
export interface AttachmentUploadResp {
  id: number;
  /** 相对地址 /api/v1/attachments/{id}，正文里直接用这个写 Markdown。 */
  url: string;
  content_type: string;
  size_bytes: number;
}

/* ------------------------------------------------------------------ *
 * 学习路线（#223，ADR 0006）
 * ------------------------------------------------------------------ */

/** 阶段里的一个资源链接。 */
export interface RoadmapResource {
  title: string;
  url: string;
}

/**
 * 一个学习阶段。
 *
 * `id` 由**服务端**生成（`s_` + 8 位 base36），前端只当不透明字符串传回，
 * 绝不自造 —— 用户的学习进度记的是阶段 id，伪造 id 会造出指向不存在阶段的进度。
 * 新建/新增阶段时传空串，服务端会发一个新 id。
 */
export interface RoadmapStage {
  id: string;
  title: string;
  summary: string;
  resources: RoadmapResource[];
}

/** 列表项：不含正文与阶段标题（列表页不渲染正文）。 */
export interface RoadmapListItem {
  id: number;
  title: string;
  direction_code: string;
  /** 方向名；方向被清理过时回落成 code（可见的降级）。 */
  direction_name: string;
  stage_count: number;
  /** 正文摘要（字符截断，保留 Markdown 原文）；列表不返回全文。 */
  excerpt: string;
  updated_at: string;
}

/** 方向筛选项：只含有路线的方向，不看招新开关。 */
export interface RoadmapDirectionOption {
  code: string;
  name: string;
  count: number;
}

export interface RoadmapListResp {
  items: RoadmapListItem[];
  total: number;
  page: number;
  page_size: number;
  directions: RoadmapDirectionOption[];
}

export interface RoadmapDetail {
  id: number;
  title: string;
  body: string;
  direction_code: string;
  stages: RoadmapStage[];
  /**
   * 当前登录用户在这条路线上的当前阶段 id（#224）。
   * 空串表示还没标记。只属于本人——管理员看到的也是自己的进度。
   */
  my_stage_id: string;
  /** 乐观锁版本号：保存时必须原样带回。 */
  version: number;
  stage_count: number;
  created_at: string;
  updated_at: string;
}

/** 整体保存的请求体。`version` 必填（乐观锁）。 */
export interface RoadmapWritePayload {
  title: string;
  body: string;
  direction_code: string;
  stages: RoadmapStage[];
  version?: number;
}

/** 标记 / 撤销进度的结果（#224）。 */
export interface RoadmapProgressResp {
  roadmap_id: number;
  /** 标记后的当前阶段；空串表示已撤销。 */
  stage_id: string;
}

export interface RoadmapListQuery {
  direction?: string | null;
  page?: number;
  page_size?: number;
}
