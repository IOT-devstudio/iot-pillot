import { afterEach, describe, expect, it, vi } from "vitest";
import { login, register } from "./auth";

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
});
