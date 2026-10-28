import { afterEach, describe, expect, it, vi } from "vitest";

import { convertCandidateToMember } from "./studio-members";

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
      async () => new Response(JSON.stringify({ code: 0, data: { id: 1 } })),
    );
}

describe("convert candidate to member API", () => {
  it("posts only the fields the admin fills in", async () => {
    const fetch = network();

    await convertCandidateToMember({
      candidate_id: 55,
      birth_year: 2003,
      cohort: "2024 级",
      role_in_studio: "前端成员",
    });

    const [path, init] = fetch.mock.calls[0]!;
    expect(path).toBe("/api/v1/admin/studio/members/from-candidate");
    expect(init?.method).toBe("POST");
    expect(new Headers(init?.headers).get("Authorization")).toBe("Bearer access");

    const body = JSON.parse(String(init?.body));
    expect(body).toEqual({
      candidate_id: 55,
      birth_year: 2003,
      cohort: "2024 级",
      role_in_studio: "前端成员",
    });
    // 姓名与方向由后端从申请带过来，请求体里不该出现。
    expect("name" in body).toBe(false);
    expect("direction" in body).toBe(false);
  });
});
