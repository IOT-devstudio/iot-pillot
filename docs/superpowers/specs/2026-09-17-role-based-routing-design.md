# 基于角色的路由守护与登录分流设计

## 目标

让登录后的落点与访问范围由用户身份决定：工作室成员进入管理后台，普通用户进入个人页面。两个目标页面本期使用占位实现，本设计只交付**身份判定、路由守护与分流导向**这条链路。

## 角色模型

复用后端已有的两级模型，不新增角色，不改动 Go 代码。

| 后端角色 | 业务含义 | 来源 |
| --- | --- | --- |
| `admin` | 工作室成员 | 环境变量 `IOT_PILOT_AUTH_ADMIN_USERS` 用户名白名单 |
| `member` | 普通用户 | 不在白名单中的注册用户 |

角色不落库，由 `apps/api/internal/config/config.go` 的 `AdminUsers` 白名单在签发令牌时决定。这意味着**普通用户自行注册即可获得 `member` 角色**，成员身份需要由运维往白名单中添加用户名。

角色取值必须**每次向服务端查询**（`GET /api/v1/me`），不使用本地缓存：白名单变更后本地旧值会过期。这一点沿用 `apps/web/src/api/auth.ts` 中 `fetchCurrentUser` 的现有约定。

## 访问矩阵

| 路由 | 归属 | `admin` | `member` | 未登录 |
| --- | --- | --- | --- | --- |
| `/`、`/login`、`/home` | 公开 | 放行 | 放行 | 放行 |
| `/dashboard`、`/forms`、`/recruitment`、`/templates`、`/settings` | 成员页 | 放行 | 跳 `/member` | 跳 `/login` |
| `/admin/buildings` | 成员页 | 放行 | 跳 `/member` | 跳 `/login` |
| `/member` | 个人页 | 放行 | 放行 | 跳 `/login` |

访问规则是**单向**的：成员可以访问普通用户页面，反向不行。因此 `/member` 的允许角色是两个角色，而不是仅 `member`。

## 守卫改造

`apps/web/src/router/guards.ts` 的守卫从「单布尔开关」升级为「角色清单」。

### 接口变化

```ts
// 改造前
resolveAdminAccess(requiresAdmin: boolean, fullPath: string, deps)
  : Promise<AdminRedirect | null>

// 改造后
resolveRouteAccess(allowRoles: readonly UserRole[], fullPath: string, deps)
  : Promise<RouteRedirect | null>
```

`allowRoles` 为空数组表示公开路由，直接放行且**不向服务端发请求**——这保留了现有实现中「公开路由零额外请求」的优化。

### 判定流程

```text
allowRoles 为空              -> 放行
无本地 token                 -> /login?redirect=<原路径>
查询 /me 得 role
  ├─ role 在 allowRoles 内   -> 放行
  └─ role 不在 allowRoles 内 -> 跳该 role 自己的落点
查询 /me 抛 401 或 403       -> 清会话 -> /login?redirect=<原路径>
查询 /me 抛其他错误（网络等） -> 不清会话 -> /login?redirect=<原路径>
```

越权时跳「该角色自己的落点」而非固定的拒绝页：普通用户访问成员页会落到 `/member`，成员访问 `/member` 本就放行。这样不会出现「被跳到自己也进不去的页面」的死循环。

网络异常不清会话，沿用现有实现：网络抖动不应该删掉用户的会话。

### 路由元信息

路由通过 `meta.allowRoles` 声明允许的角色清单：

```ts
meta: { allowRoles: ["admin"] }             // 成员页
meta: { allowRoles: ["admin", "member"] }   // 个人页（成员也可访问）
```

公开路由不声明 `meta.allowRoles`。这取代了现有的 `meta.requiresAdmin: true`。

## 登录分流

当前 `apps/web/src/auth/submit.ts` 在登录成功后硬编码跳转 `/dashboard`，不区分身份。改造后登录流程增加一步角色查询：

```text
登录成功 -> 保存会话 -> 查询 /me 得 role -> 决定落点 -> 跳转
```

落点判定抽成纯函数，供守卫与登录流程共用：

```ts
landingFor(role)  // admin -> /dashboard，其余 -> /member
```

### redirect 参数

守卫在跳转登录页时会带上 `?redirect=<原路径>`，但当前 `AuthView.vue` 从不读取该参数，登录后一律跳 `/dashboard`。本次一并修复，使该机制真正生效。

**实现取舍**：最初设计让登录流程自己查询路由表、用 `canAccess` 校验 redirect 目标的权限后再决定落点。实现时改成了更简单的做法——**登录后只做 `redirect ?? "/dashboard"`，权限交给守卫把关**：

- 普通用户带着 `redirect=/admin/buildings` 登录会先跳到该路径，守卫随即判定无权，把它送到 `/member`。
- 行为与「登录时先校验」完全一致，但少了一整套「路径 → 角色清单」的查表管线。
- 而且**更省请求**：普通用户无 redirect 时，前者需要 2 次 `/me`，后者只需守卫那 1 次。
- Vue Router 的异步守卫在目标组件渲染前完成，用户看不到中间跳转。

