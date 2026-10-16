// 招新领域类型：prospect（意向新生）→ candidate（候选人）→ hired/rejected。
// 时间戳统一用 ISO-8601 字符串（common.ISODateTime）。
//
// 注意：本文件同时承载两个层次：
//   1. 后端 #88 已实现的报名申请模型（AppliedApplication/Direction/Round）——前端 #89 可对接
//   2. 后端 #90 已实现的邀请与候选人 DTO（number ID）——前端 #91 对接
//   3. 文件末尾保留的旧 Prospect/Candidate 草案（string ID）——不用于实际接口

import type { ID, ISODateTime } from "./common";

/* ------------------------------------------------------------------ *
 * 后端 #88 报名申请模型（已实现）
 * ------------------------------------------------------------------ */

/** 报名申请状态机：applied → reviewing → accepted | rejected */
export type AppliedApplicationStatus =
  "applied" | "reviewing" | "accepted" | "rejected";

/**
 * 方向的面试表单定位信息（后端 #135 交付）。
 *
 * 只给三个值：点「面试」跳哪张表（share_token）、该表当前开不开放（is_open）。
 * 表单定义本身由填写页按 share_token 自己读，不在这里重复下发。
 *
 * share_token 是定位符、不是凭据：读取与提交都要求登录（#92 裁决）。
 */
export interface RecruitmentDirectionForm {
  id: number;
  share_token: string;
  is_open: boolean;
}

/** 招新方向（可报名白名单：front-end / back-end / agent） */
export interface RecruitmentDirection {
  id: number;
  code: "front-end" | "back-end" | "agent";
  name: string;
  open: boolean;
  sort_order: number;
  /**
   * 该方向的面试表单；无开放表单时为 null。
   *
   * 后端序列化的是 JSON null（不是空对象），消费方必须按 null 分支降级：
   * 面试按钮置灰并给出原因文案，不能直接读 form.share_token。
   */
  form: RecruitmentDirectionForm | null;
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

/* ------------------------------------------------------------------ *
 * BE-04 已交付接口（apps/api/internal/dto/{request,response}/recruitment_invitation_*.go）
 * ID 一律 number：后端主键是 int，禁止用通用 ID(string) 掩盖差异。
 * 时间戳是 RFC3339 字符串；"未发生"（如未接受）序列化为空串，故不用 ISODateTime?。
 * ------------------------------------------------------------------ */

/** 后端 domain.InvitationStatus：pending → accepted | revoked；过期是查询期判定，不是存储状态。 */
export type InvitationStatus = "pending" | "accepted" | "revoked";

/** POST /api/v1/admin/invitations/send */
export interface SendInvitationInput {
  application_id: number;
  /** invitation 类型模板 id；类型不符后端报业务错误 */
  template_id: number;
  /** 有效天数；缺省 7，上限 90 */
  expires_in_days?: number;
  vars?: Record<string, string>;
}

/** 管理端邀请视图。token 不进管理端列表；接受凭据只出现在本人邀请里。 */
export interface InvitationView {
  id: number;
  application_id: number;
  status: InvitationStatus;
  expires_at: string;
  /** 空串 = 邮件从未投递成功（不算已邀请，允许重试投递） */
  email_sent_at: string;
  sent_by: number;
  accepted_at: string;
  created_at: string;
}

/** GET /api/v1/admin/invitations */
export interface InvitationListView {
  items: InvitationView[];
  total: number;
  page: number;
  page_size: number;
}

/** GET /api/v1/me/invitations 单条目：本人邀请（携带接受凭据）。 */
export interface MyInvitationView {
  application_id: number;
  direction: string;
  status: InvitationStatus;
  /** 服务端按查询时刻判定，前端不自行比对时间 */
  expired: boolean;
  token: string;
  expires_at: string;
  accepted_at: string;
  created_at: string;
}

export interface MyInvitationListView {
  items: MyInvitationView[];
  total: number;
}

/** POST /api/v1/me/invitations/accept */
export interface AcceptInvitationInput {
  token: string;
}

/** 接受成功的响应 data：直接给出本人候选人视图。 */
export interface AcceptInvitationView {
  candidate: MyCandidateView;
}

/** 管理端候选人视图（列表/详情内嵌）。 */
export interface CandidateView {
  id: number;
  application_id: number;
  user_id: number;
  name: string;
  email: string;
  direction: string;
  status: CandidateStatus;
  created_at: string;
  updated_at: string;
}

/** GET /api/v1/admin/candidates */
export interface CandidateListView {
  items: CandidateView[];
  total: number;
  page: number;
  page_size: number;
}

/** 候选人时间线一行（GET /api/v1/admin/candidates/:id 的 timeline 条目）。 */
export interface CandidateTimelineView {
  /** 初始行（进入面试）from_status 为空串 */
  from_status: string;
  to_status: CandidateStatus;
  note: string;
  operator_id: number;
  created_at: string;
}

export interface CandidateDetailView {
  candidate: CandidateView;
  timeline: CandidateTimelineView[];
}

/** POST /api/v1/admin/candidates/:id/advance */
export interface AdvanceCandidateInput {
  action: "send_offer" | "hire" | "reject";
  /** send_offer / reject 必填 */
  template_id?: number;
  note?: string;
  vars?: Record<string, string>;
}

/** GET /api/v1/me/candidates 单条目：本人候选人记录。 */
export interface MyCandidateView {
  application_id: number;
  direction: string;
  status: CandidateStatus;
  created_at: string;
  updated_at: string;
}

export interface MyCandidateListView {
  items: MyCandidateView[];
  total: number;
}

/* ------------------------------------------------------------------ *
 * 以下为设计草案（后端未实现，勿当已交付 API）
 * ------------------------------------------------------------------ */

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
