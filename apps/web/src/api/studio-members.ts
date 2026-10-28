/**
 * 工作室成员名录 API（#195）。
 *
 * 后端契约：/api/v1/admin/studio/members*，全部仅管理员可访问。
 * 网络层保持后端 snake_case（与 api/forms.ts 一致），不做 camelCase 转换。
 */
import type {
  CreateStudioMemberInput,
  StudioMember,
  StudioMemberPage,
  UpdateStudioMemberInput,
} from "@iot-pillot/shared-types";
import { requestWithSession } from "./session-request";

// 直接复用共享包的契约类型，不在这里复制一份：
// 后端字段一旦变化，typecheck 会在这一层就报出来，而不是等页面渲染时才炸。
export type {
  CreateStudioMemberInput,
  StudioMember,
  StudioMemberPage,
  UpdateStudioMemberInput,
};

/** 列表筛选条件；缺省项不下发（后端按「不筛选」处理）。 */
export interface StudioMemberFilters {
  q?: string;
  direction?: string;
  /**
   * 在读状态三态：
   *   undefined = 全部（含已归档）
   *   true      = 仅在读
   *   false     = 仅已归档
   */
  is_active?: boolean;
}

const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

/** 分页读取成员名录。 */
export function listStudioMembers(
  filters: StudioMemberFilters = {},
  page = 1,
  pageSize = 20,
): Promise<StudioMemberPage> {
  const query = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (filters.q?.trim()) query.set("q", filters.q.trim());
  if (filters.direction?.trim()) query.set("direction", filters.direction.trim());
  // 三态必须显式判断 undefined，不能写 if (filters.is_active)——
  // false 是「仅已归档」这个真实筛选，写成真值判断会被静默丢掉。
  if (filters.is_active !== undefined) {
    query.set("is_active", String(filters.is_active));
  }
  return requestWithSession<StudioMemberPage>(
    `/api/v1/admin/studio/members?${query.toString()}`,
  );
}

/** 读取单个成员档案。 */
export function fetchStudioMember(id: number): Promise<StudioMember> {
  return requestWithSession<StudioMember>(`/api/v1/admin/studio/members/${id}`);
}

/** 录入成员（user_id 可空 = 无系统账号的校友/外部人员）。 */
export function createStudioMember(
  input: CreateStudioMemberInput,
): Promise<StudioMember> {
  return requestWithSession<StudioMember>(
    "/api/v1/admin/studio/members",
    json("POST", input),
  );
}

/** 补丁式更新：只传要改的项。 */
export function updateStudioMember(
  id: number,
  input: UpdateStudioMemberInput,
): Promise<StudioMember> {
  return requestWithSession<StudioMember>(
    `/api/v1/admin/studio/members/${id}`,
    json("PUT", input),
  );
}

/**
 * 归档成员（POST 而不是 DELETE：档案仍要在名录里可见）。
 *
 * 后端无 data 载荷时走 message 出口会丢掉成员对象，所以这里按成员返回——
 * 归档端点返回的是完整成员，直接复用 updateStudioMember 的同款解包。
 */
export function archiveStudioMember(id: number): Promise<StudioMember> {
  return requestWithSession<StudioMember>(
    `/api/v1/admin/studio/members/${id}/archive`,
    { method: "POST" },
  );
}
