import { afterEach, describe, expect, it, vi } from "vitest";

import {
  describeHealthError,
  fetchHealth,
  HEALTH_BAD_RESPONSE,
  HEALTH_UNREACHABLE,
} from "./health";

afterEach(() => vi.restoreAllMocks());

describe("health api", () => {
  it("requests the backend root health endpoint", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          data: { status: "ok", time: "2026-09-17T10:00:00Z" },
        }),
        { status: 200 },
      ),
    );

    await expect(fetchHealth()).resolves.toEqual({
      status: "ok",
      time: "2026-09-17T10:00:00Z",
    });
    // 5s 超时：后端挂起时不能让卡片永远停在「检测中…」
    expect(fetchMock).toHaveBeenCalledWith(
      "/health",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("maps network failure to the unreachable message, never the raw TypeError", async () => {
    // 网络不可达时 fetch 抛的是英文 TypeError，绝不能原样上屏
    vi.spyOn(globalThis, "fetch").mockRejectedValue(
      new TypeError("Failed to fetch"),
    );

    await expect(fetchHealth()).rejects.toThrow(HEALTH_UNREACHABLE);
    await expect(fetchHealth()).rejects.not.toThrow(/Failed to fetch/);
  });

  it("maps a non-JSON response (proxy HTML error page) to the unreachable message", async () => {
    // issue #56 的原始复现：代理打不到后端时回 500 + HTML，
    // response.json() 抛 "Unexpected token '<', \"<!DOCTYPE…\""
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("<!DOCTYPE html><html>502 Bad Gateway</html>", {
        status: 500,
        headers: { "Content-Type": "text/html" },
      }),
    );

    await expect(fetchHealth()).rejects.toThrow(HEALTH_UNREACHABLE);
    await expect(fetchHealth()).rejects.not.toThrow(/Unexpected token/);
  });

  it("maps a JSON error response to the bad-response message", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: 500, message: "内部错误" }), {
        status: 500,
      }),
    );

    await expect(fetchHealth()).rejects.toThrow(HEALTH_BAD_RESPONSE);
  });

  it("maps a 200 without data to the bad-response message", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({}), { status: 200 }),
    );

    await expect(fetchHealth()).rejects.toThrow(HEALTH_BAD_RESPONSE);
  });
});

describe("describeHealthError", () => {
  it("passes through classified messages", () => {
    expect(describeHealthError(new Error(HEALTH_UNREACHABLE))).toBe(
      HEALTH_UNREACHABLE,
    );
    expect(describeHealthError(new Error(HEALTH_BAD_RESPONSE))).toBe(
      HEALTH_BAD_RESPONSE,
    );
  });

  it("falls back for anything else, never leaking the original message", () => {
    // 引擎原始异常即使被误传进来，也只能落到兜底文案
    expect(describeHealthError(new SyntaxError("Unexpected token '<'"))).toBe(
      "暂时无法获取后端状态，请稍后重试",
    );
    expect(describeHealthError(new TypeError("Failed to fetch"))).toBe(
      "暂时无法获取后端状态，请稍后重试",
    );
    expect(describeHealthError("字符串异常")).toBe(
      "暂时无法获取后端状态，请稍后重试",
    );
    expect(describeHealthError(undefined)).toBe(
      "暂时无法获取后端状态，请稍后重试",
    );
  });
});
