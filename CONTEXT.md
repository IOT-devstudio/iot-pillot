# Context

当前项目上下文与工作记忆（single-context 布局的核心上下文文件）。与 `GLOSSARY.md`（术语）和 `docs/adr/`（架构决策记录）互补：本文件记录"正在做什么、近期决定、下一步"，不重复定义术语，也不记录已固化的决策（后者进 ADR）。

## 当前阶段

- iot-pillot：IoT 全栈开发工作室的后台管理 web 程序；招新主线已大部分落地。
- **工作室运营域（除招新外）**：2026-10-27 完成检票（术语 + ADR + 迁移 0013–0015 + shared-types 草案）；同日 **#195 成员名录闭环已实现**（迁移 0013 + admin API + 管理端页面）。项目（#198）与知识库（#196）仍未实现。
- 开放 issue：#91 前端接入邀请接受与候选人处理。

## 工作室运营检票结论（2026-10-27）

- 做：成员名录、项目管理（多对多参与）、内部知识库（版本修订）。
- 不做（本轮）：例会签到系统；member/游客可见工作室管理 API；关于页自动接库。
- 权限：仅 admin 读写。
- 成员：独立 `studio_members`；`hired` 手动转成员。
- 详文：`docs/superpowers/specs/2026-10-27-studio-ops-check-report.md`、`docs/adr/0001-studio-member-project-model.md`。

## 未决项

详见 `CLAUDE.md` 后续未决项：部署目标平台、邮件模板渲染选型、国际化范围、Postgres 备份策略。

## #195 成员名录交付口径（2026-10-27）

- 后端：`domain.StudioMember` + `repository.StudioMemberRepo` + `service.StudioMemberUseCase` + `handler.StudioMemberHandler`；
  端点 `/api/v1/admin/studio/members`（GET/POST）、`/:id`（GET/PUT）、`/:id/archive`（POST）。
- 迁移 0013 已纳管：`go run ./tools/migrate up` 首次会把 schema 版本推到 15（0014/0015 的表同批建出，但**没有 handler/页面**）。
- `user_id` 可空 = 无系统账号的校友；`(name, birth_year)` 唯一 → 同名同出生年返回 400。
- 归档不物理删除（`archived_at` + `is_active=false`）；`PUT {is_active:true}` 恢复在读并清归档时刻。
- 更新是补丁语义；`user_id` 三态（键不出现 = 不改 / null = 解绑 / 数字 = 绑定），
  用 `request.OptionalInt` 实现——`**int` 会把 null 与「没传」解成同一个 nil。
- 前端：`/studio/members`（仅 admin，AdminLayout「工作室」分组），api 在 `apps/web/src/api/studio-members.ts`。

## 下一步

- 工作室运营剩余实现票：
  - Frontier：**#196** 知识库、**#198** 项目与参与（#195 已完成）
  - 阻塞于 #195 且仍未做：**#197** hired 转成员
  - 阻塞于 195+196+198：**#199** 文档回填
- #91 与工作室实现可并行，无 schema 冲突。
