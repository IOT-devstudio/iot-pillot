import { afterEach, describe, expect, it, vi } from "vitest";
import {
  listAdminUsers,
  login,
  logout,
  refreshAuthSession,
  register,
  sendVerifyCode,
} from "./auth";

afterEach(() => vi.restoreAllMocks());

describe("auth api", () => {
  it("posts login credentials to the backend and returns data", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "success",
          data: { access_token: "a", refresh_token: "r", user_id: 7 },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(
      login({ username: "ada", password: "123456" }),
    ).resolves.toEqual({
      access_token: "a",
      refresh_token: "r",
      user_id: 7,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/login",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ username: "ada", password: "123456" }),
      }),
    );
  });

  it("posts the registration DTO and never calls the verification-code endpoint", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "success",
          data: { access_token: "a", refresh_token: "r", user_id: 8 },
        }),
        { status: 200 },
      ),
    );

    await register({
      name: "Ada",
      email: "ada@example.com",
      password: "123456",
      code: "123456",
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/register",
      expect.objectContaining({
        body: JSON.stringify({
          name: "Ada",
          email: "ada@example.com",
          password: "123456",
          code: "123456",
        }),
      }),
    );
  });

  it("throws the backend message for a failed response", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: 4001, message: "验证码错误" }), {
        status: 400,
      }),
    );

    await expect(
      register({
        name: "Ada",
        email: "ada@example.com",
        password: "123456",
        code: "123456",
      }),
    ).rejects.toThrow("验证码错误");
  });

  it("converts network failures to the unavailable message", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("offline"));

    await expect(
      login({ username: "ada", password: "123456" }),
    ).rejects.toThrow("服务暂时不可用，请稍后重试");
  });

  it("converts malformed backend JSON to the unavailable message", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("not json", { status: 200 }),
    );

    await expect(
      login({ username: "ada", password: "123456" }),
    ).rejects.toThrow("服务暂时不可用，请稍后重试");
  });

  it("posts the verifier and accepts a response without data", async () => {
    // 后端 SendVerifyCode 走 OKWithMsg(..., nil)，data 被 omitempty 丢掉，
    // 所以这里刻意不返回 data —— 不能因为缺 data 就当成服务不可用。
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: 0, message: "发送验证码成功" }), {
        status: 200,
      }),
    );

    await expect(sendVerifyCode("ada@example.com")).resolves.toBe(
      "发送验证码成功",
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/send-verify-code",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          verifier: "ada@example.com",
          verifier_type: "email",
        }),
      }),
    );
  });

  it("surfaces the backend message when sending a code fails", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: 4001, message: "验证码发送失败" }), {
        status: 400,
      }),
    );

    await expect(sendVerifyCode("ada@example.com")).rejects.toThrow(
      "验证码发送失败",
    );
  });

  it("refreshes the session through the backend refresh endpoint", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "success",
          data: { access_token: "new-a", refresh_token: "new-r", user_id: -1 },
        }),
        { status: 200 },
      ),
    );

    await expect(refreshAuthSession("old-r")).resolves.toEqual({
      access_token: "new-a",
      refresh_token: "new-r",
      user_id: -1,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/refresh",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ refresh_token: "old-r" }),
      }),
    );
  });

  it("logs out with both bearer and refresh tokens", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: 0, message: "退出登录成功" }), {
        status: 200,
      }),
    );

    await expect(logout("access-token", "refresh-token")).resolves.toBe(
      "退出登录成功",
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/logout",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ refresh_token: "refresh-token" }),
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer access-token",
        },
      }),
    );
  });

  it("loads the paginated admin user list with the access token", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "success",
          data: {
            items: [
              { user_id: 7, name: "Ada", created_at: "2026-09-17T10:00:00Z" },
            ],
            total: 1,
            page: 2,
            page_size: 20,
          },
        }),
        { status: 200 },
      ),
    );

    await expect(listAdminUsers("access-token", 2, 20)).resolves.toMatchObject({
      total: 1,
      page: 2,
      page_size: 20,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/users?page=2&page_size=20",
      expect.objectContaining({
        method: "GET",
        headers: { Authorization: "Bearer access-token" },
      }),
    );
  });
});
