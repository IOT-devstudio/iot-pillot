/**
 * 工作室运营域 DTO 草案（Studio Ops）。
 *
 * 对齐 docs/adr/0001-studio-member-project-model.md 与
 * docs/superpowers/specs/2026-10-27-studio-ops-check-report.md。
 *
 * 注意：
 * - ID 为 number（与现有 Go 后端一致），不是 common.ID 的 UUID 字符串。
 * - 本文件是契约草案：后端 handler 落地前，前端不得伪装为已联调。
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

export interface CreateStudioMemberInput {
  name: string;
  birth_year: number;
  cohort: string;
  direction: StudioDirectionCode;
  role_in_studio: string;
  internship?: string;
  employment?: string;
  user_id?: number | null;
  is_active?: boolean;
}

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

/** hired 候选人 → 工作室成员 */
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

export interface CreateStudioKnowledgeInput {
  title: string;
  body: string;
  is_public?: boolean;
  permission?: StudioKnowledgePermission;
  parent_id?: number | null;
}

export interface UpdateStudioKnowledgeInput {
  title?: string;
  body?: string;
  is_public?: boolean;
  permission?: StudioKnowledgePermission;
  parent_id?: number | null;
}
