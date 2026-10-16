/**
 * 招新接口封装（issue #89 / FE-03，对应后端 #88/#97）。
 *
 * 契约来源 apps/api/internal/dto/{request,response}/recruitment_*.go：
 *   用户端（除方向列表外需登录，归属由令牌决定，body 不含 user_id）：
 *     GET  /api/v1/recruitment/directions            公开方向列表
 *     POST /api/v1/recruitment/applications          报名 {direction_code}
 *     GET  /api/v1/recruitment/applications          本人申请列表
 *     GET  /api/v1/recruitment/applications/:id      本人申请详情（含时间线）
 *   管理端（RequireAdmin）：
 *     GET   /api/v1/admin/recruitment/rounds              轮次列表
 *     PATCH /api/v1/admin/recruitment/rounds/:id          轮次开关 {accepting}
 *     PATCH /api/v1/admin/recruitment/directions/:code    方向开关 {open}
 *     GET   /api/v1/admin/recruitment/applications        全量申请（?status=）
 *     PATCH /api/v1/admin/recruitment/applications/:id/review  审核 {target}
 *
 * 默认用 session-requester 管理令牌；公开页可注入原地降为游客的请求器。
 */
import {
  requestMessageWithSession,
  requestWithSession,
  type SessionRequester,
} from "./session-request";
import { AuthRequestError, requestBackend, UNAVAILABLE_MESSAGE } from "./auth";

/** 申请状态机（与后端 domain.ApplicationStatus 一致） */
export type ApplicationStatus = "applied" | "reviewing" | "accepted" | "rejected";

/** 审核可流转到的目标（与后端 RecruitmentReviewReq 一致） */
export type ReviewTarget = Extract<ApplicationStatus, "reviewing" | "accepted" | "rejected">;

/** 方向绑定的面试表单（issue #136；由 #135 交付，未交付前字段缺省 = null） */
export interface DirectionForm {
  id: number;
  share_token: string;
  is_open: boolean;
}

export interface RecruitmentDirection {
  id: number;
  code: string;
  name: string;
  open: boolean;
  sort_order: number;
  /**
   * 面试表单；无表单时为 null（#136）。
   * 后端 #135 交付前字段缺省，按 null 处理（面试入口降级置灰）。
   */
  form?: DirectionForm | null;
}

export interface RecruitmentApplication {
  id: number;
  user_id: number;
  round_id: number;
  direction_code: string;
  status: ApplicationStatus;
  name: string;
  email: string;
  class: string;
  student_id: number;
  qq: string;
  note: string;
  created_at: string;
  updated_at: string;
}

export interface RecruitmentTimelineEntry {
  id: number;
  to_status: ApplicationStatus;
  created_at: string;
}

export interface RecruitmentApplicationDetail {
  application: RecruitmentApplication;
  timeline: RecruitmentTimelineEntry[];
}

export interface ApplicationListPage {
  items: RecruitmentApplication[];
  total: number;
  page: number;
  page_size: number;
}

export interface RecruitmentRound {
  id: number;
  round_num: number;
  name: string;
  accepting: boolean;
}

// ── 用户端 ──

/** 开放方向列表（含开放状态，按 sort_order 排序由后端保证） */
export async function fetchDirections(): Promise<{ items: RecruitmentDirection[] }> {
  const { body, status } = await requestBackend<{ items: RecruitmentDirection[] }>(
    "/api/v1/recruitment/directions",
    {},
  );
  if (body.data === undefined) {
    throw new AuthRequestError(UNAVAILABLE_MESSAGE, status);
  }
  return body.data;
}

/**
 * 报名。成功后端返回完整申请对象；同一方向重复报名由后端拒绝
 * （唯一约束），前端应先用本人申请列表挡掉重复入口。
 */
export function submitApplication(
  directionCode: string,
  request: SessionRequester = requestWithSession,
  expectedUserId?: number,
): Promise<RecruitmentApplication> {
  return request("/api/v1/recruitment/applications", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ direction_code: directionCode }),
  }, expectedUserId);
}

/** 本人申请列表（最新在前由后端保证） */
export function fetchMyApplications(
  request: SessionRequester = requestWithSession,
): Promise<ApplicationListPage> {
  return request("/api/v1/recruitment/applications");
}

/** 本人申请详情，含状态时间线（仅归属申请人，越权 403） */
export function fetchMyApplication(
  id: number,
  request: SessionRequester = requestWithSession,
): Promise<RecruitmentApplicationDetail> {
  return request(`/api/v1/recruitment/applications/${id}`);
}

// ── 管理端 ──

export function fetchAdminRounds(): Promise<{ items: RecruitmentRound[] }> {
  return requestWithSession("/api/v1/admin/recruitment/rounds");
}

export function setRoundAccepting(
  id: number,
  accepting: boolean,
): Promise<string> {
  return requestMessageWithSession(
    `/api/v1/admin/recruitment/rounds/${id}`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ accepting }),
    },
  );
}

export function setDirectionOpen(
  code: string,
  open: boolean,
): Promise<string> {
  return requestMessageWithSession(
    `/api/v1/admin/recruitment/directions/${code}`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ open }),
    },
  );
}

/** 全量申请列表；status 缺省为全部 */
export function fetchAdminApplications(
  status?: ApplicationStatus,
): Promise<ApplicationListPage> {
  const path = status
    ? `/api/v1/admin/recruitment/applications?status=${status}`
    : "/api/v1/admin/recruitment/applications";
  return requestWithSession(path);
}

/**
 * 审核流转：applied→reviewing，reviewing→accepted / rejected。
 * 非法目标由后端拒绝（400），前端只按当前状态展示合法动作。
 */
export function reviewApplication(
  id: number,
  target: ReviewTarget,
): Promise<string> {
  return requestMessageWithSession(
    `/api/v1/admin/recruitment/applications/${id}/review`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target }),
    },
  );
}
