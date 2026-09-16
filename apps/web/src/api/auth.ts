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

async function postAuth<T>(path: string, payload: unknown): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  } catch {
    throw new AuthRequestError(UNAVAILABLE_MESSAGE, 0);
  }

  let body: Partial<BackendResponse<T>> | null;
  try {
    body = (await response.json()) as Partial<BackendResponse<T>>;
  } catch {
    throw new AuthRequestError(UNAVAILABLE_MESSAGE, response.status);
  }

  const businessCode =
    typeof body?.code === "number" ? body.code : undefined;
  const message = body?.message || UNAVAILABLE_MESSAGE;

  if (!response.ok || body?.code !== 0 || body.data === undefined) {
    throw new AuthRequestError(message, response.status, businessCode);
  }

  return body.data;
}

export function login(payload: LoginPayload): Promise<AuthSession> {
  return postAuth<AuthSession>("/api/v1/login", payload);
}

export function register(payload: RegisterPayload): Promise<AuthSession> {
  return postAuth<AuthSession>("/api/v1/register", payload);
}
