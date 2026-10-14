import type {
  ChangePasswordPayload,
  LoginPayload,
  RegisterPayload,
  ResetPayload,
} from "@/auth/form";

export const UNAVAILABLE_MESSAGE = "服务暂时不可用，请稍后重试";

export interface AuthSession {
  access_token: string;
  refresh_token: string;
  user_id: number;
}

export interface RefreshAuthSession {
  access_token: string;
  refresh_token: string;
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
 * 401 的细分业务码（issue #84 契约，apps/api/internal/dto/response）：
 *   4010 纯过期——签名有效只是 exp 到了，安静刷新即可；
 *   4011 会话失效——登出 / 被新登录顶替 / 改密后撤销，必须重新登录。
 * 网络异常（status 0）两者都不是，不得当成会话失效提示。
 */
export const AUTH_CODE_TOKEN_EXPIRED = 4010;
export const AUTH_CODE_SESSION_INVALIDATED = 4011;

/**
 * 发一次请求并解包统一响应体 { code, message, data }。
 *
 * 只负责「网络层是否成功 + 业务码是否为 0」，**不要求 data 存在**：
 * 登录/注册有 data，发送验证码没有，两种都由调用方决定怎么处理。
 * 网络断开、响应不是 JSON、code !== 0 都统一抛 AuthRequestError。
 */
export async function requestBackend<T>(
  path: string,
  init: RequestInit,
): Promise<{ body: Partial<BackendResponse<T>>; status: number }> {
  const controller = new AbortController();
  let timedOut = false;
  const abort = () => controller.abort();
  if (init.signal?.aborted) abort();
  else init.signal?.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, 15000);
  const networkMessage = () =>
    timedOut ? "请求超时，请检查网络后重试" : UNAVAILABLE_MESSAGE;
  try {
    let response: Response;
    try {
      response = await fetch(path, { ...init, signal: controller.signal });
    } catch {
      throw new AuthRequestError(networkMessage(), 0);
    }

    let body: Partial<BackendResponse<T>> | null;
    try {
      body = (await response.json()) as Partial<BackendResponse<T>>;
    } catch {
      throw new AuthRequestError(networkMessage(), response.status);
    }
    const businessCode = typeof body?.code === "number" ? body.code : undefined;
    const message = body?.message || UNAVAILABLE_MESSAGE;
    if (!response.ok || body?.code !== 0)
      throw new AuthRequestError(message, response.status, businessCode);
    return { body, status: response.status };
  } finally {
    clearTimeout(timer);
    init.signal?.removeEventListener("abort", abort);
  }
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
  const { body, status } = await requestBackend<T>(path, jsonPost(payload));

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
  headers?: HeadersInit,
): Promise<string> {
  const init = jsonPost(payload);
  init.headers = { "Content-Type": "application/json", ...headers };
  const { body } = await requestBackend<unknown>(path, init);
  return body.message ?? "success";
}

export function login(payload: LoginPayload): Promise<AuthSession> {
  return postAuth<AuthSession>("/api/v1/login", payload);
}

export function register(payload: RegisterPayload): Promise<AuthSession> {
  return postAuth<AuthSession>("/api/v1/register", payload);
}

/** 使用 refresh token 轮换出新的双令牌。 */
export function refreshAuthSession(
  refreshToken: string,
): Promise<RefreshAuthSession> {
  return postAuth<RefreshAuthSession>("/api/v1/refresh", {
    refresh_token: refreshToken,
  });
}

/** 注销当前会话；后端会让该用户的 access/refresh token 一并失效。 */
export function logout(
  accessToken: string,
  refreshToken: string,
): Promise<string> {
  return postAuthNoData(
    "/api/v1/logout",
    { refresh_token: refreshToken },
    { Authorization: `Bearer ${accessToken}` },
  );
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

// ── 密码找回 / 修改（issue #84 后端契约，FE-01 接入）──

/**
 * 发送找回密码验证码。与注册码互不覆盖（后端 purpose=reset），
 * 同一邮箱 60 秒冷却；未注册邮箱由后端返回明确错误。
 */
export function sendResetCode(email: string): Promise<string> {
  return postAuthNoData("/api/v1/send-reset-code", { email });
}

/** 用邮箱验证码重置密码。成功后该用户全部会话失效，所有端需重新登录。 */
export function resetPassword(payload: ResetPayload): Promise<string> {
  return postAuthNoData("/api/v1/reset-password", payload);
}

/**
 * 登录状态下修改密码。成功后后端撤销该用户全部会话（含当前请求的令牌），
 * 前端必须清掉本地会话并引导重新登录。
 */
export function changePassword(
  accessToken: string,
  payload: ChangePasswordPayload,
): Promise<string> {
  return postAuthNoData(
    "/api/v1/change-password",
    payload,
    { Authorization: `Bearer ${accessToken}` },
  );
}
