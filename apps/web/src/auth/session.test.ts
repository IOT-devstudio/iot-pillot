import type { AuthSession } from "@/api/auth";
import { describe, expect, it, vi } from "vitest";
import {
  clearAuthSession,
  mergeAuthSession,
  readAuthSession,
  saveAuthSession,
  signOut,
} from "./session";

function memoryStorage(): Storage {
  const values = new Map<string, string>();

  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => {
      values.set(key, value);
    },
    removeItem: (key) => {
      values.delete(key);
    },
    clear: () => {
      values.clear();
    },
    key: (index) => [...values.keys()][index] ?? null,
    get length() {
      return values.size;
    },
  };
}

describe("auth session storage", () => {
  it("round-trips the auth session under the fixed storage key", () => {
    const storage = memoryStorage();
    const session: AuthSession = {
      access_token: "a",
      refresh_token: "r",
      user_id: 3,
    };

    saveAuthSession(session, storage);

    expect(storage.getItem("iot-pillot.auth")).toBe(JSON.stringify(session));
    expect(readAuthSession(storage)).toEqual(session);
  });

  it("returns null for malformed or cleared session data", () => {
    const storage = memoryStorage();

    storage.setItem("iot-pillot.auth", "not-json");
    expect(readAuthSession(storage)).toBeNull();

    saveAuthSession(
      { access_token: "a", refresh_token: "r", user_id: 3 },
      storage,
    );
    clearAuthSession(storage);

    expect(readAuthSession(storage)).toBeNull();
  });

  it("returns null when the stored session is missing string token fields", () => {
    const storage = memoryStorage();

    storage.setItem(
      "iot-pillot.auth",
      JSON.stringify({ access_token: "a", user_id: 3 }),
    );
    expect(readAuthSession(storage)).toBeNull();

    storage.setItem(
      "iot-pillot.auth",
      JSON.stringify({ access_token: "a", refresh_token: 1, user_id: 3 }),
    );
    expect(readAuthSession(storage)).toBeNull();
  });

  it("keeps the original user id when the refresh endpoint returns -1", () => {
    const original = {
      access_token: "old-a",
      refresh_token: "old-r",
      user_id: 7,
    };

    expect(
      mergeAuthSession(original, {
        access_token: "new-a",
        refresh_token: "new-r",
        user_id: -1,
      }),
    ).toEqual({
      access_token: "new-a",
      refresh_token: "new-r",
      user_id: 7,
    });
  });

  it("clears local session even when the backend logout request fails", async () => {
    const readSession = vi.fn(() => ({
      access_token: "a",
      refresh_token: "r",
      user_id: 7,
    }));
    const requestLogout = vi.fn().mockRejectedValue(new Error("offline"));
    const clearSession = vi.fn();

    await expect(
      signOut({ readSession, requestLogout, clearSession }),
    ).rejects.toThrow("offline");
    expect(requestLogout).toHaveBeenCalledWith("a", "r");
    expect(clearSession).toHaveBeenCalledTimes(1);
  });
});
