import type { LoginPayload, RegisterPayload } from "@/auth/form";

const UNAVAILABLE_MESSAGE = "服务暂时不可用，请稍后重试";

export interface AuthSession {
  access_token: string;
  refresh_token: string;
  user_id: number;
}

export interface BackendResponse<T> {
  code: number;
  message: string;
  data?: T;
}

export class AuthRequestError extends Error {
  readonly status: number;
  readonly businessCode?: number;

  constructor(message: string, status: number, businessCode?: number) {
    super(message);
    this.name = "AuthRequestError";
    this.status = status;
    this.businessCode = businessCode;
  }
}

/**
 * 发一次请求并解包统一响应体 { code, message, data }。
 *
 * 只负责「网络层是否成功 + 业务码是否为 0」，**不要求 data 存在**：
 * 登录/注册有 data，发送验证码没有，两种都由调用方决定怎么处理。
 * 网络断开、响应不是 JSON、code !== 0 都统一抛 AuthRequestError。
 */
async function requestAuth<T>(
  path: string,
  init: RequestInit,
): Promise<{ body: Partial<BackendResponse<T>>; status: number }> {
  let response: Response;
  try {
    response = await fetch(path, init);
  } catch {
    throw new AuthRequestError(UNAVAILABLE_MESSAGE, 0);
  }

  let body: Partial<BackendResponse<T>> | null;
  try {
    body = (await response.json()) as Partial<BackendResponse<T>>;
  } catch {
    throw new AuthRequestError(UNAVAILABLE_MESSAGE, response.status);
  }

  const businessCode = typeof body?.code === "number" ? body.code : undefined;
  const message = body?.message || UNAVAILABLE_MESSAGE;

  if (!response.ok || body?.code !== 0) {
    throw new AuthRequestError(message, response.status, businessCode);
  }

  return { body, status: response.status };
}

function jsonPost(payload: unknown): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  };
}

/** 需要 data 的接口：data 缺失按「服务不可用」处理，避免把 undefined 当业务对象用 */
async function postAuth<T>(path: string, payload: unknown): Promise<T> {
  const { body, status } = await requestAuth<T>(path, jsonPost(payload));

  if (body.data === undefined) {
    throw new AuthRequestError(UNAVAILABLE_MESSAGE, status, body.code);
  }

  return body.data;
}

/** 需要 data 的 GET（携带 Bearer 令牌）*/
async function getAuth<T>(path: string, accessToken: string): Promise<T> {
  const { body, status } = await requestAuth<T>(path, {
    method: "GET",
    headers: { Authorization: `Bearer ${accessToken}` },
  });

  if (body.data === undefined) {
    throw new AuthRequestError(UNAVAILABLE_MESSAGE, status, body.code);
  }

  return body.data;
}

/**
 * 不返回业务数据的接口（例如发送验证码）返回后端 message。
 * 不能复用 postAuth：它要求 data 必须存在，而后端对这类接口返回
 * `OKWithMsg(..., nil)`，data 会被 omitempty 丢掉。
 */
async function postAuthNoData(
  path: string,
  payload: unknown,
): Promise<string> {
  const { body } = await requestAuth<unknown>(path, jsonPost(payload));
  return body.message ?? "success";
}

export function login(payload: LoginPayload): Promise<AuthSession> {
  return postAuth<AuthSession>("/api/v1/login", payload);
}

export function register(payload: RegisterPayload): Promise<AuthSession> {
  return postAuth<AuthSession>("/api/v1/register", payload);
}

/**
 * 发送邮箱验证码。注册（RegisterReq.code）必须先拿到它才能提交。
 * 后端只回 message，不回 data。
 */
export function sendVerifyCode(
  verifier: string,
  verifierType: "email" | "phone" = "email",
): Promise<string> {
  return postAuthNoData("/api/v1/send-verify-code", {
    verifier,
    verifier_type: verifierType,
  });
}

/** 后端角色取值。前端只据此控制可见性，真正的强制在服务端的 RequireRole 中间件 */
export type UserRole = "admin" | "member";

export interface CurrentUser {
  user_id: number;
  username: string;
  role: UserRole | string;
}

/**
 * 拉取当前登录用户的身份与角色（需要 access token）。
 *
 * 角色写在令牌里、来源是服务端的部署配置白名单，所以这里必须**问服务端**，
 * 不能用本地缓存猜：改了白名单之后本地旧值会过期。取不到（401/网络异常）
 * 一律抛 AuthRequestError，调用方按「不是管理员」处理。
 */
export function fetchCurrentUser(accessToken: string): Promise<CurrentUser> {
  return getAuth<CurrentUser>("/api/v1/me", accessToken);
}
