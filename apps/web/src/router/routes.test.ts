import { describe, expect, it, vi } from "vitest";

vi.mock("@/views/Home.vue", () => ({ default: { name: "HomeView" } }));
vi.mock("@/views/PlaceholderView.vue", () => ({
  default: { name: "PlaceholderView" },
}));

import { routes } from "./routes";

describe("application routes", () => {
  it("registers each planned placeholder page on the actual route list", () => {
    const placeholderRoutes = routes.filter((route) => route.path !== "/");

    expect(placeholderRoutes.map(({ path, name, meta }) => ({
      path,
      name,
      title: meta?.title,
    }))).toEqual([
      { path: "/login", name: "login", title: "登录" },
      { path: "/dashboard", name: "dashboard", title: "控制台" },
      { path: "/forms", name: "forms", title: "表单管理" },
      { path: "/recruitment", name: "recruitment", title: "招聘管理" },
      { path: "/templates", name: "templates", title: "邮件模板" },
      { path: "/settings", name: "settings", title: "系统设置" },
    ]);
    expect(new Set(placeholderRoutes.map(({ component }) => component)).size).toBe(1);
  });
});
