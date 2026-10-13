# Context

当前项目上下文与工作记忆（single-context 布局的核心上下文文件）。与 `GLOSSARY.md`（术语）和 `docs/adr/`（架构决策记录）互补：本文件记录"正在做什么、近期决定、下一步"，不重复定义术语，也不记录已固化的决策（后者进 ADR）。

## 当前阶段

- iot-pillot：IoT 全栈开发工作室的后台管理 web 程序，当前阶段为工作室招新管理。
- 认证与角色分流基础交付已完成：`/login`、`/register`、刷新、退出、`/me`、管理员用户列表已对接；`admin`/`member` RBAC 与前端路由守卫已联调。
- 业务模块（表单 / 模板 / 招新 / 用户权限 / SMTP 设置）后端路由尚未就绪，前端以与最终 DTO 一致的 fixture 驱动，未伪装真实联调。

## 本次初始化（2026-09-20）

- 记录 issue tracker（GitHub Issues，PRs 不作 triage 面）、triage 标签、domain docs 结论（single-context）。
- 已建 `docs/agents/{issue-tracker,triage-labels,domain}.md`，并在 `CLAUDE.md` 追加 `## Agent skills` 块。
- 已建根 `GLOSSARY.md`（术语表）与 `CONTEXT.md`（本文件）。

## 未决项

详见 `CLAUDE.md` 后续未决项：部署目标平台、邮件模板渲染选型、数据库迁移工具、国际化范围、Postgres 备份策略。

## 下一步

- 首个真实领域术语或架构决策产生时，用 `/domain-modeling` 补充 `GLOSSARY.md` 或新增 `docs/adr/NNNN-*.md`。
- 标签与 issue 约定见 `docs/agents/triage-labels.md` 与 `docs/agents/issue-tracker.md`。
