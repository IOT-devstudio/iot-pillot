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

// 占位路由清单（placeholderRouteSpecs）已随 issue #59 退役：
// /dashboard 与 /settings 都有了真实页面，PlaceholderView 不再被任何路由引用。
// 将来的占位需求（设置中心新区块等）按票内教训**不做占位导航**，
// 等内容 issue 就绪再加真实路由。
