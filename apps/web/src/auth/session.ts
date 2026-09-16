import type { AuthSession } from "@/api/auth";

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
