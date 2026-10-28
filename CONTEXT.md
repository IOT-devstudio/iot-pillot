# Context

当前项目上下文与工作记忆（single-context 布局的核心上下文文件）。与 `GLOSSARY.md`（术语）和 `docs/adr/`（架构决策记录）互补：本文件记录"正在做什么、近期决定、下一步"，不重复定义术语，也不记录已固化的决策（后者进 ADR）。

## 当前阶段

- iot-pillot：IoT 全栈开发工作室的后台管理 web 程序；招新主线已大部分落地。
- **工作室运营域（除招新外）**：2026-10-27 完成检票（术语 + ADR + 迁移 0013–0015 + shared-types 草案）；同日 **#195 成员名录**、**#196 知识库与修订历史**均已实现（admin API + 管理端页面）。项目（#198）仍未实现。
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
- 迁移 0013 已纳管：`go run ./tools/migrate up` 首次会把 schema 版本推到 15（0014/0015 的表同批建出）。
- `user_id` 可空 = 无系统账号的校友；`(name, birth_year)` 唯一 → 同名同出生年返回 400。
- 归档不物理删除（`archived_at` + `is_active=false`）；`PUT {is_active:true}` 恢复在读并清归档时刻。
- 更新是补丁语义；`user_id` 三态（键不出现 = 不改 / null = 解绑 / 数字 = 绑定），
  用 `request.OptionalInt` 实现——`**int` 会把 null 与「没传」解成同一个 nil。
- 前端：`/studio/members`（仅 admin，AdminLayout「工作室」分组），api 在 `apps/web/src/api/studio-members.ts`。

## #196 知识库交付口径（2026-10-28）

- 后端：`domain.StudioKnowledgeEntry`/`StudioKnowledgeRevision` + `repository.StudioKnowledgeRepo` +
  `service.StudioKnowledgeUseCase` + `handler.StudioKnowledgeHandler`。
- 端点（全部仅 admin）：
  `GET/POST /api/v1/admin/studio/knowledge`、`GET/PUT/DELETE /:id`、
  `GET /:id/revisions`、`GET /:id/revisions/:version`。
- **保存 = 升版**：每次 PUT 把 `version + 1` 并在同一事务里写入同版本快照；
  创建本身也写 v1（否则历史缺第一版）。
- **无自动回滚**（ADR 0001 §3）：恢复 = 从历史「带回编辑器」读回旧版内容再保存一版，
  历史因此始终是追加式的。前端历史弹窗只有「查看内容 / 带回编辑器」，没有回滚按钮。
- **乐观锁**：`version` 必填（缺省报 400，不默默回退到当前版本——那会让乐观锁退化成「总是覆盖」）。
  条件更新 `WHERE id = ? AND version = ?`，并发保存返回「已被其他人修改，请刷新后重试」。
- `is_public` 是 `permission` 的**派生值**（`permission === "public"`），不接收请求入参。
- `parent_id` 三态（键不出现 = 不改 / null = 移到顶层 / 数字 = 挂到该条目下），用 `request.OptionalInt`；
  环检测走递归 CTE（`IsDescendant`，深度上限 100 防脏数据死循环）。
- 删除条目级联删修订（`ON DELETE CASCADE`）；子条目的 `parent_id` 置空，**不**连带删子树。
- 前端：`/studio/knowledge`（仅 admin），api 在 `apps/web/src/api/studio-knowledge.ts`。

## 下一步

- 工作室运营剩余实现票：
  - Frontier：**#198** 项目与参与（#195/#196 已完成）
  - 阻塞于 #195 且仍未做：**#197** hired 转成员
  - 阻塞于 195+196+198：**#199** 文档回填
- #91 与工作室实现可并行，无 schema 冲突。
