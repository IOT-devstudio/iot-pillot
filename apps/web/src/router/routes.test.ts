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
vi.mock("@/modules/recruitment/views/ProspectsView.vue", () => ({
  default: { name: "ProspectsView" },
}));
vi.mock("@/modules/recruitment/views/ProspectDetailView.vue", () => ({
  default: { name: "ProspectDetailView" },
}));

import { routes } from "./routes";

describe("application routes", () => {
  it("mounts the 3D opener on its own path", () => {
    const openerPaths = routes.filter((route) => route.path === "/opener");

    expect(openerPaths).toHaveLength(1);
    expect(openerPaths[0]?.name).toBe("opener");
    expect(openerPaths[0]?.component).toMatchObject({ name: "StudioOpener" });
  });

  it("marks the buildings editor as admin-only", () => {
    const adminRoutes = routes.filter((route) => route.path === "/admin/buildings");

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.component).toMatchObject({ name: "BuildingsEditor" });
    // 丢了 requiresAdmin 就等于门禁静默失效（guard 只认这个 meta），必须锁住
    expect(adminRoutes[0]?.meta?.requiresAdmin).toBe(true);
  });

  it("keeps the fallback login page and mounts the health page on the root path", () => {
    const loginRoutes = routes.filter((route) => route.path === "/login");
    expect(loginRoutes).toHaveLength(1);
    expect(loginRoutes[0]?.component).toMatchObject({ name: "AuthView" });

    // 根路径回到内容首页，健康检查入口也随之回到 /
    const homeRoutes = routes.filter((route) => route.path === "/");
    expect(homeRoutes).toHaveLength(1);
    expect(homeRoutes[0]?.component).toMatchObject({ name: "HomeView" });
  });

  it("registers each planned placeholder page on the actual route list", () => {
    const reservedPaths = [
      "/",
      "/opener",
      "/login",
      "/admin/buildings",
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
      { path: "/dashboard", name: "dashboard", title: "控制台" },
      { path: "/forms", name: "forms", title: "表单管理" },
      { path: "/templates", name: "templates", title: "邮件模板" },
      { path: "/settings", name: "settings", title: "系统设置" },
    ]);
    expect(new Set(placeholderRoutes.map(({ component }) => component)).size).toBe(1);
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
});
