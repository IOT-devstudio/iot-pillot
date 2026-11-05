import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createQaPost,
  createQaReply,
  deleteQaPost,
  deleteQaReply,
  fetchQaPost,
  listQaPosts,
  resolveQaPost,
  updateQaPost,
} from "./qa";

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
        : { code: 4004, message: "帖子不存在" },
    ),
    { status },
  );
}

/**
 * 内容域切路径（#221）。
 *
 * 后端两个域是**不同路径**（/api/v1/forum 与 /api/v1/studio/forum），不是同一
 * 端点加参数。这里锁住每个函数都真切了前缀——一旦漏切，内部论坛的请求会打到
 * 公开分组上，普通成员就能读到内部内容。
 */

describe("qa api — 默认落在招新问答分组", () => {
  it("列表、详情、写入都走 /api/v1/forum", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ items: [], total: 0 }));

    await listQaPosts({ page: 1 });
    await fetchQaPost(5);
    await createQaPost({ title: "t", body: "b" });
    await updateQaPost(5, { title: "t", body: "b" });
    await deleteQaPost(5);
    await createQaReply(5, { body: "r" });
    await deleteQaReply(9);
    await resolveQaPost(5, true);

    const paths = fetcher.mock.calls.map(([p]) => String(p));
    expect(paths.every((p) => p.startsWith("/api/v1/forum/"))).toBe(true);
    expect(paths.some((p) => p.includes("/studio/"))).toBe(false);
  });
});

describe("qa api — studio 域切到管理员分组", () => {
  it("所有调用都带 /api/v1/studio/forum 前缀", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ items: [], total: 0 }));

    await listQaPosts({ page: 1 }, "studio");
    await fetchQaPost(5, "studio");
    await createQaPost({ title: "t", body: "b" }, "studio");
    await updateQaPost(5, { title: "t", body: "b" }, "studio");
    await deleteQaPost(5, "studio");
    await createQaReply(5, { body: "r" }, "studio");
    await deleteQaReply(9, "studio");
    await resolveQaPost(5, true, "studio");

    const paths = fetcher.mock.calls.map(([p]) => String(p));
    // 一个都不能漏：漏了就等于越权访问公开分组
    expect(paths.every((p) => p.startsWith("/api/v1/studio/forum/"))).toBe(true);
  });
});

describe("qa api — 已解决筛选与标记", () => {
  it("三态：只有明确 true/false 才发 resolved 参数", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ items: [], total: 0 }));

    await listQaPosts({ resolved: true });
    await listQaPosts({ resolved: false });
    await listQaPosts({ resolved: null });
    await listQaPosts({});

    const paths = fetcher.mock.calls.map(([p]) => String(p));
    expect(paths[0]).toContain("resolved=true");
    expect(paths[1]).toContain("resolved=false");
    // null 与缺省都不带参数：false 不能兼作「不筛选」
    expect(paths[2]).not.toContain("resolved=");
    expect(paths[3]).not.toContain("resolved=");
  });

  it("标记已解决用 POST + JSON body", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ id: 5, is_resolved: true }));

    await resolveQaPost(5, true);

    const [path, init] = fetcher.mock.calls[0]!;
    expect(String(path)).toBe("/api/v1/forum/posts/5/resolve");
    expect(init?.method).toBe("POST");
    expect(init?.body).toBe(JSON.stringify({ resolved: true }));
  });
});
