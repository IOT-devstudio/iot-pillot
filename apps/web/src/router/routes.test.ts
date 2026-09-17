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

import { routes } from "./routes";

describe("application routes", () => {
  it("mounts the 3D opener on the root path", () => {
    const rootRoutes = routes.filter((route) => route.path === "/");

    expect(rootRoutes).toHaveLength(1);
    expect(rootRoutes[0]?.name).toBe("opener");
    expect(rootRoutes[0]?.component).toMatchObject({ name: "StudioOpener" });
  });

  it("marks the buildings editor as admin-only", () => {
    const adminRoutes = routes.filter((route) => route.path === "/admin/buildings");

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.component).toMatchObject({ name: "BuildingsEditor" });
    // 丢了 requiresAdmin 就等于门禁静默失效（guard 只认这个 meta），必须锁住
    expect(adminRoutes[0]?.meta?.requiresAdmin).toBe(true);
  });

  it("keeps the fallback login page and the health page on their own paths", () => {
    const loginRoutes = routes.filter((route) => route.path === "/login");
    expect(loginRoutes).toHaveLength(1);
    expect(loginRoutes[0]?.component).toMatchObject({ name: "AuthView" });

    // 根路径让给开屏后，健康检查页必须还在，否则会丢掉这个入口
    const homeRoutes = routes.filter((route) => route.path === "/home");
    expect(homeRoutes).toHaveLength(1);
    expect(homeRoutes[0]?.component).toMatchObject({ name: "HomeView" });
  });

  it("registers each planned placeholder page on the actual route list", () => {
    const reservedPaths = [
      "/",
      "/home",
      "/login",
      "/admin/buildings",
      "/user/home",
      "/user/about",
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
  });
});
