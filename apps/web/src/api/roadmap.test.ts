import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createRoadmap,
  deleteRoadmap,
  fetchMyRoadmap,
  fetchRoadmap,
  listMyRoadmaps,
  listRoadmaps,
  markRoadmapProgress,
  updateRoadmap,
} from "./roadmap";

vi.mock("@/auth/session", async (original) => ({
  ...(await original<typeof import("@/auth/session")>()),
  readAuthSession: () => ({
    access_token: "test-access",
    refresh_token: "test-refresh",
    user_id: 7,
  }),
  saveAuthSession: vi.fn(),
  clearAuthSession: vi.fn(),
}));

afterEach(() => vi.restoreAllMocks());

function response(data: unknown, status = 200) {
  return new Response(
    JSON.stringify(
      status === 200
        ? { code: 0, message: "ok", data }
        : { code: 4000, message: "保存失败" },
    ),
    { status },
  );
}

/**
 * 学习路线 API 契约（#223）。
 *
 * 重点锁两件容易出错的事：
 *   1. 全部走 /api/v1/admin/studio/roadmaps（仅管理员分组，不是用户端路径）；
 *   2. 保存必须带上 version —— 漏带就会让乐观锁退化成「总是覆盖」。
 */
describe("roadmap api", () => {
  it("列表走管理员分组并按需拼查询参数", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ items: [], total: 0, directions: [] }));

    await listRoadmaps();
    await listRoadmaps({ direction: "front-end", page: 2, page_size: 10 });

    const paths = fetcher.mock.calls.map(([p]) => String(p));
    expect(paths[0]).toBe("/api/v1/admin/studio/roadmaps");
    expect(paths[1]).toContain("direction=front-end");
    expect(paths[1]).toContain("page=2");
    expect(paths[1]).toContain("page_size=10");
  });

  it("详情、创建、删除都在管理员分组下", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ id: 1 }));

    await fetchRoadmap(1);
    await createRoadmap({ title: "t", body: "b", direction_code: "front-end", stages: [] });
    await deleteRoadmap(1);

    const calls = fetcher.mock.calls;
    expect(String(calls[0]![0])).toBe("/api/v1/admin/studio/roadmaps/1");
    expect(String(calls[1]![0])).toBe("/api/v1/admin/studio/roadmaps");
    expect(calls[1]![1]?.method).toBe("POST");
    expect(String(calls[2]![0])).toBe("/api/v1/admin/studio/roadmaps/1");
    expect(calls[2]![1]?.method).toBe("DELETE");
  });

  it("保存带上 version（乐观锁）", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ id: 1, version: 4 }));

    await updateRoadmap(1, {
      title: "t",
      body: "b",
      direction_code: "front-end",
      stages: [],
      version: 3,
    });

    const [path, init] = fetcher.mock.calls[0]!;
    expect(String(path)).toBe("/api/v1/admin/studio/roadmaps/1");
    expect(init?.method).toBe("PUT");
    // 必须带 version：不带后端会拒绝，而不是默默覆盖别人的改动。
    expect(JSON.parse(String(init?.body)).version).toBe(3);
  });
});

/* ------------------------------------------------------------------ *
 * #224：用户端接口与进度标记
 * ------------------------------------------------------------------ */

describe("roadmap api — 用户端走 /learning 而不是 admin 分组", () => {
  it("列表与详情都在 /api/v1/learning/roadmaps 下", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ items: [], total: 0, directions: [] }));

    await listMyRoadmaps({ direction: "front-end" });
    await fetchMyRoadmap(10);

    const paths = fetcher.mock.calls.map(([p]) => String(p));
    expect(paths[0]).toContain("/api/v1/learning/roadmaps?");
    expect(paths[0]).toContain("direction=front-end");
    // 不能落到 admin 分组：那会把普通成员的读请求送进仅管理员的路径。
    expect(paths[0]).not.toContain("/admin/");
    expect(paths[1]).toBe("/api/v1/learning/roadmaps/10");
  });

  it("标记进度用 POST + stage_id，撤销传空串", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ roadmap_id: 10, stage_id: "s_aaaa1111" }));

    await markRoadmapProgress(10, "s_aaaa1111");
    await markRoadmapProgress(10, "");

    const first = fetcher.mock.calls[0]!;
    expect(String(first[0])).toBe("/api/v1/learning/roadmaps/10/progress");
    expect(first[1]?.method).toBe("POST");
    expect(JSON.parse(String(first[1]?.body))).toEqual({ stage_id: "s_aaaa1111" });

    const second = fetcher.mock.calls[1]!;
    // 再点同一阶段 = 撤销，请求体里 stage_id 是空串。
    expect(JSON.parse(String(second[1]?.body))).toEqual({ stage_id: "" });
  });
});
