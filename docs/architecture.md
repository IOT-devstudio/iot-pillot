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
- Vue Router 4，`/` 与 `/login` 为瑞士信号 2D 认证界面，用户侧为 `/user/*`，管理侧为 `/admin/*` 与 `/dashboard`
- Element Plus 通过 `unplugin-auto-import` + `unplugin-vue-components` 按需注册
- Vite dev server 监听 `:5173`，`/api` 代理到 `:8080`
- Home 视图调根级 `/health` 验证前后端联通；认证页、用户页和管理员控制台已接入当前后端认证接口

### 后端（apps/api）

- Go 1.25 + Gin + Viper
- `cmd/api/main.go` -> `internal/config.Load()` -> `internal/server.New()` -> `:8080`
- `internal/handler/` 装 HTTP handler，目前只有 `health.go`
- `internal/middleware/cors.go` 手写白名单 CORS（不依赖 `gin-contrib/cors`）
- 配置优先级：env `IOT_PILOT_*` > `config.yaml`（可选） > 默认值
- 多阶段 `Dockerfile`：golang:1.25-alpine -> alpine:3.20

### 共享类型（packages/shared-types）

- `common.ts`：`ApiResponse<T>`、`PaginationMeta`、`ApiErrorBody`、`Role`
- `recruitment.ts`：BE-04 邀请/候选人 DTO（number ID）+ `Prospect`、`Candidate` 草案与状态机（详见下）
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
  - `GET /forms/public/:shareToken` 分享链接读取表单定义（需登录；shareToken 仅用于定位分享表单，不构成匿名访问）
  - `POST /forms/public/:shareToken/submit` 登录用户经分享链接提交（需登录 + 限流）
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

### RecruitmentModule（BE-04 邀请/候选人流程已落地）

- **依赖**：Template + Mail + Auth（发邀请、offer、感谢信复用 MailModule 的模板发送路径）
- **路由**（BE-04 交付，见 `internal/handler/router.go`）：
  - 管理端（`/api/v1/admin`，RequireAdmin）：
    - `POST /invitations/send` 对报名申请发邀请函（pending 邀请复用 + 幂等重投递）
    - `GET /invitations` 邀请列表（可按申请过滤）
    - `POST /invitations/:id/revoke` 撤销 pending 邀请
    - `GET /candidates` 候选人列表；`GET /candidates/:id` 详情（含时间线）
    - `POST /candidates/:id/advance` 状态推进（send_offer / hire / reject）
  - 用户端（`/api/v1`，需登录）：
    - `GET /me/invitations` 本人邀请（含接受凭据 token）
    - `POST /me/invitations/accept` 接受邀请 -> 升级为 candidate（仅归属申请人，幂等）
    - `GET /me/candidates` 本人候选人记录
- **不变式**：邮件投递失败不落状态（邀请不算已邀请、候选人不流转），重试不产生重复流转；过期是查询期判定（pending 且超过 expires_at），无后台任务。
- **数据模型**：`apps/api/migrations/0008_invitation_flow.up.sql`（invitations + candidates + candidate_timeline）；报名申请 `recruitment_applications` 由 #88（迁移 0007）拥有，BE-04 通过 `ApplicationStore`（消费者声明的窄接口，实现方是 `repository.RecruitmentRepo`）读取与推进其状态，不再有本票的锚点表。
- **共享 DTO**：`packages/shared-types/src/recruitment.ts` 的 BE-04 段（ID 一律 number）。

---

## 数据模型（参考 `packages/shared-types`）

### 状态机

```
InvitationStatus: pending ─► accepted
            │             （expired = pending 且 now > expires_at，查询期判定）
            └─► revoked

CandidateStatus:  interviewing ─► offered ─► hired
            │                       │
            └──────────────────────► rejected
```

- offered / rejected 仅在对应邮件（offer / 感谢信）投递成功后落地；hired 与 rejected 是终态。

### 关键字段

- `Invitation.token`：接受邀请的用户端凭据（唯一索引），不出现在管理端列表
- `Invitation.email_sent_at`：空 = 邮件从未投递成功，业务上不算「已邀请」，允许重试投递
- 同一申请至多一条 pending 邀请（部分唯一索引）；同一申请至多一名候选人（唯一索引）——并发接受的数据库兜底
- 候选人时间线（`recruitment_candidate_timeline`）：管理端与用户端读同一张表，两端状态一致
- 报名申请（`recruitment_applications`）由 #88 拥有：applied → reviewing → accepted | rejected；BE-04 接受邀请后把它推进为 accepted

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


## 瑞士信号 2D 用户界面（Issue #81）

认证与普通用户首页/个人设置共用局部视觉外壳，不覆盖后台样式。首页身份与资料读取 GET `/api/v1/me`；方向、申请与报名体验仍为明确标注的 fixture。资料编辑复用现有 GET/PUT `/me` 白名单契约。动画按 Vue 生命周期清理，支持暂停、减少动态效果和后台暂停；认证网络请求有 15 秒超时兜底。登录转场两侧蓝块从下方、中间黄块从上方同时拼合，文字跟随黄块；入场 450ms、退场 650ms，路由与接口等待另计。动画终点写入元素样式，避免结束或中断后回闪，外壳卸载后停止转场。实现基准、业务差异与验收材料见 [Issue #81 审查说明](reviews/issue-81-swiss-signal.md)。
