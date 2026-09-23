import { describe, expect, it, vi } from "vitest";

vi.mock("@/modules/dashboard/views/DashboardView.vue", () => ({
  default: { name: "DashboardView" },
}));
vi.mock("@/modules/dashboard/views/AdminMembersView.vue", () => ({
  default: { name: "AdminMembersView" },
}));
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
vi.mock("@/modules/home/views/UserHomeView.vue", () => ({
  default: { name: "UserHomeView" },
}));
vi.mock("@/modules/about/views/AboutView.vue", () => ({
  default: { name: "AboutView" },
}));
vi.mock("@/modules/user-settings/views/UserSettingsView.vue", () => ({
  default: { name: "UserSettingsView" },
}));
vi.mock("@/modules/recruitment/views/ProspectsView.vue", () => ({
  default: { name: "ProspectsView" },
}));
vi.mock("@/modules/recruitment/views/ProspectDetailView.vue", () => ({
  default: { name: "ProspectDetailView" },
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

  it("mounts the admin dashboard separately from placeholder pages", () => {
    const dashboardRoutes = routes.filter((route) => route.path === "/dashboard");

    expect(dashboardRoutes).toHaveLength(1);
    expect(dashboardRoutes[0]?.component).toMatchObject({
      name: "DashboardView",
    });
    // 管理台含后端健康状态与用户名单，必须是成员专属
    expect(dashboardRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
  });

  it("mounts the admin members page as a member-only dashboard child", () => {
    const adminRoutes = routes.filter(
      (route) => route.path === "/dashboard/admins",
    );

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.name).toBe("dashboard-admins");
    expect(adminRoutes[0]?.component).toMatchObject({
      name: "AdminMembersView",
    });
    // 用户与权限页能改角色，漏标 allowRoles 就等于对普通用户敞开
    expect(adminRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
  });

  it("keeps the fallback login page public and retires the old home path", () => {
    const loginRoutes = routes.filter((route) => route.path === "/login");
    expect(loginRoutes).toHaveLength(1);
    expect(loginRoutes[0]?.component).toMatchObject({ name: "AuthView" });
    // 后备登录页必须公开：3D 渲染失败转投过来时，守卫不能把它再挡走
    expect(loginRoutes[0]?.meta?.allowRoles).toBeUndefined();

    // 内容首页已并入管理台，旧路径必须重定向过去，否则收藏夹和旧链接会变成死链
    const retiredHome = routes.filter((route) => route.path === "/home");
    expect(retiredHome).toHaveLength(1);
    expect(retiredHome[0]?.redirect).toBe("/dashboard");
  });

  it("guards the user-side pages for signed-in users", () => {
    const userPages = routes.filter((route) => route.path.startsWith("/user/"));

    expect(userPages.map(({ path, name }) => ({ path, name }))).toEqual([
      { path: "/user/home", name: "user-home" },
      { path: "/user/about", name: "about" },
      { path: "/user/settings", name: "user-settings" },
    ]);
    // 用户侧页面必须挂守卫：未登录不能看，成员则照常放行（单向规则）
    for (const page of userPages) {
      expect(page.meta?.allowRoles).toEqual(["admin", "member"]);
    }
  });

  it("registers each planned placeholder page as member-only", () => {
    const reservedPaths = [
      "/",
      "/home",
      "/login",
      "/admin/buildings",
      "/dashboard",
      "/dashboard/admins",
      "/user/home",
      "/user/about",
      "/user/settings",
      "/recruitment",
      "/recruitment/prospects",
      "/recruitment/prospects/:id",
    ];
    const placeholderRoutes = routes.filter(
      (route) => !reservedPaths.includes(route.path),
    );

    expect(placeholderRoutes.map(({ path, name, meta }) => ({
      path,
      name,
      title: meta?.title,
    }))).toEqual([
      { path: "/forms", name: "forms", title: "表单管理" },
      { path: "/templates", name: "templates", title: "邮件模板" },
      { path: "/settings", name: "settings", title: "系统设置" },
    ]);
    expect(new Set(placeholderRoutes.map(({ component }) => component)).size).toBe(1);
    // 后台占位页全部只对成员开放，任何一页漏标都会让普通用户直接走进去
    expect(
      placeholderRoutes.every(
        ({ meta }) => JSON.stringify(meta?.allowRoles) === '["admin"]',
      ),
    ).toBe(true);
  });

  it("registers the recruitment flow routes", () => {
    const recruitmentPaths = routes
      .filter((route) => route.path.startsWith("/recruitment"))
      .map((route) => route.path);

    expect(recruitmentPaths).toEqual([
      "/recruitment",
      "/recruitment/prospects",
      "/recruitment/prospects/:id",
    ]);
  });

  it("locks the recruitment pages behind the member role", () => {
    const recruitmentPages = routes.filter(
      (route) => route.path.startsWith("/recruitment") && route.component,
    );

    expect(recruitmentPages).toHaveLength(2);
    for (const page of recruitmentPages) {
      // 漏标 allowRoles 的页面会被守卫当成公开路由，这里锁死
      expect(page.meta?.allowRoles).toEqual(["admin"]);
    }
  });
});
