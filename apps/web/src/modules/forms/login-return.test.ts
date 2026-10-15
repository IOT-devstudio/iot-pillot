import { describe, expect, it } from "vitest";
import { sessionLoginPath } from "@/api/session-request";
import { resolveRouteAccess } from "@/router/guards";
import { defaultNavigateTarget } from "@/auth/submit";

describe("share link login return", () => {
  it("preserves the exact protected share target for guests and after login", async () => {
    const target = "/f/abc?source=share";
    expect(
      await resolveRouteAccess(["admin", "member"], target, {
        readToken: () => null,
        fetchRole: async () => "member",
        clearSession: () => {},
      }),
    ).toEqual({ path: "/", query: { redirect: target } });
    expect(defaultNavigateTarget(target)).toBe(target);
  });
  it("keeps the share destination when a request discovers an expired session", () => {
    const url = new URL(
      sessionLoginPath("expired", "/f/abc?source=share"),
      "https://test.local",
    );
    expect(url.searchParams.get("redirect")).toBe("/f/abc?source=share");
    expect(url.searchParams.get("reason")).toBe("expired");
    expect(
      new URL(
        sessionLoginPath("invalidated", "//evil.test"),
        "https://test.local",
      ).searchParams.has("redirect"),
    ).toBe(false);
  });
});
