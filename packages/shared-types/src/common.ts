// 通用 API 协议：响应、分页、错误码、用户角色。

export interface ApiResponse<T> {
  data: T;
  meta?: PaginationMeta;
}

export interface PaginationMeta {
  page: number;
  pageSize: number;
  total: number;
}

export interface ApiErrorBody {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

export type Role = "admin" | "member";

/**
 * 个人方向（招新「意向」）。与 Prospect 的 `intention` 不是同一个东西：
 * Prospect 描述「报名者走到哪一步」，Direction 描述「这个人现在选哪个方向」。
 *
 * 后端 `domain.UserDetail.direction` 的取值。前端下拉的中文标签与之一一对应，
 * 加新方向必须前后端一起改，否则前端会让用户提交非法值。
 */
export type Direction =
  | "front-end"
  | "back-end"
  | "agent"
  | "all"
  | "game"
  | "other";

// ISO-8601 时间戳的统一别名。
export type ISODateTime = string;

// 通用 ID（UUID v4）。
export type ID = string;
