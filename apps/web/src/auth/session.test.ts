import type { AuthSession } from "@/api/auth";
import { describe, expect, it } from "vitest";
import { clearAuthSession, readAuthSession, saveAuthSession } from "./session";

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
});
