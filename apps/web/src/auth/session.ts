import {
  logout as requestLogout,
  type AuthSession,
  type RefreshAuthSession,
} from "@/api/auth";

export const AUTH_STORAGE_KEY = "iot-pillot.auth";

function getStorage(storage?: Storage): Storage {
  return storage ?? window.localStorage;
}

function isAuthSession(value: unknown): value is AuthSession {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const session = value as Partial<AuthSession>;
  return (
    typeof session.access_token === "string" &&
    typeof session.refresh_token === "string" &&
    typeof session.user_id === "number"
  );
}

export function saveAuthSession(
  session: AuthSession,
  storage?: Storage,
): void {
  getStorage(storage).setItem(AUTH_STORAGE_KEY, JSON.stringify(session));
}

export function readAuthSession(storage?: Storage): AuthSession | null {
  const rawSession = getStorage(storage).getItem(AUTH_STORAGE_KEY);
  if (rawSession === null) {
    return null;
  }

  try {
    const session: unknown = JSON.parse(rawSession);
    return isAuthSession(session) ? session : null;
  } catch {
    return null;
  }
}

export function clearAuthSession(storage?: Storage): void {
  getStorage(storage).removeItem(AUTH_STORAGE_KEY);
}

/**
 * 合并 refresh 接口返回的令牌。
 *
 * 后端 refresh 只返回新令牌，用户 ID 需要沿用本地会话中的值。
 */
export function mergeAuthSession(
  current: AuthSession,
  refreshed: RefreshAuthSession,
): AuthSession {
  return {
    ...current,
    ...refreshed,
    user_id: current.user_id,
  };
}

export interface SignOutDeps {
  readSession: () => AuthSession | null;
  requestLogout: (accessToken: string, refreshToken: string) => Promise<unknown>;
  clearSession: () => void;
}

const defaultSignOutDeps: SignOutDeps = {
  readSession: () => readAuthSession(),
  requestLogout,
  clearSession: () => clearAuthSession(),
};

/** 请求后端注销，并且无论网络结果如何都清理本地会话。 */
export async function signOut(
  deps: SignOutDeps = defaultSignOutDeps,
): Promise<void> {
  const session = deps.readSession();

  try {
    if (session !== null) {
      await deps.requestLogout(session.access_token, session.refresh_token);
    }
  } finally {
    deps.clearSession();
  }
}
