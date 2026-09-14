import { describe, expect, it } from "vitest";
import { placeholderRouteSpecs } from "./route-specs";

describe("placeholder route specifications", () => {
  it("registers each planned placeholder page with its title", () => {
    expect(placeholderRouteSpecs).toEqual([
      { path: "/login", name: "login", title: "登录" },
      { path: "/dashboard", name: "dashboard", title: "控制台" },
      { path: "/forms", name: "forms", title: "表单管理" },
      { path: "/recruitment", name: "recruitment", title: "招聘管理" },
      { path: "/templates", name: "templates", title: "邮件模板" },
      { path: "/settings", name: "settings", title: "系统设置" },
    ]);
  });
});
