/**
 * 招新接口封装（issue #89 / FE-03，对应后端 #88/#97）。
 *
 * 契约来源 apps/api/internal/dto/{request,response}/recruitment_*.go：
 *   用户端（需登录，归属一律由令牌决定，body 不含 user_id）：
 *     GET  /api/v1/recruitment/directions            开放方向列表
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
 * 令牌与刷新交给 session-requester（统一 401 处理），本文件只关心路径与 DTO。
 */
import {
  requestMessageWithSession,
  requestWithSession,
} from "./session-request";

/** 申请状态机（与后端 domain.ApplicationStatus 一致） */
export type ApplicationStatus = "applied" | "reviewing" | "accepted" | "rejected";

/** 审核可流转到的目标（与后端 RecruitmentReviewReq 一致） */
export type ReviewTarget = Extract<ApplicationStatus, "reviewing" | "accepted" | "rejected">;

export interface RecruitmentDirection {
  id: number;
  code: string;
  name: string;
  open: boolean;
  sort_order: number;
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
export function fetchDirections(): Promise<{ items: RecruitmentDirection[] }> {
  return requestWithSession("/api/v1/recruitment/directions");
}

/**
 * 报名。成功后端返回完整申请对象；同一方向重复报名由后端拒绝
 * （唯一约束），前端应先用本人申请列表挡掉重复入口。
 */
export function submitApplication(
  directionCode: string,
): Promise<RecruitmentApplication> {
  return requestWithSession("/api/v1/recruitment/applications", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ direction_code: directionCode }),
  });
}

/** 本人申请列表（最新在前由后端保证） */
export function fetchMyApplications(): Promise<ApplicationListPage> {
  return requestWithSession("/api/v1/recruitment/applications");
}

/** 本人申请详情，含状态时间线（仅归属申请人，越权 403） */
export function fetchMyApplication(
  id: number,
): Promise<RecruitmentApplicationDetail> {
  return requestWithSession(`/api/v1/recruitment/applications/${id}`);
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
