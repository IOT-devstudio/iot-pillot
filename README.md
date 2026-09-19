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
| `auto-merge.yml`：成员 + 无冲突 → 自动 squash merge + 删分支 | ✅ 落地 |
| `ci.yml`：PR 触发 Go vet/build/test + 前端 typecheck/build | ✅ 落地 |
| `deploy.yml`：push main → VPS 部署（需 secrets 配置） | ✅ 落地 |
| Auth 登录注册页（`/login`） | 🟡 页面与接口适配已落地；注册验证码待后端接入 |
| 业务模块（Form / Template / Mail / Recruitment） | ⏳ 空白，待开工 |

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
