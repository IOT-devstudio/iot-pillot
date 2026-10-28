/**
 * 工作室项目与参与 API（#198）。
 *
 * 后端契约：/api/v1/admin/studio/projects*，全部仅管理员可访问。
 * 网络层保持后端 snake_case（与 api/studio-members.ts 一致）。
 */
import type {
  AddStudioProjectMemberInput,
  CreateStudioProjectInput,
  StudioProject,
  StudioProjectDetail,
  StudioProjectMember,
  StudioProjectMemberList,
  StudioProjectMemberRole,
  StudioProjectPage,
  StudioProjectStatus,
  UpdateStudioProjectInput,
  UpdateStudioProjectMemberInput,
} from "@iot-pillot/shared-types";
import { requestMessageWithSession, requestWithSession } from "./session-request";

// 直接复用共享包的契约类型，不在这里复制一份：
// 后端字段一旦变化，typecheck 会在这一层就报出来，而不是等页面渲染时才炸。
export type {
  AddStudioProjectMemberInput,
  CreateStudioProjectInput,
  StudioProject,
  StudioProjectDetail,
  StudioProjectMember,
  StudioProjectMemberList,
  StudioProjectMemberRole,
  StudioProjectPage,
  StudioProjectStatus,
  UpdateStudioProjectInput,
  UpdateStudioProjectMemberInput,
};

/** 列表筛选条件；缺省项不下发（后端按「不筛选」处理）。 */
export interface StudioProjectFilters {
  q?: string;
  status?: StudioProjectStatus | "";
  year?: string;
}

const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

/** 分页读取项目列表。 */
export function listStudioProjects(
  filters: StudioProjectFilters = {},
  page = 1,
  pageSize = 20,
): Promise<StudioProjectPage> {
  const query = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (filters.q?.trim()) query.set("q", filters.q.trim());
  if (filters.status) query.set("status", filters.status);
  if (filters.year?.trim()) query.set("year", filters.year.trim());
  return requestWithSession<StudioProjectPage>(
    `/api/v1/admin/studio/projects?${query.toString()}`,
  );
}

/** 读取项目详情（元数据 + 完整参与名单）。 */
export function fetchStudioProject(id: number): Promise<StudioProjectDetail> {
  return requestWithSession<StudioProjectDetail>(
    `/api/v1/admin/studio/projects/${id}`,
  );
}

/** 创建项目。项目名全局唯一，冲突由后端返回 400。 */
export function createStudioProject(
  input: CreateStudioProjectInput,
): Promise<StudioProject> {
  return requestWithSession<StudioProject>(
    "/api/v1/admin/studio/projects",
    json("POST", input),
  );
}

/** 更新项目元数据（补丁语义）。 */
export function updateStudioProject(
  id: number,
  input: UpdateStudioProjectInput,
): Promise<StudioProject> {
  return requestWithSession<StudioProject>(
    `/api/v1/admin/studio/projects/${id}`,
    json("PUT", input),
  );
}

/**
 * 删除项目（参与行由后端级联删除）。
 *
 * 后端返回 OKWithMsg(..., nil)，data 被省略 —— 走 message 出口（同 #196 的删除）。
 */
export function deleteStudioProject(id: number): Promise<string> {
  return requestMessageWithSession(`/api/v1/admin/studio/projects/${id}`, {
    method: "DELETE",
  });
}

/** 参与名单（按开始时间升序）。 */
export function listStudioProjectMembers(
  projectId: number,
): Promise<StudioProjectMemberList> {
  return requestWithSession<StudioProjectMemberList>(
    `/api/v1/admin/studio/projects/${projectId}/members`,
  );
}

/**
 * 添加参与。
 *
 * name 是**姓名快照**必填：不从成员档案自动取。这样成员以后改名，
 * 历史参与仍显示当时的名字；同时外部参与者（没有成员档案）也能记录。
 */
export function addStudioProjectMember(
  projectId: number,
  input: AddStudioProjectMemberInput,
): Promise<StudioProjectMember> {
  return requestWithSession<StudioProjectMember>(
    `/api/v1/admin/studio/projects/${projectId}/members`,
    json("POST", input),
  );
}

/**
 * 更新参与行（补丁语义）。
 *
 * ended_at 三态：不传 = 不改；传 null = 仍在参与；传时间 = 已结束。
 * member_id 同理：不传 = 不改关联；传 null = 解除关联；传数字 = 关联该成员。
 */
export function updateStudioProjectMember(
  projectId: number,
  memberId: number,
  input: UpdateStudioProjectMemberInput,
): Promise<StudioProjectMember> {
  return requestWithSession<StudioProjectMember>(
    `/api/v1/admin/studio/projects/${projectId}/members/${memberId}`,
    json("PUT", input),
  );
}

/** 移除参与行（不影响成员档案本身）。 */
export function removeStudioProjectMember(
  projectId: number,
  memberId: number,
): Promise<string> {
  return requestMessageWithSession(
    `/api/v1/admin/studio/projects/${projectId}/members/${memberId}`,
    { method: "DELETE" },
  );
}

