/**
 * 后台管理页的占位路由清单。
 *
 * 这些页面都属于「工作室成员」功能区，只允许 admin 进入；
 * 普通用户（member）会被守卫带到 /member（见 router/guards.ts）。
 */
export const placeholderRouteSpecs = [
  { path: "/dashboard", name: "dashboard", title: "控制台" },
  { path: "/forms", name: "forms", title: "表单管理" },
  { path: "/recruitment", name: "recruitment", title: "招聘管理" },
  { path: "/templates", name: "templates", title: "邮件模板" },
  { path: "/settings", name: "settings", title: "系统设置" },
] as const;

/** 成员页允许的角色。抽成常量避免各处重复字面量写错。 */
export const MEMBER_ONLY = ["admin"] as const;
