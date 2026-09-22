import { afterEach, describe, expect, it, vi } from "vitest";

import { describeHealthError, fetchHealth, HealthRequestError } from "./health";

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("fetchHealth", () => {
  it("returns the data payload on 200 OK", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
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
  });

  it("wraps fetch failures as HealthRequestError(network=true, status=0)", async () => {
    // 涵盖 TypeError: Failed to fetch 与 AbortSignal.timeout 触发两种来源；
    // fetchHealth 内部 try/catch 把它们归一为同一种 Error。
    vi.spyOn(globalThis, "fetch").mockRejectedValue(
      new TypeError("Failed to fetch"),
    );

    await expect(fetchHealth()).rejects.toMatchObject({
      name: "HealthRequestError",
      network: true,
      status: 0,
    });
  });

  it("wraps non-JSON responses (proxy returns HTML) as network error", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("<!DOCTYPE html><html>...", {
        status: 502,
        headers: { "content-type": "text/html" },
      }),
    );

    await expect(fetchHealth()).rejects.toMatchObject({
      name: "HealthRequestError",
      network: true,
      status: 502,
    });
  });

  it("wraps missing-data payload as backend-unhealthy error", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: 500, message: "boom" }), {
        status: 500,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(fetchHealth()).rejects.toMatchObject({
      name: "HealthRequestError",
      network: false,
      status: 500,
    });
  });
});

describe("describeHealthError", () => {
  it("returns '连接失败，无法访问后端服务' for network errors", () => {
    expect(
      describeHealthError(new HealthRequestError("any", true, 0)),
    ).toBe("连接失败，无法访问后端服务");
  });

  it("returns '后端服务响应异常' for backend-unhealthy errors", () => {
    expect(
      describeHealthError(new HealthRequestError("any", false, 500)),
    ).toBe("后端服务响应异常");
  });

  it("does not leak the original message for unknown errors", () => {
    expect(describeHealthError(new Error("Unexpected token <"))).toBe(
      "后端服务状态未知",
    );
    expect(describeHealthError("oops")).toBe("后端服务状态未知");
  });
});