# iot-pillot

IoT 全栈开发工作室 [IOT-devstudio](https://github.com/IOT-devstudio) 的后台管理 web 程序。
**当前阶段：招新管理**。

- 仓库：https://github.com/IOT-devstudio/iot-pillot
- 计划与决策：[CLAUDE.md](./CLAUDE.md)
- 架构说明：[docs/architecture.md](./docs/architecture.md)
- 贡献规范：[CONTRIBUTING.md](./CONTRIBUTING.md)

---

## 项目简介

本系统支撑工作室招新全流程：

- 表单系统（登录后填写 + 分享链接）
- SMTP 邀请流程（prospect → candidate）
- 候选人处理（一键 offer / 感谢信）
- 邮件模板 CRUD
- 管理员 / 成员 RBAC
- 可替换的 SMTP 发件配置

---

## 项目状态

两条贯穿全文的业务约束：**报名与表单提交必须登录**（分享链接进入也需先登录，不支持匿名提交）；**账号绑定邮箱不可修改**（登录凭据，密码找回也使用原邮箱）。

| 模块 | 状态 |
| --- | --- |
| Monorepo 脚手架（apps/web + apps/api + packages/shared-types） | ✅ 落地 |
| 共享 TS 类型（招新 / 表单 / 模板 / 通用） | ✅ 落地 |
| 本地依赖编排：Postgres + Redis + 全栈 app（docker-compose） | ✅ 落地 |
| `auto-merge.yml`：有写权限成员的非 Draft PR + 无冲突 → squash merge 并删远端分支（不等待 CI） | ✅ 落地 |
| `ci.yml`：PR 触发 Go vet/build/test、前端 typecheck/test/build，以及 Docker 镜像构建与冒烟测试 | ✅ 落地 |
| `deploy.yml`：auto-merge 成功后触发部署，也支持手动触发（需配置服务器 secrets） | ✅ 落地 |
| 认证（`/` 与 `/login`） | ✅ 登录、注册、邮箱验证码、刷新令牌与登出 API 已接入；发码需配置 SMTP；姓名可编辑、绑定邮箱不可修改。个人设置默认只读，「更改」后可编辑姓名/班级/学号/QQ；班级和学号在前端必填，每次登录检查并提示补全；方向根据最新报名自动带出，冲突时禁止保存并提示联系管理员。密码找回与修改密码已接入 |
| 用户侧页面（个人首页 `/user/home` / 个人设置 `/user/settings` / 关于我们 `/user/about`） | ✅ 页面与导航已落地；首页方向与申请记录已通过 #89 接入真实招新 API；个人设置规则与验证边界见 [#101 前端说明](apps/web/docs/profile-completion.md)；关于页的年份/人数/成果/成员为「待补充」状态 |
| 管理端（控制台 / 用户与权限 / 邮件中心） | ✅ 仪表盘、用户与管理员管理、邮件模板 CRUD 与发送记录页面及对应受保护 API 已落地 |
| 邮件能力 | 🟡 邮件中心页面与后端模板 CRUD、发送和记录 API 已落地；招新邀请/offer/感谢信的后端发送已接入 MailModule 真实路径，前端页面仍用本地 fixture |
| 招新（邀请 / 候选人后端） | ✅ BE-04 已落地：发送/撤销邀请、本人接受（幂等/归属校验/过期撤销拒绝）、候选人推进（send_offer/hire/reject，邮件失败不落状态）与两端列表；迁移 0008 |
| 招新（用户端：方向 / 报名 / 申请进度） | ✅ 前端已接入真实 API（#89）：首页读真实方向与开放状态，登录报名，进度/时间线来自服务端 |
| 招新（管理端：轮次 / 方向开关 / 审核） | ✅ 申请审核页已接入（#89）：申请列表、状态筛选、基础审核与轮次/方向配置；邀请与候选人流转前端待 #91 |
| 招新（前端页面） | 🟡 前端流程页面已落地，目前使用本地 fixture，待 FE-04 接入 BE-04 API |
| 表单管理 | 🟡 后端 API 已落地（BE-05：创建/编辑/分享/登录提交/提交记录，提交强制登录）；前端为占位路由，接入见 #93 |
| 系统设置（SMTP） | ⏳ 当前为占位路由，待实现 |

---

## 技术栈（详见 [CLAUDE.md](./CLAUDE.md)）

### 已落地

- **前端**：Vue 3 + Element Plus + TypeScript + Vite
- **后端**：Go + Gin + Viper
- **数据库**：PostgreSQL
- **数据库迁移**：golang-migrate；部署在切换应用前自动执行，首次接管已有库需先校验迁移清单
- **Monorepo**：pnpm workspaces
- **CI/CD**：GitHub Actions（auto-merge + ci + deploy）

### 已规划（业务模块开工时引入）

GORM · go-mail · `html/template` + Handlebars 子集 · casbin

---

## 快速开始

```bash
# 0. 准备：Node >= 18、Go >= 1.25、pnpm、Docker
node --version && go version && pnpm --version && docker --version

# 1. 克隆（推荐 SSH）
git clone git@github.com:IOT-devstudio/iot-pillot.git
cd iot-pillot

# 2. 启动本地依赖（Postgres + Redis）
#    两者都是后端启动期硬依赖：DB 连不上 ProvideDB 返回错误，Redis 连不上直接退出。
docker compose up -d postgres redis

# 3. 配置后端环境变量
cp .env.example .env
# 当前 API 不会自动读取 .env；请将必需配置导出到进程环境。
# JWT secret 必填，SMTP_* 暂时可以留空。
# PowerShell：$env:IOT_PILOT_JWT_SECRET = "dev-only-change-me"
# macOS/Linux：export IOT_PILOT_JWT_SECRET=dev-only-change-me

# 4. 安装前端依赖
pnpm install

# 5. 先迁移数据库，再启动两个 dev server（两个终端窗口）
(cd apps/api && go run ./tools/migrate up)
# 已有手工迁移库先 check/adopt，见迁移说明；不要从头重放 SQL。
# 以下各命令从仓库根目录执行：
cd apps/api && go run ./cmd         # 监听 :8080
cd apps/web && pnpm dev             # 监听 :5173，/api 代理到 :8080

# 6. 浏览器打开 http://localhost:5173/login
#    登录页支持登录 / 注册模式切换；注册需先发邮箱验证码（需后端配置 SMTP）
#    后端健康检查地址为 http://localhost:8080/health，就绪探针为 /health/ready
```

数据库本地执行、已有库接管和失败恢复见[迁移操作说明](./apps/api/migrations/README.md)。迁移失败会中止部署并保留旧容器；应用镜像回滚不会自动回滚数据库。

### 接口文档与在线调试（Swagger UI）

后端起来之后：

- `http://localhost:8080/docs` —— Swagger UI，与 FastAPI 的 `/docs` 是同一套界面。
  右上角 **Authorize** 填登录拿到的 access_token，就能在页面上直接 "Try it out"。
- `http://localhost:8080/openapi.json` —— spec 本体。

默认**跟随运行模式**：`mode: debug` 开、`release` 关（文档会把整个接口面暴露出来）。
要单独控制就设 `docs.enabled`（或环境变量 `IOT_PILOT_DOCS_ENABLED`）。

spec 由 handler 上的 swaggo 注解生成，改完接口重新生成（生成物随源码入库，镜像与 CI
只跑 `go build`，不装 swag CLI）：

```bash
cd apps/api && go generate ./cmd
```

### 一键起全栈（可选，不调代码时用）

```bash
docker compose up -d --build        # 前端产物 + nginx + Go API，全部在仓库根 Dockerfile 里
# 浏览器打开 http://localhost:8080（nginx 托管前端并反代 /api）
```

这一步会占用 8080，别再同时跑 `go run ./cmd`。只想单独跑后端容器时用
`apps/api/Dockerfile`（context 是 `apps/api`）；生产部署走的是根 `Dockerfile`。

国内网络构建全栈镜像需要 Go module 镜像：`docker-compose.yml` 的 `app.build.args`
已默认取 `IOT_PILLOT_GOPROXY`（默认 `https://goproxy.cn,direct`）；
`apps/api/Dockerfile` 不带这个默认值，需要显式传
`--build-arg GOPROXY=https://goproxy.cn,direct`。CI 与生产镜像在 GitHub runner 上构建，
走 Dockerfile 的官方源。

---

## 生产部署配置

生产部署由 `.github/workflows/deploy.yml` 完成。除了服务器、数据库、Redis、JWT 与
SMTP 的 GitHub Actions Secrets，还必须在仓库的
`Settings → Secrets and variables → Actions → Variables` 中配置：

```text
REDIS_DB=1
ADMIN_USERS=1
```

`REDIS_DB` 必须与生产 Redis 实际使用的逻辑库一致。管理员角色的真源是该逻辑库中的
Redis SET `auth:admins`；应用启动时会把 `ADMIN_USERS` 中已存在的用户名或用户 ID
补种进集合。因此首次部署前应先确保对应用户已经存在，或在注册后重新部署/重启应用。

部署 workflow 自身属于部署判定路径：修改 `.github/workflows/deploy.yml` 合并到
`main` 后，也会触发重新构建和部署，不需要额外手动选择 `force`。

---

## 贡献流程（速览）

详细规范见 [CONTRIBUTING.md](./CONTRIBUTING.md)。核心规则：

1. **禁止在 `main` 上直接开发**。所有改动走分支 + PR。
2. **Rebase main 再切新分支**：`git fetch && git merge --ff-only origin/main && git checkout -b <type>/<desc>`
3. **分支命名**：`feat/`、`fix/`、`docs/`、`refactor/`、`test/`、`chore/`、`perf/`、`hotfix/`
4. **Commit 规范**：Conventional Commits——`<type>(<scope>): <subject>`
5. **PR 合并**：由机器人自动 squash merge（成员 + 无冲突）；合并后远程分支自动删除

---

## 文档索引

- [CLAUDE.md](./CLAUDE.md) — 项目身份、功能范围、技术栈、架构原则、决策日志
- [docs/architecture.md](./docs/architecture.md) — 当前架构、5 个业务模块规划、API/数据模型规划
- [CONTRIBUTING.md](./CONTRIBUTING.md) — 分支策略、commit 规范、PR 流程、review 期望
