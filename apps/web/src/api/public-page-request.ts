import { AuthRequestError, requestBackend, UNAVAILABLE_MESSAGE } from "./auth";
import { clearAuthSession, readAuthSession, syncAuthSession } from "@/auth/session";
import type { SessionRequester } from "./session-request";

/** 公开页面读取个人数据：失效时留在当前页降为游客，不触发全页登录跳转。 */
export const requestForPublicPage: SessionRequester = async <T>(
  path: string,
  init?: RequestInit,
  expectedUserId?: number,
): Promise<T> => {
  syncAuthSession();
  const session = readAuthSession();
  if (!session) throw new AuthRequestError("请先登录", 401);
  if (expectedUserId !== undefined && session.user_id !== expectedUserId) {
    throw new AuthRequestError("登录账号已变更，请重新操作。", 409);
  }
  const headers = new Headers(init?.headers);
  headers.set("Authorization", `Bearer ${session.access_token}`);
  try {
    const { body, status } = await requestBackend<T>(path, { ...init, headers });
    syncAuthSession();
    if (readAuthSession()?.access_token !== session.access_token) {
      throw new AuthRequestError("登录账号已变更，请重新操作。", 409);
    }
    if (body.data === undefined) {
      throw new AuthRequestError(UNAVAILABLE_MESSAGE, status);
    }
    return body.data;
  } catch (error) {
    syncAuthSession();
    if (
      error instanceof AuthRequestError &&
      (error.status === 401 || error.status === 403) &&
      readAuthSession()?.access_token === session.access_token
    ) {
      clearAuthSession();
    }
    throw error;
  }
};
