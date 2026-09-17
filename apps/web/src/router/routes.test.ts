import { describe, expect, it, vi } from "vitest";

vi.mock("@/views/Home.vue", () => ({ default: { name: "HomeView" } }));
vi.mock("@/views/PlaceholderView.vue", () => ({
  default: { name: "PlaceholderView" },
}));
vi.mock("@/views/AuthView.vue", () => ({ default: { name: "AuthView" } }));
// opener 视图会在模块作用域引入 three / gsap 并操作 WebGL，测试里必须打桩
vi.mock("@/modules/opener/views/StudioOpener.vue", () => ({
  default: { name: "StudioOpener" },
}));
vi.mock("@/modules/opener-editor/views/BuildingsEditor.vue", () => ({
  default: { name: "BuildingsEditor" },
}));
vi.mock("@/modules/shared/views/MemberView.vue", () => ({
  default: { name: "MemberView" },
}));

import { routes } from "./routes";

describe("application routes", () => {
  it("mounts the 3D opener on the root path", () => {
    const rootRoutes = routes.filter((route) => route.path === "/");

    expect(rootRoutes).toHaveLength(1);
    expect(rootRoutes[0]?.name).toBe("opener");
    expect(rootRoutes[0]?.component).toMatchObject({ name: "StudioOpener" });
    // 开屏对所有人开放，不能挂上任何角色限制
    expect(rootRoutes[0]?.meta?.allowRoles).toBeUndefined();
  });

  it("marks the buildings editor as member-only", () => {
    const adminRoutes = routes.filter((route) => route.path === "/admin/buildings");

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.component).toMatchObject({ name: "BuildingsEditor" });
    // 丢了 allowRoles 就等于门禁静默失效（guard 只认这个 meta），必须锁住
    expect(adminRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
  });

  it("keeps the fallback login page and the health page on their own paths", () => {
    const loginRoutes = routes.filter((route) => route.path === "/login");
    expect(loginRoutes).toHaveLength(1);
    expect(loginRoutes[0]?.component).toMatchObject({ name: "AuthView" });
    // 登录页必须公开，否则未登录用户会被守卫挡住，形成死循环
    expect(loginRoutes[0]?.meta?.allowRoles).toBeUndefined();

    // 根路径让给开屏后，健康检查页必须还在，否则会丢掉这个入口
    const homeRoutes = routes.filter((route) => route.path === "/home");
    expect(homeRoutes).toHaveLength(1);
    expect(homeRoutes[0]?.component).toMatchObject({ name: "HomeView" });
  });

  it("registers the personal page for both roles", () => {
    const memberRoutes = routes.filter((route) => route.path === "/member");

    expect(memberRoutes).toHaveLength(1);
    expect(memberRoutes[0]?.component).toMatchObject({ name: "MemberView" });
    // 访问规则是单向的：成员也能进普通用户页，所以两个角色都要列上
    expect(memberRoutes[0]?.meta?.allowRoles).toEqual(["admin", "member"]);
  });

  it("registers each planned placeholder page as member-only", () => {
    const reservedPaths = [
      "/",
      "/home",
      "/login",
      "/admin/buildings",
      "/member",
    ];
    const placeholderRoutes = routes.filter(
      (route) => !reservedPaths.includes(route.path),
    );

    expect(placeholderRoutes.map(({ path, name, meta }) => ({
      path,
      name,
      title: meta?.title,
    }))).toEqual([
      { path: "/dashboard", name: "dashboard", title: "控制台" },
      { path: "/forms", name: "forms", title: "表单管理" },
      { path: "/recruitment", name: "recruitment", title: "招聘管理" },
      { path: "/templates", name: "templates", title: "邮件模板" },
      { path: "/settings", name: "settings", title: "系统设置" },
    ]);
    expect(new Set(placeholderRoutes.map(({ component }) => component)).size).toBe(1);
    // 后台页全部只对成员开放，任何一页漏标都会让普通用户直接走进去
    expect(
      placeholderRoutes.every(
        ({ meta }) => JSON.stringify(meta?.allowRoles) === '["admin"]',
      ),
    ).toBe(true);
  });
});
