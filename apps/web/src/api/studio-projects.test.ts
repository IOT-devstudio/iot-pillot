import { afterEach, describe, expect, it, vi } from "vitest";

import {
  addStudioProjectMember,
  createStudioProject,
  deleteStudioProject,
  fetchStudioProject,
  listStudioProjectMembers,
  listStudioProjects,
  removeStudioProjectMember,
  updateStudioProject,
  updateStudioProjectMember,
} from "./studio-projects";

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
      async () => new Response(JSON.stringify({ code: 0, data: { items: [], total: 0 } })),
    );
}

describe("studio projects API", () => {
  it("keeps empty filter values out of the query", async () => {
    const fetch = network();
    await listStudioProjects({}, 1);
    expect(String(fetch.mock.calls[0]?.[0])).toBe(
      "/api/v1/admin/studio/projects?page=1&page_size=20",
    );
  });

  it("sends only the filter terms that were provided", async () => {
    const fetch = network();
    await listStudioProjects({ q: " 导览 ", status: "active", year: " 2024 " }, 3, 10);
    const url = String(fetch.mock.calls[0]?.[0]);
    expect(url).toContain("q=%E5%AF%BC%E8%A7%88");
    expect(url).toContain("status=active");
    expect(url).toContain("year=2024");
    expect(url).toContain("page=3");
    expect(url).toContain("page_size=10");
  });

  it("omits the status filter when it is the empty string", async () => {
    const fetch = network();
    await listStudioProjects({ status: "" }, 1);
    expect(String(fetch.mock.calls[0]?.[0])).not.toContain("status=");
  });

  it("uses the admin project endpoints with a bearer token", async () => {
    const fetch = network();

    await createStudioProject({ name: "校园导览小程序", year: "2024" });
    await updateStudioProject(1, { status: "completed" });
    await fetchStudioProject(1);
    await deleteStudioProject(1);
    await listStudioProjectMembers(1);
    await addStudioProjectMember(1, { member_id: 7, name: "张三", role: "lead" });
    await updateStudioProjectMember(1, 100, { role: "contributor" });
    await removeStudioProjectMember(1, 100);

    const [createPath, createInit] = fetch.mock.calls[0]!;
    expect(createPath).toBe("/api/v1/admin/studio/projects");
    expect(createInit?.method).toBe("POST");
    expect(new Headers(createInit?.headers).get("Authorization")).toBe("Bearer access");
    expect(JSON.parse(String(createInit?.body))).toEqual({
      name: "校园导览小程序",
      year: "2024",
    });

    expect(fetch.mock.calls[1]?.[0]).toBe("/api/v1/admin/studio/projects/1");
    expect(fetch.mock.calls[1]?.[1]?.method).toBe("PUT");
    expect(JSON.parse(String(fetch.mock.calls[1]?.[1]?.body))).toEqual({
      status: "completed",
    });

    expect(fetch.mock.calls[2]?.[0]).toBe("/api/v1/admin/studio/projects/1");

    // 删除走 message 出口：后端返回 OKWithMsg(..., nil)，data 被省略。
    expect(fetch.mock.calls[3]?.[0]).toBe("/api/v1/admin/studio/projects/1");
    expect(fetch.mock.calls[3]?.[1]?.method).toBe("DELETE");

    expect(fetch.mock.calls[4]?.[0]).toBe("/api/v1/admin/studio/projects/1/members");
    expect(fetch.mock.calls[5]?.[0]).toBe("/api/v1/admin/studio/projects/1/members");
    expect(fetch.mock.calls[5]?.[1]?.method).toBe("POST");
    expect(JSON.parse(String(fetch.mock.calls[5]?.[1]?.body))).toEqual({
      member_id: 7,
      name: "张三",
      role: "lead",
    });

    expect(fetch.mock.calls[6]?.[0]).toBe(
      "/api/v1/admin/studio/projects/1/members/100",
    );
    expect(fetch.mock.calls[6]?.[1]?.method).toBe("PUT");

    expect(fetch.mock.calls[7]?.[0]).toBe(
      "/api/v1/admin/studio/projects/1/members/100",
    );
    expect(fetch.mock.calls[7]?.[1]?.method).toBe("DELETE");
  });
});
