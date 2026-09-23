import { requestWithSession } from "./session-request";

export type UserRole = "admin" | "member";

export interface CurrentUser {
  user_id: number;
  username: string;
  role: UserRole | string;
}

/**
 * 管理员列表里的 detail 字段（#57 后端契约）。
 *
 * 在 #57 合并之前它是 optional：当前后端还没把 detail 写到 items 上，
 * 前端用本地 fixture 给 demo 数据展示，#57 落地后由后端真实填充，
 * 字段缺失时 UI 退化为「暂无」。
 *
 * 形状以 #57 PR 合并时的版本为准；此处是契约草案。
 */
export interface AdminUserDetail {
  class?: string;
  student_id?: number;
  qq?: string;
  direction?: import("@iot-pillot/shared-types").Direction | null;
  email?: string;
}

export interface AdminUser {
  user_id: number;
  name: string;
  created_at: string;
  is_admin: boolean;
  /** #57 落地后由后端填充；当前前端用 fixture 模拟。 */
  detail?: AdminUserDetail;
}

export interface AdminUserList {
  items: AdminUser[];
  total: number;
  page: number;
  page_size: number;
}

/** 管理员名单（不分页）。 */
export interface AdminList {
  items: AdminUser[];
  total: number;
}

/** 提升/撤销管理员的结果。session_revoked=false 表示对方的旧令牌没能即时失效。 */
export interface AdminMutation {
  user_id: number;
  is_admin: boolean;
  session_revoked: boolean;
}

/** 拉取当前登录用户的身份与角色，自动处理会话刷新。 */
export function fetchCurrentUser(): Promise<CurrentUser> {
  return requestWithSession<CurrentUser>("/api/v1/me");
}

/** 分页读取管理员用户列表，自动处理会话刷新。 */
export function listAdminUsers(
  page = 1,
  pageSize = 20,
): Promise<AdminUserList> {
  const query = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  return requestWithSession<AdminUserList>(
    `/api/v1/admin/users?${query.toString()}`,
  );
}

/** 读取当前管理员名单，自动处理会话刷新。 */
export function listAdmins(): Promise<AdminList> {
  return requestWithSession<AdminList>("/api/v1/admin/admins");
}

/** 把已注册用户提升为管理员；后端会顺带撤销其会话，需重新登录。 */
export function grantAdmin(userId: number): Promise<AdminMutation> {
  return requestWithSession<AdminMutation>("/api/v1/admin/admins", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ user_id: userId }),
  });
}

/** 撤销管理员；后端同样会撤销其会话。撤销自己会被服务端拒绝。 */
export function revokeAdmin(userId: number): Promise<AdminMutation> {
  return requestWithSession<AdminMutation>(`/api/v1/admin/admins/${userId}`, {
    method: "DELETE",
  });
}

/** 发信记录（后端 MailRecordResp）。完整管理在 #52，这里只供概览展示。 */
export interface MailRecord {
  id: number;
  title: string;
  from_user_id: number;
  to_user_id: number;
  to_email: string;
  created_at: string;
}

export interface MailRecordList {
  items: MailRecord[];
  total: number;
  page: number;
  page_size: number;
}

/** 分页读取发信记录，自动处理会话刷新。 */
export function listMails(page = 1, pageSize = 20): Promise<MailRecordList> {
  const query = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  return requestWithSession<MailRecordList>(
    `/api/v1/admin/mails?${query.toString()}`,
  );
}
