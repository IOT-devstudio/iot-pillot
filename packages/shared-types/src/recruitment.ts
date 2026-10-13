// 招新领域类型：prospect（意向新生）→ candidate（候选人）→ hired/rejected。
// 时间戳统一用 ISO-8601 字符串（common.ISODateTime）。
//
// 注意：本文件同时承载两个层次：
//   1. 后端 #88 已实现的报名申请模型（AppliedApplication/Direction/Round）——前端 #89 可对接
//   2. 下游 #90 邀请流转模型（Prospect/Candidate）——尚未实现

import type { ID, ISODateTime } from "./common";

/* ------------------------------------------------------------------ *
 * 后端 #88 报名申请模型（已实现）
 * ------------------------------------------------------------------ */

/** 报名申请状态机：applied → reviewing → accepted | rejected */
export type AppliedApplicationStatus = "applied" | "reviewing" | "accepted" | "rejected";

/** 招新方向（可报名白名单：front-end / back-end / agent） */
export interface RecruitmentDirection {
  id: number;
  code: "front-end" | "back-end" | "agent";
  name: string;
  open: boolean;
  sort_order: number;
}

/** 招新轮次 */
export interface RecruitmentRound {
  id: number;
  round_num: number;
  name: string;
  accepting: boolean;
}

/** 报名申请 */
export interface AppliedApplication {
  id: number;
  user_id: number;
  round_id: number;
  direction_code: "front-end" | "back-end" | "agent";
  status: AppliedApplicationStatus;
  name: string;
  email: string;
  class: string;
  student_id: number;
  qq: string;
  note: string;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

/** 时间线一行 */
export interface AppliedApplicationTimeline {
  id: number;
  to_status: string;
  created_at: ISODateTime;
}

/** 申请详情（含时间线） */
export interface AppliedApplicationDetail {
  application: AppliedApplication;
  timeline: AppliedApplicationTimeline[];
}

/** 分页列表响应 */
export interface AppliedApplicationListResp {
  items: AppliedApplication[];
  total: number;
  page: number;
  page_size: number;
}

/* ------------------------------------------------------------------ *
 * 下游 #90 邀请流转模型（尚未实现，保留供契约参考）
 * ------------------------------------------------------------------ */

export type ProspectStatus = "pending" | "invited" | "accepted" | "rejected";
export type CandidateStatus = "interviewing" | "offered" | "hired" | "rejected";

export interface Prospect {
  id: ID;
  name?: string;
  email: string;
  status: ProspectStatus;
  sourceFormId?: ID;
  invitedAt?: ISODateTime;
  acceptedAt?: ISODateTime;
  createdAt: ISODateTime;
}

export interface Candidate {
  id: ID;
  prospectId: ID;
  name: string;
  email: string;
  status: CandidateStatus;
  notes?: string;
  offeredAt?: ISODateTime;
  hiredAt?: ISODateTime;
  rejectedAt?: ISODateTime;
  createdAt: ISODateTime;
}

export interface SendOfferInput {
  candidateId: ID;
  templateId: ID;
  customFields?: Record<string, string>;
}

export interface SendRejectionInput {
  candidateId: ID;
  templateId: ID;
  customFields?: Record<string, string>;
}
