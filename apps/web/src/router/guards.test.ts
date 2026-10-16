import { describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";
import {
  FOUNDING_SETTINGS_BLOCKED,
  landingFor,
  resolveRouteAccess,
  type RouteGateDeps,
  type RouteIdentity,
} from "./guards";

/** 成员页：只有工作室成员（admin）能进 */
const MEMBER_PAGES = ["admin"] as const;
/** 普通用户页：成员也能访问，所以两个角色都在清单里 */
const PERSONAL_PAGES = ["admin", "member"] as const;

/** 构造身份切片；默认是普通管理员（非创始） */
function identity(
  role: string,
  isFounding = false,
): RouteIdentity {
  return { role, isFounding };
}

function makeDeps(overrides: Partial<RouteGateDeps> = {}): RouteGateDeps & {
  fetchIdentity: ReturnType<typeof vi.fn>;
  clearSession: ReturnType<typeof vi.fn>;
} {
  const fetchIdentity = vi.fn(async () => identity("admin"));
  const clearSession = vi.fn();

  return {
    readToken: () => "token-abc",
    fetchIdentity,
    clearSession,
    ...overrides,
  } as RouteGateDeps & {
    fetchIdentity: ReturnType<typeof vi.fn>;
    clearSession: ReturnType<typeof vi.fn>;
  };
}

describe("resolveRouteAccess", () => {
  it("preserves an invitation link through anonymous login", async () => {
    const deps = makeDeps({ readToken: () => null });
    const target = "/user/invitations?token=test-invitation-token";
    await expect(
      resolveRouteAccess(PERSONAL_PAGES, target, deps),
    ).resolves.toEqual({ path: "/", query: { redirect: target } });
  });
  it("lets public routes through without asking the server", async () => {
    const deps = makeDeps();

    await expect(resolveRouteAccess([], "/", deps)).resolves.toBeNull();
    // 关键：公开路由不该因为守卫而多一次 /me 请求
    expect(deps.fetchIdentity).not.toHaveBeenCalled();
  });

  it("lets an admin into a member page", async () => {
    const deps = makeDeps();

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toBeNull();
    expect(deps.fetchIdentity).toHaveBeenCalledWith("token-abc");
  });

  it("sends an anonymous visitor to login and remembers the target", async () => {
    const deps = makeDeps({ readToken: () => null });

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({
      path: "/",
      query: { redirect: "/admin/buildings" },
    });
    expect(deps.fetchIdentity).not.toHaveBeenCalled();
  });

  it("redirects a plain user to their own landing page, not a dead end", async () => {
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => identity("member")),
    });

    // 普通用户访问成员页：落到 /user/home，而不是被挡在某个自己也进不去的页面
    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({ path: "/user/home" });
    // 角色是 member 说明令牌本身有效，不该清会话
    expect(deps.clearSession).not.toHaveBeenCalled();
  });

  it("lets a member into the personal page", async () => {
    // 访问规则是单向的：成员可以访问普通用户页面
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => identity("member")),
    });

    await expect(
      resolveRouteAccess(PERSONAL_PAGES, "/user/home", deps),
    ).resolves.toBeNull();
  });

  it("lets an admin into the personal page too", async () => {
    const deps = makeDeps();

    await expect(
      resolveRouteAccess(PERSONAL_PAGES, "/user/home", deps),
    ).resolves.toBeNull();
  });

  it("clears the session when the server rejects the token", async () => {
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => {
        throw new AuthRequestError("token 已过期", 401, 4002);
      }),
    });

    // 4010/未细分的 401：reason=expired，登录页出「已过期」提示条
    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({
      path: "/",
      query: { redirect: "/admin/buildings", reason: "expired" },
    });
    expect(deps.clearSession).toHaveBeenCalledTimes(1);
  });

  it("carries the invalidated reason for business code 4011", async () => {
    // issue #84 契约：4011 = 会话被撤销（登出/顶替/改密），提示要区分
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => {
        throw new AuthRequestError("会话已失效", 401, 4011);
      }),
    });

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({
      path: "/",
      query: { redirect: "/admin/buildings", reason: "invalidated" },
    });
    expect(deps.clearSession).toHaveBeenCalledTimes(1);
  });

  it("does not clear the session on a network failure", async () => {
    // 网络抖动不该把用户的会话删掉，只按"无法确认"处理
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => {
        throw new AuthRequestError("服务暂时不可用，请稍后重试", 0);
      }),
    });

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({
      path: "/",
      query: { redirect: "/admin/buildings" },
    });
    expect(deps.clearSession).not.toHaveBeenCalled();
  });

  it("treats 403 as a rejected token", async () => {
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => {
        throw new AuthRequestError("无权限访问该资源", 403, 4003);
      }),
    });

    await resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps);

    expect(deps.clearSession).toHaveBeenCalledTimes(1);
  });

  it("does not let an unexpected error message leak as access", async () => {
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => {
        throw new Error("boom");
      }),
    });

    // 任何异常都必须落到"拒绝"，绝不能因为抛错就放行
    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.not.toBeNull();
  });

  it("treats an unknown role as a plain user", async () => {
    // 服务端将来新增角色时，默认给最小可见范围，而不是放行或卡死在登录页
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => identity("superuser")),
    });

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({ path: "/user/home" });
  });

  // ── 创始管理员拦截（issue #138）──

  it("blocks a founding admin from a denyFounding page", async () => {
    // 创始管理员没有个人设置页：拦截并回用户首页 + notice
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => identity("admin", true)),
    });

    await expect(
      resolveRouteAccess(
        PERSONAL_PAGES,
        "/user/settings",
        deps,
        /* denyFounding */ true,
      ),
    ).resolves.toEqual({
      path: "/user/home",
      query: { notice: FOUNDING_SETTINGS_BLOCKED },
    });
    // 令牌有效（角色放行后才命中创始拦截），不该清会话
    expect(deps.clearSession).not.toHaveBeenCalled();
  });

  it("lets a non-founding admin into a denyFounding page", async () => {
    const deps = makeDeps();

    await expect(
      resolveRouteAccess(
        PERSONAL_PAGES,
        "/user/settings",
        deps,
        /* denyFounding */ true,
      ),
    ).resolves.toBeNull();
  });

  it("lets a founding admin into pages without denyFounding", async () => {
    // 拦截只作用于显式标记 denyFounding 的路由，不影响创始管理员的其他页面
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => identity("admin", true)),
    });

    await expect(
      resolveRouteAccess(PERSONAL_PAGES, "/user/home", deps),
    ).resolves.toBeNull();
  });

  it("does not apply founding block to public routes", async () => {
    // 公开路由直接放行，连 /me 都不查，自然也不会命中创始拦截
    const deps = makeDeps({
      fetchIdentity: vi.fn(async () => identity("admin", true)),
    });

    await expect(
      resolveRouteAccess([], "/user/settings", deps, true),
    ).resolves.toBeNull();
    expect(deps.fetchIdentity).not.toHaveBeenCalled();
  });
});

describe("landingFor", () => {
  it("sends an admin to the dashboard", () => {
    expect(landingFor("admin")).toBe("/dashboard");
  });

  it("sends a plain user to the personal page", () => {
    expect(landingFor("member")).toBe("/user/home");
  });

  it("falls back to the personal page for an unknown role", () => {
    expect(landingFor("superuser")).toBe("/user/home");
  });
});
