# Architecture

本文档记录 iot-pillot 的当前架构状态——脚手架已落地、业务模块待开工。

---

## 仓库布局

```
iot-pillot/
├── apps/
│   ├── web/                # 前端（Vue 3 + Element Plus + Vite + TS）
│   └── api/                # 后端（Go + Gin + Viper，cmd/api + internal/）
├── packages/
│   └── shared-types/       # 前后端共享 TypeScript 类型（DTO）
├── .github/workflows/      # auto-merge.yml + ci.yml + deploy.yml
├── docs/                   # 本目录：架构说明
├── docker-compose.yml      # 本地 Postgres
├── .env.example            # 后端环境变量模板
├── pnpm-workspace.yaml
├── apps/api/{go.mod, go.sum}
└── CLAUDE.md / README.md / CONTRIBUTING.md
```

---

## 已落地

### 前端（apps/web）

- Vue 3 + `<script setup>` 单文件组件
- Vue Router 4，初始路由 `/` -> Home
- Element Plus 通过 `unplugin-auto-import` + `unplugin-vue-components` 按需注册
- Vite dev server 监听 `:5173`，`/api` 代理到 `:8080`
- Home 视图调 `/api/health` 验证前后端联通

### 后端（apps/api）

- Go 1.25 + Gin + Viper
- `cmd/api/main.go` -> `internal/config.Load()` -> `internal/server.New()` -> `:8080`
- `internal/handler/` 装 HTTP handler，目前只有 `health.go`
- `internal/middleware/cors.go` 手写白名单 CORS（不依赖 `gin-contrib/cors`）
- 配置优先级：env `IOT_PILOT_*` > `config.yaml`（可选） > 默认值
- 多阶段 `Dockerfile`：golang:1.25-alpine -> alpine:3.20

### 共享类型（packages/shared-types）

- `common.ts`：`ApiResponse<T>`、`PaginationMeta`、`ApiErrorBody`、`Role`
- `recruitment.ts`：`Prospect`、`Candidate` + 状态机（详见下）
- `form.ts`：`FormDefinition`、`FormField`、`FormSubmission`、`CreateFormInput`
- `template.ts`：`EmailTemplate`、`CreateTemplateInput`

被 `apps/web` 通过 `workspace:*` 协议引用，类型导入示例：

```ts
import type { ApiResponse, Prospect, EmailTemplate } from "@iot-pillot/shared-types";
```

### CI/CD

| Workflow | 触发 | 行为 |
| --- | --- | --- |
| `auto-merge.yml` | PR open/sync/reopen + check_run completed | 检查成员 + 无冲突 -> `pulls.merge` (squash) + `git.deleteRef` |
| `ci.yml` | PR + push main | Go `vet`/`build`/`test` + 前端 `typecheck`/`build`（ubuntu-latest） |
| `deploy.yml` | push main + manual dispatch | Docker build -> SCP -> SSH 部署到 VPS（需 secrets: `HOST`/`USERNAME`/`SSH_KEY`/`PORT`） |

Branch protection：`enforce_admins=true`、`required_approving_review_count=0`、`required_linear_history=true`、`allow_force_pushes=false`。

---

## 业务模块规划（待开工）

按依赖关系排序：

```
            ┌──────────────────┐
            │   AuthModule     │  (依赖：鉴权方案先定)
            └────────┬─────────┘
                     │ 提供 guards / 中间件
       ┌─────────────┼─────────────┐
       ▼             ▼             ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│FormModule│  │TplModule │  │MailModule│  (三个互相独立)
└────┬─────┘  └────┬─────┘  └────┬─────┘
     └─────────────┴─────────────┘
                   ▼
           ┌──────────────┐
           │RecruitModule │  (组合上面 4 个模块)
           └──────────────┘
```

### AuthModule

- **依赖**：鉴权方案（本地邮箱密码 + JWT vs OAuth）—— 待定
- **提供**：登录/登出、当前用户查询、`RequireRole("admin")` 中间件
- **后续依赖**：引入 casbin 时升级为策略驱动的 RBAC

