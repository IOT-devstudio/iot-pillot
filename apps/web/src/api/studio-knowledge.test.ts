import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createStudioKnowledge,
  deleteStudioKnowledge,
  fetchStudioKnowledge,
  fetchStudioKnowledgeRevision,
  listStudioKnowledge,
  listStudioKnowledgeRevisions,
  updateStudioKnowledge,
} from "./studio-knowledge";

vi.mock("@/auth/session", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/auth/session")>()),
  readAuthSession: () => ({
    user_id: 7,
    access_token: "access",
    refresh_token: "refresh",
  }),
  clearAuthSession: vi.fn(),
  saveAuthSession: vi.fn(),
}));
afterEach(() => vi.restoreAllMocks());

function network() {
  return vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(
      async () =>
        new Response(JSON.stringify({ code: 0, data: { items: [], total: 0 } })),
    );
}

describe("studio knowledge API", () => {
  it("keeps the three-state parent_id filter instead of dropping null", async () => {
    const fetch = network();

    await listStudioKnowledge({}, 1);
    // null = 「只看顶层」，是最容易被真值判断静默丢掉的那个分支。
    await listStudioKnowledge({ parent_id: null }, 1);
    await listStudioKnowledge({ parent_id: 7, q: " 部署 ", permission: "admin" }, 2);

    const urls = fetch.mock.calls.map((call) => String(call[0]));
    expect(urls[0]).toBe("/api/v1/admin/studio/knowledge?page=1&page_size=20");
    expect(urls[1]).toContain("parent_id=none");
    expect(urls[2]).toContain("parent_id=7");
    expect(urls[2]).toContain("page=2");
    // 关键词去首尾空白后再下发。
    expect(urls[2]).toContain("q=%E9%83%A8%E7%BD%B2");
    expect(urls[2]).toContain("permission=admin");
  });

  it("omits the parent_id filter entirely when it is undefined", async () => {
    const fetch = network();
    await listStudioKnowledge({ q: "部署" }, 1);
    expect(String(fetch.mock.calls[0]?.[0])).not.toContain("parent_id");
  });

  it("uses the admin knowledge endpoints with a bearer token", async () => {
    const fetch = network();

    await createStudioKnowledge({ title: "部署手册", body: "正文" });
    await updateStudioKnowledge(3, { body: "新正文", version: 2 });
    await deleteStudioKnowledge(3);
    await fetchStudioKnowledge(3);
    await listStudioKnowledgeRevisions(3);
    await fetchStudioKnowledgeRevision(3, 1);

    const [createPath, createInit] = fetch.mock.calls[0]!;
    expect(createPath).toBe("/api/v1/admin/studio/knowledge");
    expect(createInit?.method).toBe("POST");
    expect(new Headers(createInit?.headers).get("Authorization")).toBe("Bearer access");

    const [updatePath, updateInit] = fetch.mock.calls[1]!;
    expect(updatePath).toBe("/api/v1/admin/studio/knowledge/3");
    expect(updateInit?.method).toBe("PUT");
    // version 必须真的进到请求体里，否则乐观锁形同虚设。
    expect(JSON.parse(String(updateInit?.body))).toEqual({
      body: "新正文",
      version: 2,
    });

    expect(fetch.mock.calls[2]?.[0]).toBe("/api/v1/admin/studio/knowledge/3");
    expect(fetch.mock.calls[2]?.[1]?.method).toBe("DELETE");
    expect(fetch.mock.calls[3]?.[0]).toBe("/api/v1/admin/studio/knowledge/3");
    expect(fetch.mock.calls[4]?.[0]).toBe(
      "/api/v1/admin/studio/knowledge/3/revisions",
    );
    expect(fetch.mock.calls[5]?.[0]).toBe(
      "/api/v1/admin/studio/knowledge/3/revisions/1",
    );
  });
});
