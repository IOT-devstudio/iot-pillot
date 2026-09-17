import { describe, expect, it, vi } from "vitest";
import { defaultNavigateTarget, submitAuth } from "./submit";

const session = { access_token: "a", refresh_token: "r", user_id: 9 };

describe("submitAuth", () => {
  it("returns validation errors without calling an API", async () => {
    const deps = {
      login: vi.fn(),
      register: vi.fn(),
      saveSession: vi.fn(),
      navigate: vi.fn(),
    };

    await expect(
      submitAuth("login", { username: "", password: "123" }, deps),
    ).resolves.toEqual({
      ok: false,
      errors: { username: "请输入用户名", password: "密码至少 6 位" },
    });
    expect(deps.login).not.toHaveBeenCalled();
    expect(deps.register).not.toHaveBeenCalled();
    expect(deps.saveSession).not.toHaveBeenCalled();
    expect(deps.navigate).not.toHaveBeenCalled();
  });

  it("saves a successful registration session and navigates to dashboard", async () => {
    const deps = {
      login: vi.fn(),
      register: vi.fn().mockResolvedValue(session),
      saveSession: vi.fn(),
      navigate: vi.fn(),
    };
    const form = {
      name: "Ada",
      email: "ada@example.com",
      password: "123456",
      confirmPassword: "123456",
      code: "123456",
    };

    await expect(submitAuth("register", form, deps)).resolves.toEqual({
      ok: true,
      session,
    });
    expect(deps.register).toHaveBeenCalledWith({
      name: "Ada",
      email: "ada@example.com",
      password: "123456",
      code: "123456",
    });
    expect(deps.saveSession).toHaveBeenCalledWith(session);
    // 未带 redirect 时落到默认目标，再由守卫按角色分流
    expect(deps.navigate).toHaveBeenCalledWith("/dashboard");
  });

  it("returns to the redirect target after login", async () => {
    const deps = {
      login: vi.fn().mockResolvedValue(session),
      register: vi.fn(),
      saveSession: vi.fn(),
      navigate: vi.fn(),
    };

    await expect(
      submitAuth(
        "login",
        { username: "ada", password: "123456" },
        deps,
        "/admin/buildings",
      ),
    ).resolves.toEqual({ ok: true, session });

    expect(deps.navigate).toHaveBeenCalledWith("/admin/buildings");
  });

  it("propagates API errors without saving or navigating", async () => {
    const error = new Error("login failed");
    const deps = {
      login: vi.fn().mockRejectedValue(error),
      register: vi.fn(),
      saveSession: vi.fn(),
      navigate: vi.fn(),
    };

    await expect(
      submitAuth("login", { username: "ada", password: "123456" }, deps),
    ).rejects.toBe(error);
    expect(deps.saveSession).not.toHaveBeenCalled();
    expect(deps.navigate).not.toHaveBeenCalled();
  });
});

describe("defaultNavigateTarget", () => {
  it("falls back to the dashboard when there is no redirect", () => {
    expect(defaultNavigateTarget()).toBe("/dashboard");
    expect(defaultNavigateTarget("")).toBe("/dashboard");
  });

  it("uses a same-site redirect path", () => {
    expect(defaultNavigateTarget("/admin/buildings")).toBe("/admin/buildings");
    expect(defaultNavigateTarget("/forms?a=1")).toBe("/forms?a=1");
  });

  it("rejects a protocol-relative redirect", () => {
    // "//evil.com" 会被浏览器当成站外地址，放行等于开放重定向漏洞
    expect(defaultNavigateTarget("//evil.com")).toBe("/dashboard");
  });

  it("rejects an absolute external redirect", () => {
    expect(defaultNavigateTarget("https://evil.com")).toBe("/dashboard");
  });

  it("rejects a relative redirect without a leading slash", () => {
    expect(defaultNavigateTarget("admin/buildings")).toBe("/dashboard");
  });
});
