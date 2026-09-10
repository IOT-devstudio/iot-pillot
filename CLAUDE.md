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
| 数据库        | **PostgreSQL**                        | `docker-compose.yml` 启动本地实例                                 |
| Monorepo 工具 | **pnpm workspaces**                   | 前后端统一管理                                                   |
| CI/CD         | **GitHub Actions**                    | `auto-merge.yml`（bot）+ `ci.yml`（PR 验证）+ `deploy.yml`（push main → VPS） |

### 已规划但未引入（决策已定，依赖业务模块开工时安装）

| 组件       | 建议                                | 备选             | 引入时机                  |
| ---------- | ----------------------------------- | ---------------- | ------------------------- |
| ORM        | GORM                                | sqlc             | FormModule / RecruitmentModule 开工 |
| SMTP 库    | go-mail                             | gomail           | MailModule 开工           |
| 模板渲染   | `html/template` + Handlebars 子集 | —               | TemplateModule 开工       |
| RBAC       | casbin                              | JWT + 中间件手写 | AuthModule 开工（鉴权方案定后） |
| 数据库迁移 | golang-migrate                      | goose            | schema 第一次演进         |

### 待确认

| 层       | 候选                               | 倾向 |
| -------- | ---------------------------------- | ---- |
| 鉴权方案 | 本地邮箱密码 + JWT（推荐） / OAuth | 未决 |
| 国际化   | 中文为主 / 中英双语                | 未决 |

---

## 目录结构

```
iot-pillot/
├── apps/
│   ├── web/                # 前端（Vue 3 + Element Plus + TS）
│   └── api/                # 后端 Go 服务（Gin + GORM），go.mod 在此
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
├── docker-compose.yml      # 本地 Postgres
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

## 后续未决项（开发前需明确）

- [ ] 部署目标平台（VPS / Railway / Fly.io）
- [ ] 鉴权方案（本地邮箱密码 + JWT / OAuth）
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
