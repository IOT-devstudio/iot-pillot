/**
 * home 模块 profileFixture 的测试。
 *
 * 与 dashboard/adminUserFixtures.test.ts 同性质：测的是「派生函数行为」，
 * 验证派生规则按 user_id 取模时输出稳定、特殊 user_id 触发空态、不会
 * 因为共享引用让一次 mutate 影响下一次派生。
 */
import { describe, expect, it } from "vitest";

import type { CurrentUser } from "@/api/admin";

import {
  fillDemoProfile,
  fillDemoProfileWithFallbacks,
} from "./profileFixture";

function user(userId: number): CurrentUser {
  return { user_id: userId, username: `u${userId}`, role: "member" };
}

describe("fillDemoProfile", () => {
  it("preserves user_id / username / role from input", () => {
    const out = fillDemoProfile(user(42));

    expect(out.user_id).toBe(42);
    expect(out.username).toBe("u42");
    expect(out.role).toBe("member");
  });

  it("gives a stable direction per user_id (same input → same output)", () => {
    const a = fillDemoProfile(user(7));
    const b = fillDemoProfile(user(7));

    expect(a.detail.direction).toBe(b.detail.direction);
  });

  it("covers more than one direction so the demo isn't all front-end", () => {
    const directions = new Set(
      [0, 1, 2, 3, 4, 5, 6, 7, 8].map(
        (id) => fillDemoProfile(user(id)).detail.direction,
      ),
    );

    expect(directions.size).toBeGreaterThan(1);
  });

  it("returns a fresh detail object on each call (no shared mutation)", () => {
    const a = fillDemoProfile(user(3));

    a.detail.class = "mutated";

    const b = fillDemoProfile(user(3));

    expect(b.detail.class).not.toBe("mutated");
  });
});

describe("fillDemoProfileWithFallbacks", () => {
  it("gives an empty detail when user_id % 7 === 0", () => {
    const out = fillDemoProfileWithFallbacks(user(7));

    expect(out.detail.class).toBe("");
    expect(out.detail.qq).toBe("");
    expect(out.detail.direction).toBeNull();
  });

  it("delegates to fillDemoProfile for non-fallback user_ids", () => {
    const a = fillDemoProfile(user(8));
    const b = fillDemoProfileWithFallbacks(user(8));

    expect(b).toEqual(a);
  });

  it("does not share fallback object across calls (no aliasing)", () => {
    const a = fillDemoProfileWithFallbacks(user(7));

    a.detail.class = "mutated";

    const b = fillDemoProfileWithFallbacks(user(7));

    expect(b.detail.class).toBe("");
  });
});