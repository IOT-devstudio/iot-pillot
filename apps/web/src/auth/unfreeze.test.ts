import { describe, expect, it, vi } from "vitest";

import type { AuthSession } from "@/api/auth";
import { mergeAuthSession } from "./session";
import { PROFILE_UNFREEZE_PATH, submitAuth } from "./submit";

/* ------------------------------------------------------------------ *
 * 0012 自助解冻：冻结账号的登录落点与刷新不升权
 *
 * 这组盯的是「静默升权」：受限令牌一旦被当成完整令牌，
 * 资料不全的账号就能照常用所有功能，而冻结形同虚设。
 * ------------------------------------------------------------------ */

function restrictedSession(overrides: Partial<AuthSession> = {}): AuthSession {
  return {
    access_token: "a",
    refresh_token: "r",
    user_id: 7,
    scope: "profile_only",
    frozen: true,
    freeze_issues: [{ field: "class", message: "班级未填写" }],
    ...overrides,
  };
}

function baseDeps(session: AuthSession) {
  const navigate = vi.fn(async () => undefined);
  const saveSession = vi.fn();
  const checkProfile = vi.fn(async () => ({ user_id: session.user_id, role: "member" }));
  return {
    deps: {
      login: vi.fn(async () => session),
      register: vi.fn(async () => session),
      saveSession,
      navigate,
      checkProfile,
    },
    navigate,
    saveSession,
    checkProfile,
  };
}

describe("submitAuth — 冻结会话的落点", () => {
  it("受限会话直接进补资料页，且忽略 redirect", async () => {
    const { deps, navigate } = baseDeps(restrictedSession());

    const result = await submitAuth(
      "login",
      { username: "u", password: "secret123" } as never,
      deps as never,
      // 目标页几乎一定访问不了，不能把人带过去看一片 403。
      "/user/qa",
    );

    expect(result.ok).toBe(true);
    expect(navigate).toHaveBeenCalledWith(PROFILE_UNFREEZE_PATH);
  });

  it("受限会话不会再去查 profile（省一次注定受限的请求）", async () => {
    const { deps, checkProfile } = baseDeps(restrictedSession());
    await submitAuth("login", { username: "u", password: "secret123" } as never, deps as never);
    expect(checkProfile).not.toHaveBeenCalled();
  });

  it("正常会话仍走原来的默认落点", async () => {
    const { deps, navigate } = baseDeps(
      restrictedSession({ scope: "full", frozen: false, freeze_issues: undefined }),
    );

    await submitAuth("login", { username: "u", password: "secret123" } as never, deps as never);

    expect(navigate).not.toHaveBeenCalledWith(PROFILE_UNFREEZE_PATH);
  });
});

describe("mergeAuthSession — 刷新不升权", () => {
  it("后端明确返回 profile_only 时保持受限", () => {
    const merged = mergeAuthSession(restrictedSession(), {
      access_token: "a2",
      refresh_token: "r2",
      scope: "profile_only",
    });
    expect(merged.scope).toBe("profile_only");
    expect(merged.frozen).toBe(true);
  });

  it("后端没返回 scope 时沿用本地的受限状态，不静默升权", () => {
    // 这条是核心：如果刷新响应缺 scope 就把 scope 抹成 undefined，
    // 前端会当成"正常会话"，受限用户瞬间拿到全部入口。
    const merged = mergeAuthSession(restrictedSession(), {
      access_token: "a2",
      refresh_token: "r2",
    });
    expect(merged.scope).toBe("profile_only");
    expect(merged.frozen).toBe(true);
    expect(merged.freeze_issues).toHaveLength(1);
  });

  it("令牌更新但用户 ID 沿用本地", () => {
    const merged = mergeAuthSession(restrictedSession(), {
      access_token: "new-a",
      refresh_token: "new-r",
      scope: "profile_only",
    });
    expect(merged.access_token).toBe("new-a");
    expect(merged.user_id).toBe(7);
  });

  it("已解冻的会话刷新后不会被重新标成冻结", () => {
    const unfrozen: AuthSession = {
      access_token: "a",
      refresh_token: "r",
      user_id: 7,
      scope: "full",
      frozen: false,
    };
    const merged = mergeAuthSession(unfrozen, {
      access_token: "a2",
      refresh_token: "r2",
      scope: "full",
    });
    expect(merged.scope).toBe("full");
    expect(merged.frozen).toBe(false);
  });
});
