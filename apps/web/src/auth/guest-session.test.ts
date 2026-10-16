import { describe, expect, it, vi, afterEach } from "vitest";
import { nextTick } from "vue";
import { currentSession, syncAuthSession, saveAuthSession, clearAuthSession } from "./session";
import { requestForPublicPage } from "@/api/public-page-request";

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

function storage() {
  const data = new Map<string, string>();
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => { data.set(key, value); },
    removeItem: (key: string) => { data.delete(key); },
  };
}

describe("guest session", () => {
  it("reacts to sign-in, token rotation and clearing", async () => {
    const localStorage = storage();
    vi.stubGlobal("window", { localStorage });
    syncAuthSession();
    expect(currentSession.value).toBeNull();
    saveAuthSession({ access_token: "a", refresh_token: "r", user_id: 1 });
    await nextTick();
    expect(currentSession.value?.user_id).toBe(1);
    clearAuthSession();
    expect(currentSession.value).toBeNull();
  });

  it.each([401, 403])("clears a rejected session (%s) without navigating", async (status) => {
    const assign = vi.fn();
    vi.stubGlobal("window", { localStorage: storage(), location: { assign } });
    saveAuthSession({ access_token: "a", refresh_token: "r", user_id: 1 });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(
      JSON.stringify({ code: 4011, message: "登录失效" }), { status },
    ));
    await expect(requestForPublicPage("/api/v1/me")).rejects.toMatchObject({ status });
    expect(currentSession.value).toBeNull();
    expect(assign).not.toHaveBeenCalled();
  });

  it("preserves the session on network failure", async () => {
    vi.stubGlobal("window", { localStorage: storage() });
    saveAuthSession({ access_token: "a", refresh_token: "r", user_id: 1 });
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("offline"));
    await expect(requestForPublicPage("/api/v1/me")).rejects.toMatchObject({ status: 0 });
    expect(currentSession.value?.access_token).toBe("a");
  });

  it("does not clear a newer session when an old request is rejected", async () => {
    vi.stubGlobal("window", { localStorage: storage() });
    saveAuthSession({ access_token: "old", refresh_token: "r", user_id: 1 });
    let rejectRequest!: (response: Response) => void;
    vi.spyOn(globalThis, "fetch").mockImplementation(() => new Promise<Response>(resolve => { rejectRequest = resolve; }));
    const result = requestForPublicPage("/api/v1/me");
    saveAuthSession({ access_token: "new", refresh_token: "r2", user_id: 2 });
    rejectRequest(new Response(JSON.stringify({ code: 4011, message: "失效" }), { status: 401 }));
    await expect(result).rejects.toMatchObject({ status: 401 });
    expect(currentSession.value?.access_token).toBe("new");
    expect(currentSession.value?.user_id).toBe(2);
  });

  it("blocks a write if storage changed accounts before the storage event", async () => {
    const localStorage = storage();
    vi.stubGlobal("window", { localStorage });
    saveAuthSession({ access_token: "old", refresh_token: "r", user_id: 1 });
    localStorage.setItem("iot-pillot.auth", JSON.stringify({ access_token: "new", refresh_token: "r2", user_id: 2 }));
    const fetch = vi.spyOn(globalThis, "fetch");
    await expect(requestForPublicPage("/api/v1/recruitment/applications", { method: "POST" }, 1))
      .rejects.toMatchObject({ status: 409 });
    expect(fetch).not.toHaveBeenCalled();
    expect(currentSession.value?.user_id).toBe(2);
  });

  it("rejects stale successful reads before a delayed storage event", async () => {
    const localStorage = storage();
    vi.stubGlobal("window", { localStorage });
    saveAuthSession({ access_token: "old", refresh_token: "r", user_id: 1 });
    let finish!: (response: Response) => void;
    vi.spyOn(globalThis, "fetch").mockImplementation(() => new Promise<Response>(resolve => { finish = resolve; }));
    const result = requestForPublicPage("/api/v1/me");
    localStorage.setItem("iot-pillot.auth", JSON.stringify({ access_token: "new", refresh_token: "r2", user_id: 2 }));
    finish(new Response(JSON.stringify({ code: 0, data: { user_id: 1 } }), { status: 200 }));
    await expect(result).rejects.toMatchObject({ status: 409 });
    expect(currentSession.value?.user_id).toBe(2);
  });
});
