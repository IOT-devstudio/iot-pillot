import { afterEach, describe, expect, it, vi } from "vitest";

import {
  archiveStudioMember,
  createStudioMember,
  fetchStudioMember,
  listStudioMembers,
  updateStudioMember,
} from "./studio-members";

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

describe("studio members API", () => {
  it("keeps the three-state is_active filter instead of dropping false", async () => {
    const fetch = network();

    await listStudioMembers({}, 1);
    await listStudioMembers({ is_active: true }, 2);
    // false = 「仅已归档」，是最容易被真值判断静默丢掉的那个分支。
    await listStudioMembers({ is_active: false, q: " 张三 ", direction: "front-end" }, 1);

    const urls = fetch.mock.calls.map((call) => String(call[0]));
    expect(urls[0]).toBe("/api/v1/admin/studio/members?page=1&page_size=20");
    expect(urls[1]).toContain("is_active=true");
    expect(urls[1]).toContain("page=2");
    expect(urls[2]).toContain("is_active=false");
    // 关键词去首尾空白后再下发，避免「 张三 」与「张三」查出不同结果。
    expect(urls[2]).toContain("q=%E5%BC%A0%E4%B8%89");
    expect(urls[2]).toContain("direction=front-end");
  });

  it("uses the admin studio endpoints with a bearer token", async () => {
    const fetch = network();

    await createStudioMember({
      name: "张三",
      birth_year: 2003,
      cohort: "2024 级",
      direction: "front-end",
      role_in_studio: "前端负责人",
    });
    await updateStudioMember(3, { role_in_studio: "成员" });
    await archiveStudioMember(3);
    await fetchStudioMember(3);

    const [createPath, createInit] = fetch.mock.calls[0]!;
    expect(createPath).toBe("/api/v1/admin/studio/members");
    expect(createInit?.method).toBe("POST");
    expect(new Headers(createInit?.headers).get("Authorization")).toBe("Bearer access");
    expect(JSON.parse(String(createInit?.body))).toMatchObject({ name: "张三" });

    expect(fetch.mock.calls[1]?.[0]).toBe("/api/v1/admin/studio/members/3");
    expect(fetch.mock.calls[1]?.[1]?.method).toBe("PUT");
    expect(fetch.mock.calls[2]?.[0]).toBe("/api/v1/admin/studio/members/3/archive");
    expect(fetch.mock.calls[2]?.[1]?.method).toBe("POST");
    expect(fetch.mock.calls[3]?.[0]).toBe("/api/v1/admin/studio/members/3");
  });
});
