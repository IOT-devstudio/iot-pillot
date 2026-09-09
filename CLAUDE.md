# iot-pillot

## 项目身份

- **名称**: iot-pillot
- **组织**: [IOT-devstudio](https://github.com/IOT-devstudio)
- **定位**: IoT 全栈开发工作室的后台管理 web 程序
- **传承使命**: 代代传承——为长期维护与接手友好而设计
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

### 已确认（2026-09-09）

| 层            | 选型                      | 说明                                                             |
| ------------- | ------------------------- | ---------------------------------------------------------------- |
| 前端框架      | **Vue 3 + Element Plus + TypeScript** | 会 Vue 的开发者更多，接手门槛低，契合"代代传承"           |
| 后端语言      | **Go**              | 招新规模适配，单二进制部署                                       |
| 数据库        | **PostgreSQL**      | 与 GORM 配合稳定                                                 |
| CI/CD         | **GitHub Actions**  | `ci.yml`（PR 验证） + `release.yml`（打 tag 触发构建与部署） |
| 镜像仓库      | **ghcr.io**         | 私有仓库免费                                                     |
| Monorepo 工具 | **pnpm workspaces** | 前后端统一管理                                                   |
| 部署产物      | Docker 镜像，单实例       | VPS / Railway / Fly.io 任选                                      |

### 已建议（待最终落地时敲定）

| 组件       | 建议                                | 备选             |
| ---------- | ----------------------------------- | ---------------- |
| Web 框架   | Gin                                 | Echo / Fiber     |
| ORM        | GORM                                | sqlc             |
| SMTP 库    | go-mail                             | gomail           |
| 模板渲染   | `html/template` + Handlebars 子集 | —               |
| RBAC       | casbin                              | JWT + 中间件手写 |
| 数据库迁移 | golang-migrate                      | goose            |
| 配置管理   | viper                               | envconfig        |

### 待确认

| 层       | 候选                                              | 倾向                                                          |
| -------- | ------------------------------------------------- | ------------------------------------------------------------- |
| 部署平台 | VPS / Railway / Fly.io                            | 未决                                                          |
| 鉴权方案 | 本地邮箱密码 + JWT（推荐） / OAuth                | 未决                                                          |
| 国际化   | 中文为主 / 中英双语                               | 未决                                                          |

---

## 目录结构（拟）

```
iot-pillot/
├── apps/
│   ├── web/                # 前端（Vue 3 + Element Plus + TS）
│   └── api/                # 后端 Go 服务（Gin + GORM）
├── packages/
│   └── shared-types/       # 前后端共享 TS 类型（DTO）
├── .github/
│   └── workflows/          # ci.yml + release.yml
├── docs/                   # 项目文档
├── docker-compose.yml      # 本地 Postgres + API
├── pnpm-workspace.yaml
├── go.mod / go.sum
└── CLAUDE.md
```

---

## 架构原则（代代传承约束）

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

- Go 后端代码默认落在 `apps/api/`
- 前端代码默认落在 `apps/web/`
- 跨前后端的类型定义落在 `packages/shared-types/`
- 数据库 schema 变更**必须**通过迁移文件管理，不直接改表
- 所有 SMTP 外发**统一**走 `MailModule`，业务代码不直接调用 SMTP 库
- 所有模板渲染**统一**走 `TemplateModule`，避免散落字符串拼接
- 所有权限判断**统一**走 `AuthModule` 的守卫/中间件，不在 handler 里 if-else 角色

---

## 决策日志

| 日期       | 决策                                       | 理由                                                                  |
| ---------- | ------------------------------------------ | --------------------------------------------------------------------- |
| 2026-09-09 | 后端选 Go（排除 NestJS、Express、FastAPI） | 招新规模适配，单二进制部署契合"代代传承"；开发/部署/CI 全链路 Go 更轻 |
| 2026-09-09 | 部署走 GitHub Actions + ghcr.io            | 与 GitHub 仓库同源，免费私有，PR 反馈快（Go 单测秒级）                |
| 2026-09-09 | 前端定 Vue 3 + Element Plus + TS（从 React+Antd 改定） | 会 Vue 的开发者更多，团队接手门槛低，契合"代代传承"；Element Plus 同样覆盖管理后台表格/表单场景 |
