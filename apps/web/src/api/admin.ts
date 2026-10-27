import { requestWithSession } from "./session-request";

export type UserRole = "admin" | "member";

export interface CurrentUser {
  user_id: number;
  username: string;
  role: UserRole | string;
  /**
   * 创始管理员标记（issue #120）。ADMIN_USERS 种入账号为 true；
   * 后端字段缺省时按 false 处理（后端合并前守卫拦截静默不生效）。
   */
  is_founding?: boolean;
}

/**
 * 详情字段的对外最小子集。列表里塞不下完整字段集（list 接口只透 name / role），
 * 但 issue #58 要求「详情」弹窗，所以后端 #57 把 AdminUser 扩成可带 detail。
 * detail 本身是 optional —— 字段还没填时是 undefined，弹窗里走「暂无完整资料」分支。
 */
export interface AdminUserDetail {
  class?: string | null;
  student_id?: number | null;
  qq?: string | null;
  direction?: import("@iot-pillot/shared-types").Direction | null;
  email?: string | null;
}

export interface AdminUser {
  user_id: number;
  name: string;
  created_at: string;
  is_admin: boolean;
  /**
   * 账号是否已注销（issue #142 Q3 软删标注）。后端字段缺省按 false；
   * true 时列表显示「账号已注销」，该行不再提供注销按钮。
   */
  is_deleted?: boolean;
  /**
   * 账号是否因资料不合规被冻结（0012）。列表默认不返回冻结行，
   * 打开「显示冻结账号」后才可能出现 true；此时该行提供「解冻」按钮。
   */
  is_frozen?: boolean;
  /** 冻结时刻（RFC3339），未冻结为空串。 */
  frozen_at?: string;
  /** 冻结原因快照，例如「班级须为「汉字 + 6 位数字」」。 */
  freeze_reason?: string;
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

/**
 * 分页读取管理员用户列表，自动处理会话刷新。
 *
 * includeFrozen 默认 false：冻结是系统判定「资料不合规」的处置状态，
 * 混在名单里会淹没真正在推进招新的人；需要处置时显式打开。
 */
export function listAdminUsers(
  page = 1,
  pageSize = 20,
  includeFrozen = false,
): Promise<AdminUserList> {
  const query = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
    include_frozen: String(includeFrozen),
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

/**
 * 注销账号（issue #142，DELETE /admin/users/:id，创始管理员专属）。
 * 二次确认（输入目标用户名）由调用方负责；后端另有三重守卫：
 * 非创始 403、不能注销自己、创始互不注销。
 */
export function deleteAdminUser(userId: number): Promise<void> {
  return requestWithSession<void>(`/api/v1/admin/users/${userId}`, {
    method: "DELETE",
  });
}

/**
 * 解冻账号（0012，POST /admin/users/:id/unfreeze）。
 *
 * 只清冻结标记，不校验资料是否已补齐：解冻的语义是「允许这个人重新登录去改资料」，
 * 资料仍不合规时下一轮定时扫描会再次冻结。
 */
export function unfreezeAdminUser(userId: number): Promise<{
  user_id: number;
  is_frozen: boolean;
}> {
  return requestWithSession<{ user_id: number; is_frozen: boolean }>(
    `/api/v1/admin/users/${userId}/unfreeze`,
    { method: "POST" },
  );
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

/** 发信记录查询条件：keyword 模糊匹配主题/收件人，from/to 为 YYYY-MM-DD 日期（含端点）。 */
export interface MailRecordFilter {
  keyword?: string;
  from?: string;
  to?: string;
}

/**
 * 分页读取发信记录，自动处理会话刷新。
 * 过滤在服务端完成（keyword/from/to），total 为过滤后的总数。
 */
export function listMails(
  page = 1,
  pageSize = 20,
  filter?: MailRecordFilter,
): Promise<MailRecordList> {
  const query = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (filter?.keyword?.trim()) query.set("keyword", filter.keyword.trim());
  if (filter?.from) query.set("from", filter.from);
  if (filter?.to) query.set("to", filter.to);
  return requestWithSession<MailRecordList>(
    `/api/v1/admin/mails?${query.toString()}`,
  );
}
