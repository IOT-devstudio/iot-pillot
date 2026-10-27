# Context

当前项目上下文与工作记忆（single-context 布局的核心上下文文件）。与 `GLOSSARY.md`（术语）和 `docs/adr/`（架构决策记录）互补：本文件记录"正在做什么、近期决定、下一步"，不重复定义术语，也不记录已固化的决策（后者进 ADR）。

## 当前阶段

- iot-pillot：IoT 全栈开发工作室的后台管理 web 程序；招新主线已大部分落地。
- **工作室运营域（除招新外）**：2026-10-27 完成检票（术语 + ADR + 迁移 0013–0015 + shared-types 草案）。**尚未实现 handler/页面。**
- 开放 issue：#91 前端接入邀请接受与候选人处理。

## 工作室运营检票结论（2026-10-27）

- 做：成员名录、项目管理（多对多参与）、内部知识库（版本修订）。
- 不做（本轮）：例会签到系统；member/游客可见工作室管理 API；关于页自动接库。
- 权限：仅 admin 读写。
- 成员：独立 `studio_members`；`hired` 手动转成员。
- 详文：`docs/superpowers/specs/2026-10-27-studio-ops-check-report.md`、`docs/adr/0001-studio-member-project-model.md`。

## 未决项

详见 `CLAUDE.md` 后续未决项：部署目标平台、邮件模板渲染选型、国际化范围、Postgres 备份策略。

## 下一步

- `/to-tickets` 或建 GitHub issues：BE-Studio-01…04 + FE-Studio-01…03（见检票报告 §8）。
- 实现前不要对生产执行 0013–0015 up（空表可接受则本地可 up）。
- #91 与工作室实现可并行，无 schema 冲突。
