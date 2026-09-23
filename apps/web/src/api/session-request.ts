import {
  AuthRequestError,
  requestBackend,
  refreshAuthSession,
  UNAVAILABLE_MESSAGE,
  type AuthSession,
  type BackendResponse,
  type RefreshAuthSession,
} from "./auth";
import {
  clearAuthSession,
  mergeAuthSession,
  readAuthSession,
  saveAuthSession,
} from "@/auth/session";

export interface SessionRequestDeps {
  readSession: () => AuthSession | null;
  refreshSession: (refreshToken: string) => Promise<RefreshAuthSession>;
  saveSession: (session: AuthSession) => void;
  clearSession: () => void;
  onSessionExpired: () => void;
  request: (
    path: string,
    init: RequestInit,
  ) => Promise<{
    body: Partial<BackendResponse<unknown>>;
    status: number;
  }>;
}

export type SessionRequester = <T>(
  path: string,
  init?: RequestInit,
) => Promise<T>;

const SESSION_EXPIRED_MESSAGE = "登录状态已失效，请重新登录";

function defaultSessionExpired(): void {
  if (typeof window !== "undefined") {
    window.location.assign("/");
  }
}

function defaultRequest(
  path: string,
  init: RequestInit,
): Promise<{
  body: Partial<BackendResponse<unknown>>;
  status: number;
}> {
  return requestBackend<unknown>(path, init);
}

const defaultDeps: SessionRequestDeps = {
  readSession: () => readAuthSession(),
  refreshSession: (refreshToken) => refreshAuthSession(refreshToken),
  saveSession: (session) => saveAuthSession(session),
  clearSession: () => clearAuthSession(),
  onSessionExpired: defaultSessionExpired,
  request: defaultRequest,
};

function withBearer(init: RequestInit | undefined, accessToken: string): RequestInit {
  const headers = new Headers(init?.headers);
  headers.set("Authorization", `Bearer ${accessToken}`);
  return { ...init, headers };
}

function dataOf<T>(
  result: Awaited<ReturnType<SessionRequestDeps["request"]>>,
): T {
  if (result.body.data === undefined) {
    throw new AuthRequestError(
      UNAVAILABLE_MESSAGE,
      result.status,
      result.body.code,
    );
  }

  return result.body.data as T;
}

/** 解包策略：把一次成功的响应（code===0）变成调用方要的返回值。 */
type Unwrap = <T>(
  result: Awaited<ReturnType<SessionRequestDeps["request"]>>,
) => T;

function messageOf(
  result: Awaited<ReturnType<SessionRequestDeps["request"]>>,
): string {
  return result.body.message ?? "";
}

export type SessionMessageRequester = (
  path: string,
  init?: RequestInit,
) => Promise<string>;

/**
 * 不返回业务数据的受保护接口（如删除模板：后端 OKWithMsg(..., nil)，
 * data 被 Result 的 json omitempty 丢掉）走这个出口——返回后端 message。
 * 与 auth.ts 的 postAuthNoData 同因：按 dataOf 把「data 缺失」当服务不可用，
 * 会让成功的删除弹出「服务暂时不可用」的假失败。
 */
export function createSessionMessageRequester(
  deps: SessionRequestDeps = defaultDeps,
): SessionMessageRequester {
  const run = createSessionRunner(deps, messageOf as Unwrap);
  return (path, init) => run(path, init);
}

export function createSessionRequester(
  deps: SessionRequestDeps = defaultDeps,
): SessionRequester {
  return createSessionRunner(deps, dataOf as Unwrap);
}

/** 会话请求核心：加 Bearer、401 时刷新重试、按策略解包。两个请求器共用。 */
function createSessionRunner(
  deps: SessionRequestDeps,
  unwrap: Unwrap,
): SessionRequester {
  let refreshInFlight: Promise<AuthSession> | null = null;
  let expirationKey: string | null = null;

  function expireSession(session: AuthSession | null = null): void {
    const nextExpirationKey = session?.access_token ?? "<none>";
    if (expirationKey === nextExpirationKey) {
      return;
    }

    expirationKey = nextExpirationKey;
    deps.clearSession();
    deps.onSessionExpired();
  }

  function refreshFor(attemptedAccessToken: string): Promise<AuthSession> {
    const latestSession = deps.readSession();
    if (latestSession === null) {
      const error = new AuthRequestError(SESSION_EXPIRED_MESSAGE, 401);
      expireSession();
      return Promise.reject(error);
    }

    // Another request may have completed the rotation while this request was
    // waiting for its 401 to be handled. Reuse that session instead of rotating
    // the refresh token a second time.
    if (latestSession.access_token !== attemptedAccessToken) {
      return Promise.resolve(latestSession);
    }

    if (refreshInFlight === null) {
      refreshInFlight = Promise.resolve()
        .then(() => deps.refreshSession(latestSession.refresh_token))
        .then((refreshed) => {
          const nextSession = mergeAuthSession(latestSession, refreshed);
          deps.saveSession(nextSession);
          return nextSession;
        })
        .catch((error: unknown) => {
          expireSession();
          throw error;
        })
        .finally(() => {
          refreshInFlight = null;
        });
    }

    return refreshInFlight;
  }

  return async <T>(path: string, init?: RequestInit): Promise<T> => {
    const session = deps.readSession();
    if (session === null) {
      const error = new AuthRequestError(SESSION_EXPIRED_MESSAGE, 401);
      expireSession();
      throw error;
    }

    if (expirationKey !== null && expirationKey !== session.access_token) {
      expirationKey = null;
    }

    try {
      const result = await deps.request(path, withBearer(init, session.access_token));
      return unwrap<T>(result);
    } catch (error: unknown) {
      if (!(error instanceof AuthRequestError) || error.status !== 401) {
        throw error;
      }

      const nextSession = await refreshFor(session.access_token);

      try {
        const retry = await deps.request(
          path,
          withBearer(init, nextSession.access_token),
        );
        return unwrap<T>(retry);
      } catch (retryError: unknown) {
        if (retryError instanceof AuthRequestError && retryError.status === 401) {
          expireSession(nextSession);
        }
        throw retryError;
      }
    }
  };
}

export const requestWithSession = createSessionRequester();
export const requestMessageWithSession = createSessionMessageRequester();
