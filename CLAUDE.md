# iot-pillot

## 项目身份

- **名称**: iot-pillot
- **组织**: [IOT-devstudio](https://github.com/IOT-devstudio)
- **定位**: IoT 全栈开发工作室的后台管理 web 程序
- **当前阶段**: 工作室招新管理（其他功能后续迭代）

---

## 当前功能范围（招新管理）

1. **表单系统**
   - 支持创建表单、收集新生的报名意向、获取基本信息（姓名/邮箱等）
   - 支持匿名填写
   - 支持通过分享链接填写
2. **SMTP 邀请流程**：获取邮箱后 → SMTP 发送邀请函 → 接受后角色推进为候选人
3. **候选人处理**
   - 通过面试 → 一键发送 offer
   - 未通过面试 → 一键发送感谢信
4. **邮件模板管理**：统一配置，支持扩展新增，支持删除
5. **权限管理**：管理员 / 成员 两级 RBAC
6. **SMTP 可替换**：发件配置（服务商/账号/凭据）可整体替换，不影响业务代码

---

## 技术栈

### 已确认且已落地（已在 `go.mod` 或 `package.json` 中实际安装）

| 层            | 选型                                        | 说明                                                             |
| ------------- | ------------------------------------------- | ---------------------------------------------------------------- |
| 前端框架      | **Vue 3 + Element Plus + TypeScript** | 会 Vue 的开发者更多，接手门槛低                                   |
| 前端构建      | **Vite** + `unplugin-auto-import` + `unplugin-vue-components` | Element Plus 按需自动引入；`/api` 代理到 :8080                 |
| 后端语言      | **Go**                                | 单二进制部署                                                     |
| 后端框架      | **Gin**                                | 路由 + 中间件；已装在 `apps/api/go.mod`                          |
| 配置管理      | **viper**                              | env prefix `IOT_PILOT_`，自动 ENV 覆盖；已装                     |
| 数据库        | **PostgreSQL**                        | `docker-compose.yml` 启动本地实例；Redis 同文件，两者都是启动期硬依赖 |
| ORM           | **GORM**                              | `apps/api/go.mod` 与 `internal/repository` 已实际使用               |
| SMTP 库       | **gomail**                            | `apps/api/go.mod` 与 `internal/utils/mail_util.go` 已实际使用       |
| Monorepo 工具 | **pnpm workspaces**                   | 前后端统一管理                                                   |
| CI/CD         | **GitHub Actions**                    | `auto-merge.yml`（bot）+ `ci.yml`（PR 验证）+ `deploy.yml`（push main → VPS） |

### 已规划但未引入（决策已定，依赖业务模块开工时安装）

| 组件       | 建议                                | 备选             | 引入时机                  |
| ---------- | ----------------------------------- | ---------------- | ------------------------- |
| 模板渲染   | `html/template` + Handlebars 子集 | —               | TemplateModule 开工       |
| RBAC       | casbin                              | JWT + 中间件手写 | 角色模型与路由守卫落地时引入 |
| 数据库迁移 | golang-migrate                      | goose            | schema 第一次演进         |

### 待确认

| 层       | 候选                               | 倾向 |
| -------- | ---------------------------------- | ---- |
| 国际化   | 中文为主 / 中英双语                | 未决 |

---

## 目录结构

```
iot-pillot/
├── apps/
│   ├── web/                # 前端（Vue 3 + Element Plus + TS）
│   └── api/                # 后端 Go 服务（Gin + GORM），go.mod 在此
│       ├── cmd/            # 入口 + wire 装配 + swag 生成指令（generate.go）
│       ├── docs/           # swag 生成的 OpenAPI spec 与 SwaggerInfo（生成物，随源码入库）
│       ├── Dockerfile      # 独立 API 镜像（只有 Go 二进制；生产不走它）
│       └── .dockerignore   # 该独立构建用；根 context 构建读仓库根的 .dockerignore
├── packages/
│   └── shared-types/       # 前后端共享 TS 类型（DTO）
├── deploy/                 # 部署产物配置
│   ├── nginx.conf          # 完整 nginx 配置（含 events/http 块）
│   ├── entrypoint.sh       # 容器内两进程的生命周期管理
│   ├── smoke.sh            # 镜像冒烟测试（HTTP 断言）
│   └── smoke-lifecycle.sh  # 镜像冒烟测试（进程监督语义）
├── .github/
│   └── workflows/          # ci.yml + deploy.yml + auto-merge.yml
├── docs/
│   └── superpowers/specs/  # 设计文档
├── Dockerfile              # 单镜像：前端 + 后端 + nginx，context 为仓库根
├── .dockerignore           # 必须排除 node_modules
├── docker-compose.yml      # 本地 Postgres + Redis，以及复用根 Dockerfile 的 app 服务
├── pnpm-workspace.yaml
└── CLAUDE.md
```

---

## 架构原则

- **单二进制部署**：Go 产物可直接 `./iot-pilot-api` 运行，无 JVM/解释器/额外运行时
- **显式优于隐式**：避免反射、注解魔法、控制流清晰可见——未来接手无需先学框架再读代码
- **功能 = 模块**：5 个功能对应 5 个独立模块——`FormModule`、`TemplateModule`、`AuthModule`、`MailModule`、`RecruitmentModule`
- **类型全栈打通**：前后端共享 DTO，杜绝接口漂移
- **依赖最小化**：所有 npm/pnpm 与 Go 依赖都需有明确选型理由，避免随手安装
- **避免预留扩展点**：当下用不到的抽象不写，等真实需求出现再重构

---

## 前端开发计划

### 1. 当前基线与开发边界

前端当前已完成认证与角色分流的基础交付：`apps/web/src/views/AuthView.vue` 提供 `/login` 单页中的登录 / 注册模式切换，`apps/web/src/api/auth.ts` 对接认证、验证码、当前用户和管理员用户列表接口，`apps/web/src/auth/` 包含表单校验、提交编排、会话保存与刷新，相关行为由 Vitest 覆盖。后台完整布局和招新业务页面仍在规划中。后续开发必须以实际代码为准，不能把 `packages/shared-types` 中已经存在的类型误认为后端接口已经实现。

认证页的视觉与布局约束：桌面端使用固定 `100dvh` 双栏，左侧是全栈工作室招新品牌面板，右侧认证表单独立滚动；切换登录 / 注册时左侧面板不重新挂载、不随注册表单高度变化。宽度小于 `820px` 时隐藏左侧面板，表单恢复自然文档流。页面文案应继续突出以前端与后端开发为核心的 IoT 全栈项目实践。

本地联调认证页前，必须先启动 Postgres 和 Redis，并导出 `IOT_PILOT_JWT_SECRET`；`.env` 文件仅作参考，当前 Viper 配置加载不会自动读取它。完整启动步骤见 [README.md](./README.md)。

当前可以直接对接的后端能力：

| 能力 | 实际路由 | 前端可用性 | 备注 |
| --- | --- | --- | --- |
| 健康检查 | `GET /health` | 前端已对接 | `Home.vue` 使用根级 `/health`，不走 `/api/` 代理 |
| 登录 | `POST /api/v1/login` | 前端已对接 | `/login` 页面提交 `username`、`password`；返回 access/refresh 双令牌 |
| 注册 | `POST /api/v1/register` | 前端已对接 | 注册页发送邮箱验证码后提交姓名、邮箱、密码和验证码 |
| 发送验证码 | `POST /api/v1/send-verify-code` | 前端已对接 | 3D 开屏与 WebGL 兼容登录页均可发送邮箱验证码；手机号由后端暂未实现 |
| 刷新令牌 | `POST /api/v1/refresh` | 前端已对接 | 路由守卫在 access token 收到 401 时最多轮换一次并重试 `/me` |
| 退出登录 | `POST /api/v1/logout` | 前端已对接 | Header 携带 access token，请求体携带 refresh token；网络失败也清理本地会话 |
| 当前用户 | `GET /api/v1/me` | 前端已对接 | 路由守卫和用户首页读取真实的用户名与 `admin/member` 角色 |
| 管理员用户列表 | `GET /api/v1/admin/users` | 前端已对接 | 管理员控制台分页读取用户 ID、用户名和注册时间 |
| 表单、模板、招新、用户权限、SMTP 设置 | 尚无路由 | 阻塞 | 共享 TS 类型仅代表领域草案，不能直接当作已交付 API |

开始业务页面前必须先解决以下契约差异：

- 后端统一响应为 `{ code, message, data }`，而当前 `ApiResponse<T>` 只有 `data` 和可选 `meta`；应先统一成功、失败和分页协议。
- 后端登录返回 `user_id: number`，共享类型的通用 `ID` 当前定义为 UUID 字符串；在后端迁移到 UUID 前，认证 DTO 必须如实使用 number，禁止靠类型断言掩盖差异。
- `/api/v1/refresh` 当前复用 `LoginResp`，实际返回 `user_id: -1`；前端通过合并旧会话保留真实用户 ID。
- JWT 的 `admin/member` 角色由后端管理员白名单签发，前端通过 `/api/v1/me` 查询并由路由守卫控制可见性；真正权限仍由后端中间件强制。
- 注册 DTO 接收 `email`，但当前 `domain.User` 没有邮箱字段，注册逻辑也没有持久化邮箱；依赖邮箱的个人资料和招新关联功能必须等后端模型补齐。
- `packages/shared-types` 仅供 TypeScript 工作区消费，不是 Go 与 TypeScript 自动共享的 schema；每次后端 DTO 变化都必须同步核对。

### 2. 总体开发策略

采用“公共基础能力 + 业务纵向模块”的方式推进。每个模块包含自己的路由、页面、组件、API 封装、类型适配和测试，完成后应能独立演示和验收；不先铺满所有页面骨架，也不按 components/api/views 技术层把同一业务拆散。

开发顺序：

```text
M0 接口契约清理
  -> M1 前端基础设施
  -> M2 认证与会话
  -> M3 后台应用外壳
       -> M4 表单管理与匿名填写 --┐
       -> M5 邮件模板管理 --------┴-> M6 招新流程
       -> M7 用户权限与系统设置
M1-M7 -> M8 联调、测试与发布验收
```

其中 M4 与 M5 在接口契约稳定后可以并行开发；M6 同时依赖表单、模板、邮件和招新后端能力，最后接入。每个模块只引入当期确实需要的依赖，避免一次性搭建未经验证的“大前端框架”。

### 3. 目标目录结构

业务代码按领域就近放置，公共能力保持小而明确：

```text
apps/web/src/
├── api/                     # fetch 客户端、响应解包、错误映射、刷新令牌队列
├── assets/                  # 全局样式与静态资源
├── components/common/       # 跨模块复用：页面标题、空状态、状态标签、确认弹窗
├── layouts/                 # 后台主布局、公开页面布局
├── modules/
│   ├── auth/                # 登录、注册、会话状态
│   ├── dashboard/           # 首页与待办概览
│   ├── forms/               # 表单列表、编辑器、提交记录、公开填写页
│   ├── templates/           # 邮件模板列表、编辑与预览
│   ├── recruitment/         # 意向成员、候选人、状态流转与邮件动作
│   ├── users/               # 成员与角色管理
│   └── settings/            # SMTP 等系统设置
├── router/                  # 路由表、元信息与全局守卫
├── stores/                  # 仅放跨模块状态；首期主要是 auth/session
├── types/                   # 前端内部类型，不复制 shared-types 的领域 DTO
└── utils/                   # 无业务语义的格式化与校验函数
```

每个 `modules/<name>/` 默认包含 `api.ts`、`routes.ts`、`views/`、`components/` 和 `__tests__/`；只有真实需要时才创建对应目录。模块之间不得直接读取彼此组件内部状态，通过路由参数、共享 DTO 或明确的 store action 协作。

### 4. M0：接口契约与共享类型

**目标**：先让前端看到的类型、HTTP 行为和后端真实实现一致，避免后续页面建立在错误契约上。

开发内容：

- 在 `packages/shared-types` 补齐认证请求、认证响应、统一业务错误和分页响应类型，并明确 number ID 与 UUID ID 的过渡策略。
- 逐项维护“前端调用名 → HTTP 方法与路由 → 请求 DTO → 响应 DTO → 权限”的接口清单。
- 统一字段命名边界：网络层保持后端 snake_case，进入页面前是否转换 camelCase 只允许有一个固定策略，不在各组件内零散转换。
- 明确 `code !== 0`、非 2xx、网络断开、超时、401 和 403 的展示行为。
- 后端接口尚未交付时，允许模块使用与最终 DTO 一致的本地 fixture；fixture 只能服务界面开发，不能伪装成真实联调完成。

验收标准：类型检查能发现字段漂移；认证四个已挂载路由都有明确 DTO；前端不再直接把未知 JSON 强制断言成业务对象。

### 5. M1：前端基础设施

**目标**：为后续所有业务模块提供一致的请求、状态、导航和反馈机制。

开发内容：

- 建立基于原生 `fetch` 的轻量 API 客户端，统一 base URL、JSON 序列化、Authorization、响应解包、超时与错误对象；当前规模不引入 axios。
- 会话策略分阶段落地：当前认证页使用 `apps/web/src/auth/session.ts` 将后端返回的 access/refresh token 与 `user_id` 保存到 `localStorage` 的 `iot-pillot.auth` 键，绝不保存密码或验证码；后续引入 Pinia 和受保护后台时，再评估迁移为仅保存 token 的 `sessionStorage` 或 HttpOnly Cookie 方案，并同步更新验收标准。
- 当前路由守卫提供单次 401 refresh + `/me` 重试；全局并发刷新队列和请求重放仍待后续 API 客户端统一后实现。
- 扩展 Vue Router：公开路由、需登录路由、管理员路由、404/403 页面，并通过 route meta 统一守卫。
- 建立 Element Plus 的全局交互约定：提交中禁用、危险操作二次确认、成功提示、字段错误、页面级错误和空状态。
- 建立最小设计基线：色彩、间距、字体层级、表格密度、表单宽度和响应式断点。后台优先适配桌面与平板，匿名报名页必须优先保证手机可用。
- 引入 Vitest、Vue Test Utils 和 jsdom，先覆盖 API 客户端、路由守卫和 session store；业务核心流程稳定后再引入 Playwright。

验收标准：任意新模块无需自行处理 token、基础错误、全局 loading 或未登录跳转；`pnpm -r typecheck`、前端单测和 web build 都有稳定命令。

### 6. M2：认证与会话模块

**路由建议**：`/login`、`/register`。

页面与能力：

- 登录页（当前实现位于 `/login`）：用户名、密码、提交状态、服务端错误提示；成功后回到原目标页或后台首页。登录 / 注册切换不改变左侧品牌面板布局。
- 注册页（当前与登录共用 `/login`）：姓名、邮箱、验证码、密码、确认密码、验证码倒计时；两个登录入口都可以发送邮箱验证码。
- 会话恢复：刷新页面后恢复 token，受保护路由在 access token 过期时调用 refresh 并重试 `/me`；刷新失败回到登录页。
- 主动退出：调用 logout，成功或 token 已失效时都清理本地会话。
- 多端冲突处理：后端重新登录会使旧会话失效，前端收到对应 401 后提示“账号已在其他位置重新登录”。

验收标准：登录、注册验证码、刷新、退出形成完整闭环；重复点击不会产生并发提交；密码和 token 不出现在日志、URL、错误详情或持久化调试数据中。

### 7. M3：后台应用外壳与仪表盘

**目标**：建立所有管理模块共享的工作区，而不是让每个页面单独实现导航和权限判断。

页面与能力：

- 后台主布局：侧边导航、顶部用户菜单、面包屑、内容区和移动端收起行为。
- 导航项按角色显示，但隐藏菜单不等于权限控制；所有敏感操作仍必须由后端鉴权。
- 仪表盘只展示已存在接口能支持的指标。后端聚合接口未完成前使用空状态或开发 fixture，不在浏览器端抓取多页列表后计算“总览”。
- 全局页面状态：首次加载骨架、局部刷新、空数据、无权限、资源不存在和服务异常。
- 保留健康状态入口供管理员诊断，但使用真正的 `/health`，不把健康检查当业务 API。

验收标准：登录后能稳定进入后台外壳；直接访问受保护 URL 的行为正确；刷新和浏览器前进/后退不丢失导航状态。

### 8. M4：表单管理与匿名填写模块

**后台路由建议**：`/forms`、`/forms/new`、`/forms/:id/edit`、`/forms/:id/submissions`。

**公开路由建议**：`/f/:shareToken`，放在后台布局和登录守卫之外。

页面与能力：

- 表单列表：标题、开放状态、提交数、创建时间；支持创建、编辑、开放/关闭、复制分享链接和查看提交。
- 表单编辑器：基础信息与字段列表；首期只实现 `text`、`email`、`textarea`、`select`、`checkbox` 五种共享类型已经定义的字段。
- 字段配置：标题、必填、占位提示、选项；支持新增、删除、排序和预览。首期不做复杂条件逻辑、分页问卷或拖拽设计器。
- 匿名填写页：根据 share token 拉取定义，完成客户端基础校验、提交防重、成功反馈、关闭状态和无效链接处理；必须适配手机。
- 提交记录：分页列表、单条详情和字段值展示；涉及导出时另开需求，不预先加入 CSV/Excel 功能。

数据流：后台创建表单 → 获取 share token → 公开页面拉取定义 → 匿名提交 → 后端生成 submission，并按业务规则生成 prospect。前端不能自行推断或创建 prospect。

验收标准：五种字段均能创建、预览、填写和回显；关闭表单后公开页不能提交；刷新页面不会误重复提交；无效 share token 有明确反馈。

### 9. M5：邮件模板管理模块

**路由建议**：`/templates`、`/templates/new`、`/templates/:id/edit`。

页面与能力：

- 模板列表：名称、类型、更新时间和操作；支持 invitation、offer、rejection、custom 四类。
- 模板编辑：名称、主题、正文和变量提示；变量语法与后端最终渲染规则保持一致。
- 变量扫描与预览：展示 `{{name}}` 等变量，允许输入样例值预览最终主题和正文；预览仅用于界面反馈，正式发送必须由后端重新渲染和校验。
- 删除操作显示引用风险并二次确认；若模板正在被流程使用，由后端拒绝并返回可理解的业务错误。
- 首期使用安全的纯文本或受控 HTML 预览，禁止把未经清洗的模板内容直接通过 `v-html` 注入页面。

验收标准：四类模板可以创建、编辑、预览和删除；缺失变量有明确提示；恶意 HTML 不会在管理端执行。

### 10. M6：招新流程模块

**路由建议**：`/recruitment/prospects`、`/recruitment/candidates`、`/recruitment/candidates/:id`。

MailModule 当前是后端能力，不单独创建前端 `modules/mail/`；招新页面消费邀请/offer/感谢信接口，SMTP 配置页面归 `modules/settings/`。

页面与能力：

- 意向成员列表：按状态、来源表单和关键词筛选；查看原始报名信息；选择邀请模板并发送邀请。
- 候选人列表：按 interviewing、offered、hired、rejected 状态筛选；展示关键时间和待处理动作。
- 候选人详情：基础信息、报名来源、面试备注、状态时间线和邮件动作记录。
- 状态操作：接受邀请、发送 offer、录用、拒绝；每个动作都展示目标状态、模板和影响，并防止重复提交。
- 批量操作放到单条流程稳定之后；首期不实现看板拖拽，先用表格、筛选和明确按钮保证状态流转可靠。

状态展示必须以后端返回值为准，前端只限制明显非法入口，不能在请求成功前乐观改变候选人最终状态。发送邮件失败时保留原状态并允许安全重试。

验收标准：从 prospect 到 candidate，再到 offered/hired 或 rejected 的主路径可追踪；非法状态跳转被阻止；每次邮件和状态动作都有确定的成功或失败反馈。

### 11. M7：用户权限与系统设置

此阶段拆为两个独立子模块，均只对管理员开放：

**用户与权限**

- 用户列表、角色显示、角色变更和状态管理。
- 前端路由及按钮按 `admin | member` 控制可见性；后端 403 始终作为最终判定。
- 修改自己的角色、禁用当前账号等高风险动作由后端规则决定，前端展示明确限制。

**SMTP 设置**

- 展示 host、port、username、from 等非敏感配置，密码只能重新填写，绝不回显。
- 提供“保存配置”和“发送测试邮件”两个独立动作，分别反馈校验、连接和发送结果。
- 设置更新后的生效方式由后端契约明确；前端不假设一定热更新，也不在浏览器保存 SMTP 凭据。

验收标准：成员无法进入管理员页面或调用管理员操作；敏感配置不进入响应、日志或本地存储；测试邮件不会被误认为配置已保存。

### 12. M8：联调、测试与发布验收

质量门禁按风险分层：

- **类型与构建**：继续执行 `pnpm -r typecheck` 与 `pnpm --filter @iot-pillot/web build`。
- **单元测试**：覆盖响应解包、错误映射、表单校验、模板变量解析和状态转换判断。
- **组件测试**：覆盖登录表单、动态字段渲染、危险操作确认和权限可见性。
- **端到端测试**：至少覆盖登录/刷新/退出、公开报名、发送邀请、发送 offer、拒绝候选人五条主流程；外发邮件通过测试环境或后端桩验证，不向真实地址发送。
- **可访问性**：表单 label、键盘操作、焦点回收、错误提示关联和颜色对比度达到后台工具的基本可用要求。
- **浏览器与响应式**：桌面端覆盖当前主流 Chromium；匿名报名页额外验证常见手机宽度。
- **发布回归**：验证 history 路由刷新、`/health`、`/api/` 反代、静态资源缓存和未登录跳转，确保与现有 nginx 单镜像部署一致。

模块完成定义（Definition of Done）：页面与接口联调完成、加载/空/错/无权限状态齐全、关键测试通过、无控制台错误、共享 DTO 已同步、相关文档已更新。只有 fixture 截图或静态页面不算模块完成。

### 13. 里程碑与交付节奏

| 里程碑 | 包含模块 | 可交付结果 | 前置条件 |
| --- | --- | --- | --- |
| F1 可登录后台 | M0 + M1 + M2 + M3 | 登录、刷新、退出、受保护后台外壳 | 补挂验证码路由后才能同时交付注册 |
| F2 可收集报名 | M4 | 管理员建表并分享，申请人匿名提交 | Form API、公开查询/提交 API |
| F3 可管理邮件模板 | M5 | 四类模板 CRUD、变量预览 | Template API 与渲染规则确定 |
| F4 可推进招新流程 | M6 | 邀请、候选人处理、offer/感谢信 | Recruitment + Mail API，依赖 F2/F3 |
| F5 可分权运营 | M7 | admin/member 权限和 SMTP 配置 | 用户角色、鉴权中间件、设置 API |
| F6 可稳定发布 | M8 | 核心 E2E、回归清单、CI 质量门禁 | F1-F5 主流程完成 |

每个里程碑使用独立 PR 或一组可独立回滚的小 PR。前后端接口未就绪时，前端 PR 只交付可验证的 UI/fixture 层，并在 PR 中明确阻塞项；接口真正联调通过后再关闭对应里程碑。

---

## 后续未决项（开发前需明确）

- [ ] 部署目标平台（VPS / Railway / Fly.io）
- [x] 基础鉴权方案（用户名密码 + access/refresh JWT）已落地；`admin/member` RBAC 与前端路由守卫已联调
- [ ] 邮件模板渲染选型（`html/template` + Handlebars 子集 或纯文本+占位符）
- [ ] 数据库迁移工具（golang-migrate / goose）
- [ ] 国际化范围（仅中文 / 中英双语）
- [ ] Postgres 备份策略（频度、保留周期、灾备位置）

---

## 工作约定

- 贡献流程遵循 [CONTRIBUTING.md](./CONTRIBUTING.md)：禁止在 `main` 上直接开发，走分支 + PR；commit 遵循 Conventional Commits；合并由 [`.github/workflows/auto-merge.yml`](./.github/workflows/auto-merge.yml) 自动处理（成员身份 + 无冲突 → squash merge）
- Go 后端代码默认落在 `apps/api/`
- 前端代码默认落在 `apps/web/`
- 跨前后端的类型定义落在 `packages/shared-types/`
- 数据库 schema 变更**必须**通过迁移文件管理，不直接改表
- 所有 SMTP 外发**统一**走 `MailModule`，业务代码不直接调用 SMTP 库
- 所有模板渲染**统一**走 `TemplateModule`，避免散落字符串拼接
- 所有权限判断**统一**走 `AuthModule` 的守卫/中间件，不在 handler 里 if-else 角色

---

## 决策日志

| 日期       | 决策                                       | 理由                                                                                       |
| ---------- | ------------------------------------------ | ------------------------------------------------------------------------------------------ |
| 2026-09-09 | 后端选 Go（排除 NestJS、Express、FastAPI） | 招新规模适配，单二进制部署契合"代代传承"；开发/部署/CI 全链路 Go 更轻                      |
| 2026-09-09 | 部署走 GitHub Actions + ghcr.io            | 与 GitHub 仓库同源，免费私有，PR 反馈快（Go 单测秒级）                                     |
| 2026-09-09 | 前端定 Vue 3 + Element Plus + TS           | Element Plus 同样覆盖管理后台表格/表单场景                                                 |
| 2026-09-09 | 启用 GitHub Actions auto-merge 机器人      | 成员身份 + 无冲突 → 自动 squash merge，代替手工 review；下一 PR 加 CI 后回填 status check |
| 2026-09-09 | 加 `ci.yml` + `deploy.yml`，bot 终于有 status check 可等 | `ci.yml`（PR 触发 Go vet/build/test + 前端 typecheck/build）；`deploy.yml`（push main → Docker build → SCP → SSH 部署到 VPS）；同期把 README 残留测试注释删掉、补 `.env.example`、整理 CLAUDE.md 与 go.mod 一致性 |
| 2026-09-09 | 文档收尾：README 填实快速开始 + 项目状态、docs/architecture.md 落地、.gitignore 覆盖 Vite/unplugin/vue-tsc 副产物 | README 补 install/dev/build/test 命令与项目状态表；docs/architecture.md 记录当前架构 + 5 个业务模块引入顺序 + 数据模型与状态机；.gitignore 加 *.tsbuildinfo 与 apps/web/{auto-imports.d.ts, components.d.ts, vite.config.{d.ts,js}} 避免 pnpm build 污染 PR |
| 2026-09-20 | 接口文档选 swag v1（Swagger 2.0）+ gin-swagger：UI 挂 `/docs`，spec 挂 `/openapi.json` | handler 里早已写好 swaggo 注解，只差生成器与 UI；FastAPI 的 `/docs` 本身就是 Swagger UI，界面观感一致。swag v2（OpenAPI 3.1）仍是 rc，暂不引入。默认 debug 开、release 关（`docs.enabled` 可覆盖）——文档会暴露整个接口面。生成物随源码入库，镜像/CI 只跑 go build，不装 swag CLI |
