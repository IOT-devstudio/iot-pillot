import { requestWithSession } from "./session-request";

export type UserRole = "admin" | "member";

export interface CurrentUser {
  user_id: number;
  username: string;
  role: UserRole | string;
}

export interface AdminUser {
  user_id: number;
  name: string;
  created_at: string;
  is_admin: boolean;
}

export interface AdminUserList {
  items: AdminUser[];
  total: number;
  page: number;
  page_size: number;
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
