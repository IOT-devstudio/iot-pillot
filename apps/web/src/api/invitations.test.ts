import { afterEach, describe, expect, it, vi } from "vitest";
import {
  acceptInvitation,
  advanceCandidate,
  getCandidate,
  listCandidates,
  listInvitations,
  myCandidates,
  myInvitations,
  revokeInvitation,
  sendInvitation,
} from "./invitations";
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
        : { code: 4000, message: "发送失败" },
    ),
    { status },
  );
}
describe("BE04 contracts", () => {
  it("uses authenticated list, detail and pagination endpoints", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({ items: [], total: 0 }));
    await listInvitations(2, 42);
    await listCandidates(3, "offered");
    await getCandidate(8);
    await myInvitations(7);
    await myCandidates(7);
    expect(fetcher.mock.calls.map(([path]) => path)).toEqual([
      "/api/v1/admin/invitations?page=2&page_size=20&application_id=42",
      "/api/v1/admin/candidates?page=3&page_size=20&status=offered",
      "/api/v1/admin/candidates/8",
      "/api/v1/me/invitations",
      "/api/v1/me/candidates",
    ]);
    for (const [, init] of fetcher.mock.calls)
      expect(new Headers(init?.headers).get("Authorization")).toBe(
        "Bearer test-access",
      );
  });
  it("sends exact server DTOs through business endpoints", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => response({}));
    const invite = {
      application_id: 42,
      template_id: 4,
      expires_in_days: 7,
      vars: { name: "Test" },
    };
    await sendInvitation(invite, 7);
    await revokeInvitation(9, 7);
    await acceptInvitation("test-invitation-token", 7);
    await advanceCandidate(
      8,
      { action: "send_offer", template_id: 5, note: "Passed" },
      7,
    );
    expect(
      fetcher.mock.calls.map(([path, init]) => [
        path,
        init?.method,
        JSON.parse(String(init?.body)),
      ]),
    ).toEqual([
      ["/api/v1/admin/invitations/send", "POST", invite],
      ["/api/v1/admin/invitations/9/revoke", "POST", {}],
      [
        "/api/v1/me/invitations/accept",
        "POST",
        { token: "test-invitation-token" },
      ],
      [
        "/api/v1/admin/candidates/8/advance",
        "POST",
        { action: "send_offer", template_id: 5, note: "Passed" },
      ],
    ]);
  });
  it("does not turn mail failure into success", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(response(null, 400));
    await expect(
      advanceCandidate(8, { action: "reject", template_id: 6 }, 7),
    ).rejects.toThrow();
  });
  it("blocks a write when the active account has changed", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch");
    await expect(
      acceptInvitation("test-invitation-token", 99),
    ).rejects.toThrow();
    expect(fetcher).not.toHaveBeenCalled();
  });
});
