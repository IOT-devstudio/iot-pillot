import { describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";
import { landingFor, resolveRouteAccess, type RouteGateDeps } from "./guards";

/** 成员页：只有工作室成员（admin）能进 */
const MEMBER_PAGES = ["admin"] as const;
/** 普通用户页：成员也能访问，所以两个角色都在清单里 */
const PERSONAL_PAGES = ["admin", "member"] as const;

function makeDeps(overrides: Partial<RouteGateDeps> = {}): RouteGateDeps & {
  fetchRole: ReturnType<typeof vi.fn>;
  clearSession: ReturnType<typeof vi.fn>;
} {
  const fetchRole = vi.fn(async () => "admin");
  const clearSession = vi.fn();

  return {
    readToken: () => "token-abc",
    fetchRole,
    clearSession,
    ...overrides,
  } as RouteGateDeps & {
    fetchRole: ReturnType<typeof vi.fn>;
    clearSession: ReturnType<typeof vi.fn>;
  };
}

describe("resolveRouteAccess", () => {
  it("lets public routes through without asking the server", async () => {
    const deps = makeDeps();

    await expect(resolveRouteAccess([], "/", deps)).resolves.toBeNull();
    // 关键：公开路由不该因为守卫而多一次 /me 请求
    expect(deps.fetchRole).not.toHaveBeenCalled();
  });

  it("lets an admin into a member page", async () => {
    const deps = makeDeps();

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toBeNull();
    expect(deps.fetchRole).toHaveBeenCalledWith("token-abc");
  });

  it("sends an anonymous visitor to login and remembers the target", async () => {
    const deps = makeDeps({ readToken: () => null });

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({
      path: "/",
      query: { redirect: "/admin/buildings" },
    });
    expect(deps.fetchRole).not.toHaveBeenCalled();
  });

  it("redirects a plain user to their own landing page, not a dead end", async () => {
    const deps = makeDeps({ fetchRole: vi.fn(async () => "member") });

    // 普通用户访问成员页：落到 /user/home，而不是被挡在某个自己也进不去的页面
    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({ path: "/user/home" });
    // 角色是 member 说明令牌本身有效，不该清会话
    expect(deps.clearSession).not.toHaveBeenCalled();
  });

  it("lets a member into the personal page", async () => {
    // 访问规则是单向的：成员可以访问普通用户页面
    const deps = makeDeps({ fetchRole: vi.fn(async () => "member") });

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
      fetchRole: vi.fn(async () => {
        throw new AuthRequestError("token 已过期", 401, 4002);
      }),
    });

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({
      path: "/",
      query: { redirect: "/admin/buildings" },
    });
    expect(deps.clearSession).toHaveBeenCalledTimes(1);
  });

  it("does not clear the session on a network failure", async () => {
    // 网络抖动不该把用户的会话删掉，只按"无法确认"处理
    const deps = makeDeps({
      fetchRole: vi.fn(async () => {
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
      fetchRole: vi.fn(async () => {
        throw new AuthRequestError("无权限访问该资源", 403, 4003);
      }),
    });

    await resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps);

    expect(deps.clearSession).toHaveBeenCalledTimes(1);
  });

  it("does not let an unexpected error message leak as access", async () => {
    const deps = makeDeps({
      fetchRole: vi.fn(async () => {
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
    const deps = makeDeps({ fetchRole: vi.fn(async () => "superuser") });

    await expect(
      resolveRouteAccess(MEMBER_PAGES, "/admin/buildings", deps),
    ).resolves.toEqual({ path: "/user/home" });
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
