import { describe, expect, it, vi } from "vitest";
vi.mock("@/modules/recruitment/views/InvitationsView.vue", () => ({
  default: { name: "InvitationsView" },
}));
vi.mock("@/modules/recruitment/views/CandidatesView.vue", () => ({
  default: { name: "CandidatesView" },
}));
vi.mock("@/modules/recruitment/views/CandidateDetailView.vue", () => ({
  default: { name: "CandidateDetailView" },
}));
vi.mock("@/modules/recruitment/views/MyInvitationsView.vue", () => ({
  default: { name: "MyInvitationsView" },
}));

vi.mock("@/modules/dashboard/views/DashboardView.vue", () => ({
  default: { name: "DashboardView" },
}));
vi.mock("@/modules/dashboard/views/AdminMembersView.vue", () => ({
  default: { name: "AdminMembersView" },
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
vi.mock("@/modules/recruitment/views/ProspectsView.vue", () => ({
  default: { name: "ProspectsView" },
}));
vi.mock("@/modules/recruitment/views/ApplicationsView.vue", () => ({
  default: { name: "ApplicationsView" },
}));
vi.mock("@/modules/recruitment/views/ProspectDetailView.vue", () => ({
  default: { name: "ProspectDetailView" },
}));
// mail 视图引入 Element Plus 弹窗与 composable，测试里只验路由元信息
vi.mock("@/modules/mail/views/MailCenterView.vue", () => ({
  default: { name: "MailCenterView" },
}));
// 个人设置视图在模块作用域 import Element Plus 组件，测试里只验路由元信息
vi.mock("@/modules/user-settings/views/UserSettingsView.vue", () => ({
  default: { name: "UserSettingsView" },
}));

import { routes } from "./routes";

describe("application routes", () => {
  it("mounts the same 2D auth view on both public login paths", () => {
    const rootRoutes = routes.filter((route) => route.path === "/");

    expect(rootRoutes).toHaveLength(1);
    expect(rootRoutes[0]?.name).toBe("opener");
    expect(rootRoutes[0]?.component).toMatchObject({ name: "AuthView" });
    expect(rootRoutes[0]?.component).toBe(
      routes.find((route) => route.path === "/login")?.component,
    );
    // 开屏对所有人开放，不能挂上任何角色限制
    expect(rootRoutes[0]?.meta?.allowRoles).toBeUndefined();
  });

  it("marks the buildings editor as member-only", () => {
    const adminRoutes = routes.filter(
      (route) => route.path === "/admin/buildings",
    );

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.component).toMatchObject({
      name: "BuildingsEditor",
    });
    // 丢了 allowRoles 就等于门禁静默失效（guard 只认这个 meta），必须锁住
    expect(adminRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
  });

  it("mounts the admin dashboard separately from placeholder pages", () => {
    const dashboardRoutes = routes.filter(
      (route) => route.path === "/dashboard",
    );

    expect(dashboardRoutes).toHaveLength(1);
    expect(dashboardRoutes[0]?.component).toMatchObject({
      name: "DashboardView",
    });
    // 管理台含后端健康状态与用户名单，必须是成员专属
    expect(dashboardRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
  });

  it("mounts the admin members page in the settings center (issue #59)", () => {
    const adminRoutes = routes.filter(
      (route) => route.path === "/settings/users",
    );

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.name).toBe("settings-users");
    expect(adminRoutes[0]?.component).toMatchObject({
      name: "AdminMembersView",
    });
    // 用户与权限页能改角色，漏标 allowRoles 就等于对普通用户敞开
    expect(adminRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
    // 设置中心子页同样挂统一管理外壳
    expect(adminRoutes[0]?.meta?.adminShell).toBe(true);
  });

  it("redirects the settings index and the retired admins path (issue #59)", () => {
    // /settings 索引落到第一个真实区块，避免空落点
    const settingsIndex = routes.filter((route) => route.path === "/settings");
    expect(settingsIndex).toHaveLength(1);
    expect(settingsIndex[0]?.redirect).toBe("/settings/users");

    // 旧路径 /dashboard/admins 保留重定向，收藏夹与旧链接不能变死链
    const retiredAdmins = routes.filter(
      (route) => route.path === "/dashboard/admins",
    );
    expect(retiredAdmins).toHaveLength(1);
    expect(retiredAdmins[0]?.redirect).toBe("/settings/users");
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
      { path: "/user/invitations", name: "user-invitations" },
    ]);
    // 用户侧页面必须挂守卫：未登录不能看，成员则照常放行（单向规则）
    for (const page of userPages) {
      expect(page.meta?.allowRoles).toEqual(["admin", "member"]);
    }
  });

  it("mounts the mail center as a member-only real page", () => {
    const mailRoutes = routes.filter((route) => route.path === "/templates");

    expect(mailRoutes).toHaveLength(1);
    expect(mailRoutes[0]?.name).toBe("templates");
    expect(mailRoutes[0]?.component).toMatchObject({ name: "MailCenterView" });
    // 邮件含招新对象联系方式，漏标 allowRoles 就等于对普通用户敞开
    expect(mailRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
    // 邮件中心也属于统一管理外壳，避免重新引入独立顶栏。
    expect(mailRoutes[0]?.meta?.adminShell).toBe(true);
  });

  it("leaves no placeholder pages behind (issue #59 retired the placeholder system)", () => {
    // /dashboard 与 /settings 都有了真实页面（或重定向），
    // PlaceholderView 已删除；任何回归成占位页的路由都会在这里暴露。
    const placeholderRoutes = routes.filter(
      (route) =>
        typeof route.component === "object" &&
        route.component !== null &&
        "name" in route.component &&
        (route.component as { name?: string }).name === "PlaceholderView",
    );
    expect(placeholderRoutes).toEqual([]);
  });

  it("keeps every real admin page inside the member-only shell", () => {
    // 管理页统一带 adminShell + ["admin"]；漏标任一项都会让
    // 页面脱出管理外壳或对普通用户敞开（守卫只认 allowRoles）。
    // /admin/buildings 是例外：全屏布局编辑器，刻意不挂管理外壳
    // （它有自己的成员专属断言用例），只锁 allowRoles。
    const adminPages = routes.filter(
      (route) =>
        route.component !== undefined &&
        typeof route.path === "string" &&
        (route.path.startsWith("/dashboard") ||
          route.path.startsWith("/settings") ||
          route.path.startsWith("/recruitment") ||
          route.path.startsWith("/forms") ||
          route.path === "/templates" ||
          route.path === "/admin/buildings"),
    );
    expect(adminPages.length).toBeGreaterThan(0);
    for (const page of adminPages) {
      expect(page.meta?.allowRoles).toEqual(["admin"]);
      if (page.path !== "/admin/buildings") {
        expect(page.meta?.adminShell).toBe(true);
      }
    }
  });

  it("mounts real forms pages with admin permissions and authenticated share access", () => {
    for (const path of ["/forms", "/forms/new", "/forms/:id/edit", "/forms/:id/submissions"]) {
      const matches = routes.filter(route => route.path === path);
      expect(matches).toHaveLength(1);
      expect(matches[0]?.meta?.allowRoles).toEqual(["admin"]);
      expect(matches[0]?.meta?.adminShell).toBe(true);
      expect(matches[0]?.component).not.toMatchObject({ name: "PlaceholderView" });
    }
    const share = routes.find(route => route.path === "/f/:shareToken");
    expect(share?.meta?.allowRoles).toEqual(["admin", "member"]);
    expect(share?.meta?.signalShell).toBe(true);
  });

  it("registers the recruitment flow routes", () => {
    const recruitmentPaths = routes
      .filter((route) => route.path.startsWith("/recruitment"))
      .map((route) => route.path);

    expect(recruitmentPaths).toEqual([
      "/recruitment",
      "/recruitment/prospects",
      "/recruitment/prospects/:id",
      "/recruitment/applications",
      "/recruitment/invitations",
      "/recruitment/candidates",
      "/recruitment/candidates/:id",
    ]);
  });

  it("locks the recruitment pages behind the member role", () => {
    const recruitmentPages = routes.filter(
      (route) => route.path.startsWith("/recruitment") && route.component,
    );

    expect(recruitmentPages).toHaveLength(4);
    for (const page of recruitmentPages) {
      // 漏标 allowRoles 的页面会被守卫当成公开路由，这里锁死
      expect(page.meta?.allowRoles).toEqual(["admin"]);
    }
  });
});
