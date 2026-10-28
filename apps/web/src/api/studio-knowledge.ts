/**
 * 工作室知识库 API（#196）。
 *
 * 后端契约：/api/v1/admin/studio/knowledge*，全部仅管理员可访问。
 * 网络层保持后端 snake_case（与 api/studio-members.ts 一致）。
 *
 * 保存语义：每次 PUT 都是「保存新版本」——后端把 version +1 并写入快照。
 * **没有**回滚端点：恢复 = 读回旧版内容，再保存一版。
 */
import type {
  CreateStudioKnowledgeInput,
  StudioKnowledgeEntry,
  StudioKnowledgeEntryPage,
  StudioKnowledgePermission,
  StudioKnowledgeRevision,
  StudioKnowledgeRevisionList,
  UpdateStudioKnowledgeInput,
} from "@iot-pillot/shared-types";
import { requestWithSession } from "./session-request";

// 直接复用共享包的契约类型，不在这里复制一份：
// 后端字段一旦变化，typecheck 会在这一层就报出来，而不是等页面渲染时才炸。
export type {
  CreateStudioKnowledgeInput,
  StudioKnowledgeEntry,
  StudioKnowledgeEntryPage,
  StudioKnowledgePermission,
  StudioKnowledgeRevision,
  StudioKnowledgeRevisionList,
  UpdateStudioKnowledgeInput,
};

/**
 * 列表筛选条件；缺省项不下发（后端按「不筛选」处理）。
 *
 * parent_id 三态：
 *   undefined  = 不按父子筛选（返回全部，含子条目）
 *   null       = 只看顶层
 *   number     = 只看该条目的直接子条目
 *
 * 用 undefined/null/number 三分而不是空串：空串在 query 里与「没传」
 * 不可区分，而这两件事必须分开。
 */
export interface StudioKnowledgeFilters {
  q?: string;
  permission?: StudioKnowledgePermission | "";
  parent_id?: number | null;
}

const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

/** 分页读取知识库条目。 */
export function listStudioKnowledge(
  filters: StudioKnowledgeFilters = {},
  page = 1,
  pageSize = 20,
): Promise<StudioKnowledgeEntryPage> {
  const query = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (filters.q?.trim()) query.set("q", filters.q.trim());
  if (filters.permission) query.set("permission", filters.permission);
  // 三态必须显式判断：null（只看顶层）是真实筛选，
  // 写成 if (filters.parent_id) 会被静默丢掉。
  if (filters.parent_id !== undefined) {
    query.set("parent_id", filters.parent_id === null ? "none" : String(filters.parent_id));
  }
  return requestWithSession<StudioKnowledgeEntryPage>(
    `/api/v1/admin/studio/knowledge?${query.toString()}`,
  );
}

/** 读取单个条目的当前正文。 */
export function fetchStudioKnowledge(id: number): Promise<StudioKnowledgeEntry> {
  return requestWithSession<StudioKnowledgeEntry>(
    `/api/v1/admin/studio/knowledge/${id}`,
  );
}

/** 创建条目（后端同时写入 v1 修订）。 */
export function createStudioKnowledge(
  input: CreateStudioKnowledgeInput,
): Promise<StudioKnowledgeEntry> {
  return requestWithSession<StudioKnowledgeEntry>(
    "/api/v1/admin/studio/knowledge",
    json("POST", input),
  );
}

/**
 * 保存条目：版本 +1 并写入新修订。
 *
 * version 必须带上（乐观锁）。不带会让后端退化成「总是覆盖」，
 * 两个管理员同时编辑时后保存的会静默抹掉前一个人的改动。
 */
export function updateStudioKnowledge(
  id: number,
  input: UpdateStudioKnowledgeInput,
): Promise<StudioKnowledgeEntry> {
  return requestWithSession<StudioKnowledgeEntry>(
    `/api/v1/admin/studio/knowledge/${id}`,
    json("PUT", input),
  );
}

/** 删除条目（修订由后端级联删除，子条目的 parent 置空）。 */
export function deleteStudioKnowledge(id: number): Promise<void> {
  return requestWithSession<void>(`/api/v1/admin/studio/knowledge/${id}`, {
    method: "DELETE",
  });
}

/** 修订列表（版本倒序）。 */
export function listStudioKnowledgeRevisions(
  id: number,
): Promise<StudioKnowledgeRevisionList> {
  return requestWithSession<StudioKnowledgeRevisionList>(
    `/api/v1/admin/studio/knowledge/${id}/revisions`,
  );
}

/** 取指定版本快照（用于「看看上一版写了什么」）。 */
export function fetchStudioKnowledgeRevision(
  id: number,
  version: number,
): Promise<StudioKnowledgeRevision> {
  return requestWithSession<StudioKnowledgeRevision>(
    `/api/v1/admin/studio/knowledge/${id}/revisions/${version}`,
  );
}
