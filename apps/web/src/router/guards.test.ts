import { describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";
import { resolveAdminAccess, type AdminGateDeps } from "./guards";

function makeDeps(overrides: Partial<AdminGateDeps> = {}): AdminGateDeps & {
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
  } as AdminGateDeps & {
    fetchRole: ReturnType<typeof vi.fn>;
    clearSession: ReturnType<typeof vi.fn>;
  };
}

describe("resolveAdminAccess", () => {
  it("lets public routes through without asking the server", async () => {
    const deps = makeDeps();

    await expect(resolveAdminAccess(false, "/", deps)).resolves.toBeNull();
    // 关键：公开路由不该因为守卫而多一次 /me 请求
    expect(deps.fetchRole).not.toHaveBeenCalled();
  });

  it("lets an admin into the admin route", async () => {
    const deps = makeDeps();

    await expect(
      resolveAdminAccess(true, "/admin/buildings", deps),
    ).resolves.toBeNull();
    expect(deps.fetchRole).toHaveBeenCalledWith("token-abc");
  });

  it("sends an anonymous visitor to login and remembers the target", async () => {
    const deps = makeDeps({ readToken: () => null });

    await expect(
      resolveAdminAccess(true, "/admin/buildings", deps),
    ).resolves.toEqual({
      name: "login",
      query: { redirect: "/admin/buildings" },
    });
    expect(deps.fetchRole).not.toHaveBeenCalled();
  });

  it("keeps a member out of the admin route", async () => {
    const deps = makeDeps({ fetchRole: vi.fn(async () => "member") });

    await expect(
      resolveAdminAccess(true, "/admin/buildings", deps),
    ).resolves.toEqual({ name: "dashboard" });
    // 角色是 member 说明令牌本身有效，不该清会话
    expect(deps.clearSession).not.toHaveBeenCalled();
  });

  it("clears the session when the server rejects the token", async () => {
    const deps = makeDeps({
      fetchRole: vi.fn(async () => {
        throw new AuthRequestError("token 已过期", 401, 4002);
      }),
    });

    await expect(
      resolveAdminAccess(true, "/admin/buildings", deps),
    ).resolves.toEqual({
      name: "login",
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
      resolveAdminAccess(true, "/admin/buildings", deps),
    ).resolves.toEqual({
      name: "login",
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

    await resolveAdminAccess(true, "/admin/buildings", deps);

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
      resolveAdminAccess(true, "/admin/buildings", deps),
    ).resolves.not.toBeNull();
  });
});
