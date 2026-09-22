import { describe, expect, it, vi } from "vitest";

import {
  AuthRequestError,
  type AuthSession,
  type RefreshAuthSession,
} from "./auth";
import {
  createSessionRequester,
  type SessionRequestDeps,
} from "./session-request";

const originalSession: AuthSession = {
  access_token: "old-access",
  refresh_token: "old-refresh",
  user_id: 7,
};

function ok<T>(data: T) {
  return {
    body: { code: 0, message: "success", data },
    status: 200,
  };
}

function makeDeps(
  overrides: Partial<SessionRequestDeps> = {},
): SessionRequestDeps {
  return {
    readSession: () => originalSession,
    refreshSession: vi.fn().mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    }),
    saveSession: vi.fn(),
    clearSession: vi.fn(),
    onSessionExpired: vi.fn(),
    request: vi.fn().mockResolvedValue(ok({ value: 1 })),
    ...overrides,
  };
}

describe("session requester", () => {
  it("adds the current bearer token and returns response data", async () => {
    let receivedPath = "";
    let receivedInit: RequestInit | undefined;
    const deps = makeDeps({
      request: async (path, init) => {
        receivedPath = path;
        receivedInit = init;
        return ok({ value: 42 });
      },
    });
    const request = createSessionRequester(deps);

    await expect(request<{ value: number }>("/api/v1/admin/users")).resolves.toEqual({
      value: 42,
    });

    expect(receivedPath).toBe("/api/v1/admin/users");
    expect(new Headers(receivedInit?.headers).get("Authorization")).toBe(
      "Bearer old-access",
    );
  });

  it("refreshes once, saves the rotated session, and retries the original request", async () => {
    const usedTokens: string[] = [];
    const saveSession = vi.fn();
    const deps = makeDeps({
      saveSession,
      request: async (_path, init) => {
        const token = new Headers(init?.headers).get("Authorization") ?? "";
        usedTokens.push(token);
        if (token === "Bearer old-access") {
          throw new AuthRequestError("token 已过期", 401, 4002);
        }
        return ok({ value: 42 });
      },
    });
    const request = createSessionRequester(deps);

    await expect(request<{ value: number }>("/api/v1/admin/users")).resolves.toEqual({
      value: 42,
    });

    expect(usedTokens).toEqual(["Bearer old-access", "Bearer new-access"]);
    expect(deps.refreshSession).toHaveBeenCalledWith("old-refresh");
    expect(saveSession).toHaveBeenCalledWith({
      access_token: "new-access",
      refresh_token: "new-refresh",
      user_id: 7,
    });
  });

  it("does not refresh or clear the session for a forbidden response", async () => {
    const forbidden = new AuthRequestError("无权限访问该资源", 403, 4003);
    const deps = makeDeps({
      request: vi.fn().mockRejectedValue(forbidden),
    });
    const request = createSessionRequester(deps);

    await expect(request("/api/v1/admin/users")).rejects.toBe(forbidden);

    expect(deps.refreshSession).not.toHaveBeenCalled();
    expect(deps.clearSession).not.toHaveBeenCalled();
    expect(deps.onSessionExpired).not.toHaveBeenCalled();
  });

  it("clears the session and expires once when refresh fails", async () => {
    const refreshError = new AuthRequestError("refresh token 已失效", 401, 4002);
    const deps = makeDeps({
      request: vi
        .fn()
        .mockRejectedValue(new AuthRequestError("token 已过期", 401, 4002)),
      refreshSession: vi.fn().mockRejectedValue(refreshError),
    });
    const request = createSessionRequester(deps);

    await expect(request("/api/v1/admin/users")).rejects.toBe(refreshError);

    expect(deps.clearSession).toHaveBeenCalledTimes(1);
    expect(deps.onSessionExpired).toHaveBeenCalledTimes(1);
  });

  it("clears the session when refresh throws synchronously", async () => {
    const refreshError = new Error("refresh unavailable");
    const deps = makeDeps({
      request: vi
        .fn()
        .mockRejectedValue(new AuthRequestError("token 已过期", 401, 4002)),
      refreshSession: vi.fn(() => {
        throw refreshError;
      }),
    });
    const request = createSessionRequester(deps);

    await expect(request("/api/v1/admin/users")).rejects.toBe(refreshError);

    expect(deps.clearSession).toHaveBeenCalledTimes(1);
    expect(deps.onSessionExpired).toHaveBeenCalledTimes(1);
  });

  it("shares one refresh across concurrent expired requests", async () => {
    let resolveRefresh: ((session: RefreshAuthSession) => void) | undefined;
    const refreshResult = new Promise<RefreshAuthSession>((resolve) => {
      resolveRefresh = resolve;
    });
    const refreshSession = vi.fn(() => refreshResult);
    const deps = makeDeps({
      refreshSession,
      request: async (_path, init) => {
        const token = new Headers(init?.headers).get("Authorization");
        if (token === "Bearer old-access") {
          throw new AuthRequestError("token 已过期", 401, 4002);
        }
        return ok({ token });
      },
    });
    const request = createSessionRequester(deps);

    const first = request("/api/v1/admin/users");
    const second = request("/api/v1/admin/mails");
    await vi.waitFor(() => expect(refreshSession).toHaveBeenCalledTimes(1));
    resolveRefresh?.({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });

    await expect(Promise.all([first, second])).resolves.toHaveLength(2);
    expect(refreshSession).toHaveBeenCalledTimes(1);
  });

  it("never refreshes more than once when the retried request is also unauthorized", async () => {
    const deps = makeDeps({
      request: vi
        .fn()
        .mockRejectedValue(new AuthRequestError("token 已失效", 401, 4002)),
    });
    const request = createSessionRequester(deps);

    await expect(request("/api/v1/admin/users")).rejects.toMatchObject({
      status: 401,
    });

    expect(deps.request).toHaveBeenCalledTimes(2);
    expect(deps.refreshSession).toHaveBeenCalledTimes(1);
    expect(deps.clearSession).toHaveBeenCalledTimes(1);
    expect(deps.onSessionExpired).toHaveBeenCalledTimes(1);
  });

  it("expires only once when concurrent retries are both unauthorized", async () => {
    const deps = makeDeps({
      request: vi
        .fn()
        .mockRejectedValue(new AuthRequestError("token 已失效", 401, 4002)),
    });
    const request = createSessionRequester(deps);

    await expect(
      Promise.allSettled([
        request("/api/v1/admin/users"),
        request("/api/v1/admin/mails"),
      ]),
    ).resolves.toHaveLength(2);

    expect(deps.refreshSession).toHaveBeenCalledTimes(1);
    expect(deps.clearSession).toHaveBeenCalledTimes(1);
    expect(deps.onSessionExpired).toHaveBeenCalledTimes(1);
  });
});
