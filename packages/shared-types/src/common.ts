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

// ISO-8601 时间戳的统一别名。
export type ISODateTime = string;

// 通用 ID（UUID v4）。
export type ID = string;
