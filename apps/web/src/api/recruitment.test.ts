/**
 * 招新接口封装的请求形状测试（issue #89）。
 *
 * mock 掉会话层（node 测试环境没有 localStorage），走真实的
 * session-requester 链路，锁住路径、方法、请求体与 Bearer 头——
 * 防止接入时把 snake_case 字段或管理端路径写错。
 */
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  fetchDirections,
  fetchMyApplications,
  reviewApplication,
  submitApplication,
} from "./recruitment";

vi.mock("@/auth/session", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/auth/session")>(),
  readAuthSession: () => ({
    access_token: "access-token",
    refresh_token: "refresh-token",
    user_id: 7,
  }),
  saveAuthSession: vi.fn(),
  clearAuthSession: vi.fn(),
}));

afterEach(() => {
  vi.restoreAllMocks();
});

/** 读出请求的 Bearer 头（headers 是 Headers 实例，不能用 objectContaining） */
function bearerOf(init: RequestInit | undefined): string | null {
  return new Headers(init?.headers).get("Authorization");
}

describe("recruitment api", () => {
  it("reads the direction list through the session requester", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "success",
          data: {
            items: [
              { id: 1, code: "front-end", name: "前端", open: true, sort_order: 1 },
            ],
          },
        }),
        { status: 200 },
      ),
    );

    await expect(fetchDirections()).resolves.toEqual({
      items: [
        { id: 1, code: "front-end", name: "前端", open: true, sort_order: 1 },
      ],
    });
    const [path, init] = fetchMock.mock.calls[0] ?? [];
    expect(path).toBe("/api/v1/recruitment/directions");
    expect(bearerOf(init)).toBe("Bearer access-token");
  });

  it("submits an application with the snake_case direction code", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "报名成功",
          data: { id: 11, direction_code: "back-end", status: "applied" },
        }),
        { status: 200 },
      ),
    );

    await expect(submitApplication("back-end")).resolves.toMatchObject({
      id: 11,
      status: "applied",
    });
    const [path, init] = fetchMock.mock.calls[0] ?? [];
    expect(path).toBe("/api/v1/recruitment/applications");
    expect(init?.method).toBe("POST");
    expect(init?.body).toBe(JSON.stringify({ direction_code: "back-end" }));
    expect(bearerOf(init)).toBe("Bearer access-token");
  });

  it("lists my applications", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "success",
          data: { items: [], total: 0, page: 1, page_size: 20 },
        }),
        { status: 200 },
      ),
    );

    await expect(fetchMyApplications()).resolves.toEqual({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
  });

  it("reviews an application with the target status", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: 0, message: "审核状态已更新" }), {
        status: 200,
      }),
    );

    await expect(reviewApplication(3, "reviewing")).resolves.toBe(
      "审核状态已更新",
    );
    const [path, init] = fetchMock.mock.calls[0] ?? [];
    expect(path).toBe("/api/v1/admin/recruitment/applications/3/review");
    expect(init?.method).toBe("PATCH");
    expect(init?.body).toBe(JSON.stringify({ target: "reviewing" }));
  });
});
