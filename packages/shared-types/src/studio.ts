/**
 * 工作室运营域 DTO（Studio Ops）。
 *
 * 对齐 docs/adr/0001-studio-member-project-model.md 与
 * docs/superpowers/specs/2026-10-27-studio-ops-check-report.md。
 *
 * 注意：
 * - ID 为 number（与现有 Go 后端一致），不是 common.ID 的 UUID 字符串。
 * - **成员（StudioMember）已联调**（#195）：apps/api 的
 *   domain.StudioMember / dto.StudioMemberResp 与本文件的字段一一对应，
 *   端点 /api/v1/admin/studio/members*。改字段必须两边一起改。
 * - **知识库（Knowledge）已联调**（#196）：domain.StudioKnowledgeEntry /
 *   StudioKnowledgeRevision 与本文件一一对应，端点
 *   /api/v1/admin/studio/knowledge*。is_public 是 permission 的派生只读值。
 * - 项目仍是**草案**（#198 尚未实现），不得当作已交付 API。
 * - 首期路由仅 admin；permission 字段预留 member/public。
 */

import type { ISODateTime } from "./common";

// ── 成员 ──────────────────────────────────────────────────────────

export type StudioDirectionCode =
  | "front-end"
  | "back-end"
  | "agent"
  | "all"
  | "game"
  | "other"
  | string;

export interface StudioMember {
  id: number;
  user_id: number | null;
  name: string;
  birth_year: number;
  cohort: string;
  direction: StudioDirectionCode;
  role_in_studio: string;
  internship: string;
  employment: string;
  is_active: boolean;
  archived_at: ISODateTime | null;
  source_candidate_id: number | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

/**
 * 录入成员（POST /admin/studio/members）。
 *
 * 刻意**不含 is_active**：新建成员一律是在读状态，归档只能走 archive 端点。
 * 后端也不绑定这个字段，带上它只会被静默忽略。
 */
export interface CreateStudioMemberInput {
  name: string;
  birth_year: number;
  cohort: string;
  direction: StudioDirectionCode;
  role_in_studio: string;
  internship?: string;
  employment?: string;
  /** 缺省 = 无系统账号的成员（校友 / 外部人员）。 */
  user_id?: number;
}

/**
 * 更新成员（PUT /admin/studio/members/:id）：补丁语义，只传要改的项。
 *
 * user_id 是三态，必须用可选 + null 表达：
 *   不传（undefined）= 不改绑定；
 *   传 null         = 解除账号绑定；
 *   传 number       = 绑定该账号（后端会核对账号存在且未注销）。
 *
 * is_active=true 表示恢复在读，后端会同时清空 archived_at；
 * 归档本身不走这里，走 POST /:id/archive。
 */
export interface UpdateStudioMemberInput {
  name?: string;
  birth_year?: number;
  cohort?: string;
  direction?: StudioDirectionCode;
  role_in_studio?: string;
  internship?: string;
  employment?: string;
  user_id?: number | null;
  is_active?: boolean;
}

/** 成员列表分页（GET /admin/studio/members）。 */
export interface StudioMemberPage {
  items: StudioMember[];
  total: number;
  page: number;
  page_size: number;
}

/** hired 候选人 → 工作室成员（#197 尚未实现，草案） */
export interface ConvertCandidateToMemberInput {
  candidate_id: number;
  role_in_studio?: string;
  cohort?: string;
  birth_year?: number;
  direction?: StudioDirectionCode;
  internship?: string;
  employment?: string;
}

// ── 项目 ──────────────────────────────────────────────────────────

export type StudioProjectStatus = "active" | "completed" | "archived";
export type StudioProjectMemberRole = "lead" | "contributor";

export interface StudioProjectMember {
  id: number;
  project_id: number;
  member_id: number | null;
  name: string;
  role: StudioProjectMemberRole;
  started_at: ISODateTime;
  ended_at: ISODateTime | null;
  created_at: ISODateTime;
}

export interface StudioProject {
  id: number;
  name: string;
  year: string;
  summary: string;
  status: StudioProjectStatus;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

export interface StudioProjectDetail extends StudioProject {
  members: StudioProjectMember[];
}

export interface CreateStudioProjectInput {
  name: string;
  year: string;
  summary: string;
  status?: StudioProjectStatus;
}

export interface UpdateStudioProjectInput {
  name?: string;
  year?: string;
  summary?: string;
  status?: StudioProjectStatus;
}

export interface AddStudioProjectMemberInput {
  member_id?: number | null;
  name: string;
  role: StudioProjectMemberRole;
  started_at?: ISODateTime;
  ended_at?: ISODateTime | null;
}

export interface UpdateStudioProjectMemberInput {
  name?: string;
  role?: StudioProjectMemberRole;
  started_at?: ISODateTime;
  ended_at?: ISODateTime | null;
  member_id?: number | null;
}

// ── 知识库 ────────────────────────────────────────────────────────

export type StudioKnowledgePermission = "public" | "member" | "admin";

export interface StudioKnowledgeEntry {
  id: number;
  title: string;
  body: string;
  author_id: number;
  is_public: boolean;
  permission: StudioKnowledgePermission;
  parent_id: number | null;
  version: number;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

export interface StudioKnowledgeRevision {
  id: number;
  entry_id: number;
  version: number;
  title: string;
  body: string;
  editor_id: number;
  created_at: ISODateTime;
}

/**
 * 创建知识库条目（POST /admin/studio/knowledge）。
 *
 * 刻意**不含 is_public**：它是 permission 的派生值（后端按
 * permission === "public" 算出），请求里带它只会被静默忽略。
 * author_id 同理不接受——作者一律取自登录身份，防止「替别人署名」。
 */
export interface CreateStudioKnowledgeInput {
  title: string;
  body?: string;
  /** 缺省按 admin（与迁移 0015 的列默认值一致）。 */
  permission?: StudioKnowledgePermission;
  /** 可选树形归属；缺省 = 顶层条目。 */
  parent_id?: number | null;
}

/**
 * 保存知识库条目（PUT /admin/studio/knowledge/:id）：补丁语义。
 *
 * parent_id 是三态，必须用可选 + null 表达：
 *   不传（undefined）= 不改父子关系；
 *   传 null         = 移到顶层；
 *   传 number       = 挂到该条目下。
 *
 * version 是乐观锁：带上你读到的版本号，被他人抢先保存时后端返回冲突。
 * 不传会退化成「总是覆盖」，并发编辑会静默丢改动。
 */
export interface UpdateStudioKnowledgeInput {
  title?: string;
  body?: string;
  permission?: StudioKnowledgePermission;
  parent_id?: number | null;
  version?: number;
}

/** 知识库条目分页（GET /admin/studio/knowledge）。 */
export interface StudioKnowledgeEntryPage {
  items: StudioKnowledgeEntry[];
  total: number;
  page: number;
  page_size: number;
}

/** 修订列表（GET /admin/studio/knowledge/:id/revisions，版本倒序）。 */
export interface StudioKnowledgeRevisionList {
  items: StudioKnowledgeRevision[];
  total: number;
}
