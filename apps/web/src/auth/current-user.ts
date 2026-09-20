import {
  AuthRequestError,
  fetchCurrentUser,
  refreshAuthSession,
  type AuthSession,
  type CurrentUser,
} from "@/api/auth";

import {
  mergeAuthSession,
  readAuthSession,
  saveAuthSession,
} from "./session";

export interface CurrentUserDeps {
  readSession: () => AuthSession | null;
  fetchCurrentUser: (accessToken: string) => Promise<CurrentUser>;
  refreshSession: (refreshToken: string) => Promise<AuthSession>;
  saveSession: (session: AuthSession) => void;
}

const defaultDeps: CurrentUserDeps = {
  readSession: () => readAuthSession(),
  fetchCurrentUser,
  refreshSession: refreshAuthSession,
  saveSession: saveAuthSession,
};

/**
 * 查询当前用户；access token 过期时用 refresh token 轮换一次并重试。
 * 只有 401 会触发刷新，403 仍然交给路由守卫作为拒绝处理。
 */
export async function fetchCurrentUserWithRefresh(
  accessToken: string,
  deps: CurrentUserDeps = defaultDeps,
): Promise<CurrentUser> {
  try {
    return await deps.fetchCurrentUser(accessToken);
  } catch (error: unknown) {
    if (!(error instanceof AuthRequestError) || error.status !== 401) {
      throw error;
    }

    const session = deps.readSession();
    if (session === null || session.access_token !== accessToken) {
      throw error;
    }

    const refreshed = await deps.refreshSession(session.refresh_token);
    const nextSession = mergeAuthSession(session, refreshed);
    deps.saveSession(nextSession);
    return deps.fetchCurrentUser(nextSession.access_token);
  }
}
