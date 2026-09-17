import { describe, expect, it, vi } from "vitest";

import { AuthRequestError, type AuthSession, type CurrentUser } from "@/api/auth";
import { fetchCurrentUserWithRefresh } from "./current-user";

const session: AuthSession = {
  access_token: "old-a",
  refresh_token: "old-r",
  user_id: 7,
};

const currentUser: CurrentUser = {
  user_id: 7,
  username: "ada",
  role: "admin",
};

describe("fetchCurrentUserWithRefresh", () => {
  it("refreshes once and retries /me when the access token expires", async () => {
    const fetchCurrentUser = vi
      .fn()
      .mockRejectedValueOnce(new AuthRequestError("token 已过期", 401, 4002))
      .mockResolvedValueOnce(currentUser);
    const refreshSession = vi.fn().mockResolvedValue({
      access_token: "new-a",
      refresh_token: "new-r",
      user_id: -1,
    });
    const saveSession = vi.fn();

    await expect(
      fetchCurrentUserWithRefresh("old-a", {
        readSession: () => session,
        fetchCurrentUser,
        refreshSession,
        saveSession,
      }),
    ).resolves.toEqual(currentUser);

    expect(refreshSession).toHaveBeenCalledWith("old-r");
    expect(saveSession).toHaveBeenCalledWith({
      access_token: "new-a",
      refresh_token: "new-r",
      user_id: 7,
    });
    expect(fetchCurrentUser).toHaveBeenNthCalledWith(2, "new-a");
  });

  it("does not refresh on a forbidden response", async () => {
    const error = new AuthRequestError("无权限访问该资源", 403, 4003);
    const refreshSession = vi.fn();

    await expect(
      fetchCurrentUserWithRefresh("old-a", {
        readSession: () => session,
        fetchCurrentUser: vi.fn().mockRejectedValue(error),
        refreshSession,
        saveSession: vi.fn(),
      }),
    ).rejects.toBe(error);

    expect(refreshSession).not.toHaveBeenCalled();
  });
});