### FormModule（独立域，可先行）

- **路由**（`/api/v1/forms`）：
  - `POST /forms` 创建表单（admin）
  - `GET /forms` 列表（admin/member）
  - `GET /forms/:id` 详情
  - `PATCH /forms/:id` 修改（admin）
  - `DELETE /forms/:id` 删除（admin）
  - `GET /forms/public/:shareToken` 匿名获取表单定义
  - `POST /forms/public/:shareToken/submit` 匿名提交（限流）
- **依赖**：DB（GORM）+ 限流中间件
- **引入 GORM 时机**：本模块开工时

### TemplateModule（独立域，可先行）

- **路由**（`/api/v1/templates`）：
  - `POST /templates` 创建模板（admin）
  - `GET /templates` 列表（admin/member）
  - `GET /templates/:id` 详情
  - `PATCH /templates/:id` 修改（admin）
  - `DELETE /templates/:id` 删除（admin）
  - `POST /templates/:id/preview` 渲染预览（用 mock data）
- **依赖**：模板渲染（`html/template` + Handlebars 子集 vs 纯文本）—— 待定
- **引入**：`html/template` 是 Go 标准库，无需新依赖

### MailModule（独立域，可先行）

- **接口**：`Mailer.Send(to, subject, body) error`
- **实现**：基于 `go-mail`（或 `gomail`），从 `MailModule` 配置读取 SMTP 凭据
- **职责**：统一 SMTP 外发入口，业务代码不直接调用 SMTP 库
- **可替换 SMTP**：通过替换 `MailModule` 内的 SMTP 配置实现，不动业务代码
- **引入**：`go-mail` 在本模块开工时

### RecruitmentModule（最后开工）

- **依赖**：Form + Template + Mail + Auth 全部就位
- **路由**（`/api/v1/recruitment`）：
  - `GET /prospects` 列表 + 筛选
  - `POST /prospects/:id/invite` 发送邀请函（用 TemplateModule 渲染 + MailModule 发送）
  - `POST /prospects/:id/accept` 接受邀请 -> 升级为 candidate
  - `GET /candidates` 列表
  - `POST /candidates/:id/offer` 一键发送 offer
  - `POST /candidates/:id/reject` 一键发送感谢信
- **数据模型**：完全沿用 `packages/shared-types/src/recruitment.ts`

---

## 数据模型（参考 `packages/shared-types`）

### 状态机

```
ProspectStatus:   pending ─► invited ─► accepted
                       └────────────► rejected
                                  │
                                  ▼
CandidateStatus:  interviewing ─► offered ─► hired
            │                       │
            └──────────────────────► rejected
```

### 关键字段

- `Prospect.email`：用于 SMTP 发送邀请函的主键
- `Prospect.sourceFormId`：来自哪个表单（form 提交自动创建 prospect）
- `Prospect.shareToken`：表单匿名访问令牌（`FormDefinition.shareToken`）
- `Candidate.notes`：面试记录（自由文本，admin 编辑）
- `EmailTemplate.variables`：从 body 扫描出的占位符名（`{{name}}` 等），前端用来渲染表单、后端用来校验必填字段

---

## 当前不在范围

下列事项是 audit 已识别但**不在本 PR 范围**的，明确列出避免误解：

- 鉴权方案（JWT / OAuth 选型）
- 模板渲染方案（`html/template` + Handlebars 子集 vs 纯文本+占位符）
- 数据库迁移工具（golang-migrate vs goose）
- 数据库 schema（业务模块开工时才需要）
- 国际化范围（仅中文 / 中英双语）
- Postgres 备份策略
- Docker 镜像发布到 ghcr.io 的 release workflow（当前 `deploy.yml` 走 SCP 直推，不依赖镜像仓库）

每项开工前会单独 PR + 决策记录。