因此 `guards.ts` 最终只导出 `resolveRouteAccess` 与 `landingFor`，没有 `canAccess`。

redirect 取值由 `submitAuth` 校验，**只接受站内绝对路径**（以 `/` 开头且不以 `//` 开头）。放行 `//evil.com` 这类协议相对地址会让登录变成一个开放重定向漏洞。

举例：

- 成员会话过期被踢到登录页，重新登录后回到原目标页。
- 普通用户直接访问 `/admin/buildings` 被踢到登录页，登录后经守卫落到 `/member`，不会卡在无权页面。
- 正常登录各回各家。

## 文件改动

全部集中在前端，后端零改动。

| 文件 | 动作 |
| --- | --- |
| `apps/web/src/router/guards.ts` | 改造为 `resolveRouteAccess`，新增 `landingFor` |
| `apps/web/src/router/guards.test.ts` | 适配现有用例，新增越权跳转与角色清单用例 |
| `apps/web/src/router/index.ts` | 接线新守卫，从 `meta.allowRoles` 取值 |
| `apps/web/src/router/routes.ts` | 五个占位页声明 `meta.allowRoles`，汇总 `/member` |
| `apps/web/src/router/routes.test.ts` | 新增角色守卫断言，锁定每页的 `allowRoles` |
| `apps/web/src/router/route-specs.ts` | 移除 `/login`（已单独声明），导出 `MEMBER_ONLY` |
| `apps/web/src/modules/opener-editor/routes.ts` | `requiresAdmin: true` 改为 `allowRoles: ["admin"]` |
| `apps/web/src/modules/shared/routes.ts` | 新增，声明 `/member` 与 `ANY_SIGNED_IN` |
| `apps/web/src/modules/shared/views/MemberView.vue` | 新增普通用户占位页 |
| `apps/web/src/auth/submit.ts` | 消费 `redirect`，新增 `defaultNavigateTarget` 与其校验 |
| `apps/web/src/auth/submit.test.ts` | 新增 redirect 落点与开放重定向防护用例 |
| `apps/web/src/views/AuthView.vue` | 读取并传入 `redirect` 参数 |
| `apps/web/src/views/AuthView.test.ts` | `vue-router` 桩补充 `useRoute`，新增 redirect 用例 |

### 顺带修复：`BuildingInspector.vue` 的 TDZ 崩溃

`apps/web/src/modules/opener-editor/components/BuildingInspector.vue` 中，一个带 `immediate: true` 的 `watch` 在 setup 期同步执行，而它引用的 `currentVertex` computed 用 `const` 声明在该 watch **之后**，导致 `ReferenceError: Cannot access 'currentVertex' before initialization`。

这个 bug 原先不可见，因为 `/admin/buildings` 从未真正被路由进入过（守卫与路由配置在此之前把所有人都挡在外面）。本次改动让成员首次能正常进入该页，才把它暴露出来。修复方式是把 `currentVertex` 的声明提到引用它的 watch 之前，语义不变。

注意：该页的 canvas 在**改动前就**不渲染（已用 `git stash` 对照原始代码验证），属于既有问题，不在本次范围。

## 测试

守卫是纯函数加依赖注入，沿用现有测试模式：不启动 router、不发真实网络请求，通过注入桩函数验证分支。

需要覆盖的分支：

- 公开路由放行，且不查询服务端（保持现有用例）。
- 成员访问成员页放行。
- 普通用户访问成员页，跳转到 `/member`。
- 成员访问 `/member` 放行（单向规则的直接验证）。
- 未登录访问成员页，跳登录页并带上 `redirect`。
- 令牌失效（401/403）清会话，其他错误不清会话。
- 未知角色按最小可见范围处理，不卡在登录页。
- `landingFor` 对 `admin`、`member` 与未知角色分别返回正确落点。
- 登录流程：无 redirect 时落默认页；有 redirect 时跳该路径；开放重定向（`//evil.com`、`https://...`、相对路径）一律拒绝。

## 验证结果

- `pnpm typecheck` 通过。
- `pnpm vitest run` 通过：11 个测试文件、113 个用例。
- `pnpm build` 通过（chunk 体积警告为既有问题，由 three.js 导致）。
- 浏览器实测（后端未启动，用注入的 `/api/v1/me` 桩模拟两种身份）：
  - 未登录访问 `/member` → 跳 `/login?redirect=/member`。
  - 普通用户带 `redirect=/admin/buildings` 登录 → 落到 `/member`，页面渲染「个人中心」。
  - 成员带 `redirect=/admin/buildings` 登录 → 落到 `/admin/buildings`，编辑器正常加载。

## 不在本次范围

- 成员后台与普通用户页面的真实内容（本期为占位页）。
- 后台主布局、侧边导航与面包屑（对应 CLAUDE.md 的 M3）。
- 后端角色模型变更、`/api/v1/me` 接口调整、数据库 schema 变更。
- 401 刷新队列与令牌自动续期（对应 CLAUDE.md 的 M1）。
- 404 与 403 专属错误页。
