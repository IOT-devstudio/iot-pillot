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

- 表单系统（匿名填写 + 分享链接）
- SMTP 邀请流程（prospect → candidate）
- 候选人处理（一键 offer / 感谢信）
- 邮件模板 CRUD
- 管理员 / 成员 RBAC
- 可替换的 SMTP 发件配置

---

## 项目状态

| 模块 | 状态 |
| --- | --- |
| Monorepo 脚手架（apps/web + apps/api + packages/shared-types） | ✅ 落地 |
| 共享 TS 类型（招新 / 表单 / 模板 / 通用） | ✅ 落地 |
| 本地依赖编排：Postgres + Redis + 全栈 app（docker-compose） | ✅ 落地 |
| `auto-merge.yml`：有写权限成员的非 Draft PR + 无冲突 → squash merge 并删远端分支（不等待 CI） | ✅ 落地 |
| `ci.yml`：PR 触发 Go vet/build/test、前端 typecheck/test/build，以及 Docker 镜像构建与冒烟测试 | ✅ 落地 |
| `deploy.yml`：auto-merge 成功后触发部署，也支持手动触发（需配置服务器 secrets） | ✅ 落地 |
| 认证（3D 开屏 / `/login` 备用页） | ✅ 登录、注册、邮箱验证码、刷新令牌与登出 API 已接入；发码需配置 SMTP |
| 管理端（控制台 / 用户与权限） | ✅ 仪表盘、用户和管理员管理页面及对应受保护 API 已落地 |
| 邮件能力 | 🟡 后端模板 CRUD、发送与记录 API 已落地；模板管理页面及招新流程的真实邮件联调待完成 |
| 招新（意向成员 / 详情） | 🟡 前端流程页面已落地，目前使用本地 fixture；后端业务 API 待实现 |
| 表单管理 / 系统设置 | ⏳ 当前为占位页面，待实现 |

---

## 技术栈（详见 [CLAUDE.md](./CLAUDE.md)）

### 已落地

- **前端**：Vue 3 + Element Plus + TypeScript + Vite
- **后端**：Go + Gin + Viper
- **数据库**：PostgreSQL
- **Monorepo**：pnpm workspaces
- **CI/CD**：GitHub Actions（auto-merge + ci + deploy）

### 已规划（业务模块开工时引入）

GORM · go-mail · `html/template` + Handlebars 子集 · casbin · golang-migrate

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

# 5. 启动两个 dev server（两个终端窗口）
cd apps/api && go run ./cmd         # 监听 :8080
cd apps/web && pnpm dev             # 监听 :5173，/api 代理到 :8080

# 6. 浏览器打开 http://localhost:5173/login
#    登录页支持登录 / 注册模式切换；注册验证码按钮在后端路由接入前保持禁用
#    后端健康检查地址为 http://localhost:8080/health，就绪探针为 /health/ready
```

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
