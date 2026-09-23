import { describe, expect, it } from "vitest";

import type { AdminUser } from "@/api/admin";

import { fillDemoDetailWithFallbacks } from "./adminUserFixtures";

function user(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    user_id: 1,
    name: "Ada",
    created_at: "2026-09-17T10:00:00Z",
    is_admin: false,
    ...overrides,
  };
}

describe("fillDemoDetailWithFallbacks", () => {
  it("returns the user untouched when detail is already present", () => {
    const row = user({
      detail: { class: "iot 2301", qq: "1044696157", direction: "front-end" },
    });
    expect(fillDemoDetailWithFallbacks(row)).toBe(row);
  });

  it("fills a stable per-user_id demo detail for non-fallback rows", () => {
    const row = user({ user_id: 0 });
    const filled = fillDemoDetailWithFallbacks(row);
    expect(filled.detail).toBeDefined();
    // 同一 user_id 多次填充应当稳定
    const filledAgain = fillDemoDetailWithFallbacks(user({ user_id: 0 }));
    expect(filledAgain.detail).toEqual(filled.detail);
  });

  it("returns the empty-fallback for user_ids divisible by 7", () => {
    const filled = fillDemoDetailWithFallbacks(user({ user_id: 14 }));
    // 第 14 号：user_id % 3 = 2 → AGENT（默认填充）然后被 fallback 覆盖
    expect(filled.detail?.class).toBe("");
    expect(filled.detail?.direction).toBeNull();
    expect(filled.detail?.email).toBe("");
  });

  it("rotates the demo direction across front-end / back-end / agent", () => {
    // 避开 user_id % 7 === 0 的「未填」档位（0、7、14 都命中）
    const fe = fillDemoDetailWithFallbacks(user({ user_id: 3 })).detail?.direction;
    const be = fillDemoDetailWithFallbacks(user({ user_id: 4 })).detail?.direction;
    const ag = fillDemoDetailWithFallbacks(user({ user_id: 5 })).detail?.direction;
    expect(fe).toBe("front-end");
    expect(be).toBe("back-end");
    expect(ag).toBe("agent");
  });
});