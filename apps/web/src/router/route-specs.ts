/**
 * 路由角色清单的集中定义。
 *
 * 抽成常量避免各处重复字面量写错；守卫只认 meta.allowRoles，
 * 任何一页漏标都会让门禁静默失效（routes.test.ts 锁住了这一点）。
 */

/** 只允许工作室成员进入。后台管理页用这个。 */
export const MEMBER_ONLY = ["admin"] as const;

/**
 * 登录用户都能进。用户侧页面用这个。
 *
 * 同时列出两个角色是因为访问规则是**单向**的：工作室成员也能访问
 * 普通用户页面，反向不行。
 */
export const ANY_SIGNED_IN = ["admin", "member"] as const;

/**
 * 后台管理页的占位路由清单。
 *
 * 这些页面都属于「工作室成员」功能区；普通用户会被守卫带到自己的落点
 * （见 router/guards.ts 的 landingFor）。
 */
export const placeholderRouteSpecs = [
  { path: "/dashboard", name: "dashboard", title: "控制台" },
  { path: "/forms", name: "forms", title: "表单管理" },
  { path: "/templates", name: "templates", title: "邮件模板" },
  { path: "/settings", name: "settings", title: "系统设置" },
] as const;
