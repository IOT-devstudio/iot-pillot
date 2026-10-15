import {
  AUTH_CODE_SESSION_INVALIDATED,
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

/**
 * 会话被服务端判定失效的原因（issue #84/#85）：
 *   expired     —— 4010 纯过期且刷新失败，或刷新令牌缺失；
 *   invalidated —— 4011 会话被撤销（登出 / 被新登录顶替 / 改密）。
 * 网络异常两者都不是，不会触发 onSessionExpired，更不会误报「其他位置登录」。
 */
export type SessionEndReason = "expired" | "invalidated";

export interface SessionRequestDeps {
  readSession: () => AuthSession | null;
  refreshSession: (refreshToken: string) => Promise<RefreshAuthSession>;
  saveSession: (session: AuthSession) => void;
  clearSession: () => void;
  onSessionExpired: (reason: SessionEndReason) => void;
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
  expectedUserId?: number,
) => Promise<T>;

const SESSION_EXPIRED_MESSAGE = "登录状态已失效，请重新登录";

function defaultSessionExpired(reason: SessionEndReason): void {
  if (typeof window !== "undefined") {
    // 带 reason 回登录页：登录面板按 ?reason= 出对应提示条（AuthView）
    window.location.assign(`/?reason=${reason}`);
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

  function assertExpectedAccount(expectedUserId?: number, session?: AuthSession | null): void {
    if (expectedUserId === undefined) return;
    const current = session === undefined ? deps.readSession() : session;
    if (current?.user_id !== expectedUserId) {
      // localStorage 在标签页间共享；旧表单不能使用另一账号的新令牌写入。
      throw new AuthRequestError("登录账号已变更，请刷新页面后重新编辑资料。", 409);
    }
  }

  function expireSession(
    session: AuthSession | null = null,
    reason: SessionEndReason = "expired",
  ): void {
    const nextExpirationKey = session?.access_token ?? "<none>";
    if (expirationKey === nextExpirationKey) {
      return;
    }

    expirationKey = nextExpirationKey;
    deps.clearSession();
    deps.onSessionExpired(reason);
  }

  /**
   * 4011 会话已被服务端撤销（顶替/改密/登出）：刷新没有意义——refresh
   * token 和 access token 属于同一个已销毁的会话，必定同样 401。直接
   * 收尾并把 invalidated 原因带给登录页，别再走一轮注定失败的刷新。
   */
  function isSessionInvalidated(error: AuthRequestError): boolean {
    return (
      error.status === 401 &&
      error.businessCode === AUTH_CODE_SESSION_INVALIDATED
    );
  }

  function refreshFor(attemptedAccessToken: string, expectedUserId?: number): Promise<AuthSession> {
    const latestSession = deps.readSession();
    assertExpectedAccount(expectedUserId, latestSession);
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
          const current = deps.readSession();
          assertExpectedAccount(latestSession.user_id, current);
          // 刷新期间同账号也可能重新登录，不覆盖其较新的会话。
          if (current && current.access_token !== latestSession.access_token) return current;
          const nextSession = mergeAuthSession(latestSession, refreshed);
          deps.saveSession(nextSession);
          return nextSession;
        })
        .catch((error: unknown) => {
          assertExpectedAccount(latestSession.user_id);
          // 只有后端明确说令牌无效（401/4011）才认定会话结束；
          // 网络异常等其余失败保留本地会话——断网不该把人登出
          if (
            error instanceof AuthRequestError &&
            (error.status === 401 || isSessionInvalidated(error))
          ) {
            expireSession(
              null,
              isSessionInvalidated(error) ? "invalidated" : "expired",
            );
          }
          throw error;
        })
        .finally(() => {
          refreshInFlight = null;
        });
    }

    return refreshInFlight;
  }

  return async <T>(path: string, init?: RequestInit, expectedUserId?: number): Promise<T> => {
    const session = deps.readSession();
    assertExpectedAccount(expectedUserId, session);
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
      assertExpectedAccount(expectedUserId);
      return unwrap<T>(result);
    } catch (error: unknown) {
      if (!(error instanceof AuthRequestError) || error.status !== 401) {
        throw error;
      }

      assertExpectedAccount(expectedUserId);

      if (isSessionInvalidated(error)) {
        expireSession(session, "invalidated");
        throw error;
      }

      const nextSession = await refreshFor(session.access_token, expectedUserId);
      assertExpectedAccount(expectedUserId, nextSession);
      assertExpectedAccount(expectedUserId);

      try {
        const retry = await deps.request(
          path,
          withBearer(init, nextSession.access_token),
        );
        assertExpectedAccount(expectedUserId);
        return unwrap<T>(retry);
      } catch (retryError: unknown) {
        assertExpectedAccount(expectedUserId);
        if (retryError instanceof AuthRequestError && retryError.status === 401) {
          expireSession(
            nextSession,
            isSessionInvalidated(retryError) ? "invalidated" : "expired",
          );
        }
        throw retryError;
      }
    }
  };
}

export const requestWithSession = createSessionRequester();
export const requestMessageWithSession = createSessionMessageRequester();
