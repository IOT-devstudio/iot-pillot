# 工作室运营功能 · 检票报告

日期：2026-10-27  
范围：**除招新外的工作室功能**（成员名录 / 项目管理 / 知识库；例会简化不做签到系统）

## 1. 已拍板决策

| 项 | 决定 |
| --- | --- |
| 模块 | 成员名录 + 项目管理 + 内部知识库 |
| 例会 | 本轮不做完整签到；飞书外链/备注即可 |
| 权限 | **仅 admin 可见可改**（member/游客不进管理页） |
| 成员来源 | 独立 `studio_members`；`hired` **手动**「转为成员」 |
| 项目关系 | 多对多 `studio_project_members`，角色 lead/contributor |
| 知识库 | 当前正文 + revisions 版本快照；无自动回滚 |
| 交付深度 | 先检票（术语/ADR/迁移/DTO 草案）；实现另开票 |

## 2. 仓库事实核对

- 已有迁移至 `0012_user_freeze`；工作室表编号为 **0013–0015**。
- 无既有 studio domain / handler / 前端模块。
- `recruitment_candidates.hired` 终态存在，无转成员动作。
- 关于页 `STUDIO_MEMBERS` / `STUDIO_WORKS` 仍为空 fixture（#94/#181 口径）；本轮 **不** 自动改公开页。

## 3. 产物清单

| 路径 | 说明 |
| --- | --- |
| [GLOSSARY.md](../../../GLOSSARY.md) | 工作室运营域术语 |
| [docs/adr/0001-studio-member-project-model.md](../../adr/0001-studio-member-project-model.md) | 硬约束 ADR |
| `apps/api/migrations/0013_studio_members.{up,down}.sql` | 成员表 |
| `apps/api/migrations/0014_studio_projects.{up,down}.sql` | 项目 + 参与 |
| `apps/api/migrations/0015_studio_knowledge.{up,down}.sql` | 知识库 + 修订 |
| `packages/shared-types/src/studio.ts` | DTO 草案（number ID） |

## 4. API 契约草案（admin only）

统一响应仍为 `{ code, message, data }`。下列路径全部 `authRequired + requireAdmin`。

### 4.1 成员

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/admin/studio/members` | 分页列表；query: `q`, `direction`, `is_active`, `page`, `page_size` |
| POST | `/api/v1/admin/studio/members` | 创建 |
| GET | `/api/v1/admin/studio/members/:id` | 详情 |
| PUT | `/api/v1/admin/studio/members/:id` | 更新 |
| POST | `/api/v1/admin/studio/members/:id/archive` | 归档（写 `archived_at`, `is_active=false`） |
| POST | `/api/v1/admin/studio/members/from-candidate` | body: `{ candidate_id, role_in_studio?, cohort? }`；要求 candidate.status=`hired` 且未转入 |

### 4.2 项目

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/admin/studio/projects` | 列表；query: `status`, `year`, `q` |
| POST | `/api/v1/admin/studio/projects` | 创建 |
| GET | `/api/v1/admin/studio/projects/:id` | 详情含 members[] |
| PUT | `/api/v1/admin/studio/projects/:id` | 更新项目元数据 |
| POST | `/api/v1/admin/studio/projects/:id/members` | 添加参与 |
| PUT | `/api/v1/admin/studio/projects/:id/members/:pm_id` | 改角色/时间 |
| DELETE | `/api/v1/admin/studio/projects/:id/members/:pm_id` | 移除参与行 |

### 4.3 知识库

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/admin/studio/knowledge` | 列表；query: `q`, `permission`, `parent_id` |
| POST | `/api/v1/admin/studio/knowledge` | 创建（同时写 revision v1） |
| GET | `/api/v1/admin/studio/knowledge/:id` | 当前正文 |
| PUT | `/api/v1/admin/studio/knowledge/:id` | 更新正文 → version+1 + 新 revision |
| GET | `/api/v1/admin/studio/knowledge/:id/revisions` | 修订列表 |
| GET | `/api/v1/admin/studio/knowledge/:id/revisions/:version` | 某版快照 |
| DELETE | `/api/v1/admin/studio/knowledge/:id` | 删条目（级联修订） |

## 5. 字段口径（实现时校验）

**StudioMember**

- `name`：必填，建议 1–40 字符（可含中英文；与注册「2–20 全中文」**解耦**，因校友可能用英文名）
- `birth_year`：必填，1900–2100 整数
- `cohort`：必填，如 `2024 级`
- `direction`：必填 text（可与招新 direction code 对齐）
- `role_in_studio`：必填（可空串禁止，至少 1 字）
- `internship` / `employment`：可选
- `user_id`：可选；若填须存在且未注销

**StudioProject**

- `name`：必填唯一
- `year`：必填展示字符串
- `summary`：必填，建议 ≤ 2000 字
- `status`：enum

**ProjectMember**

- `name`：必填快照
- `role`：`lead` | `contributor`
- `member_id`：可选
- `started_at` 必填；`ended_at` 可选且 ≥ started_at

**KnowledgeEntry**

- `title` / `body` 必填
- `permission` 默认 `admin`
- `parent_id` 可选树
- 每次 PUT 递增 `version` 并插入 revision

## 6. 前端导航草案（实现票）

管理端 `AdminLayout`「管理」分组下增加：

- 成员名录 → `/studio/members`
- 项目管理 → `/studio/projects`
- 知识库 → `/studio/knowledge`

`meta.allowRoles: ['admin']`，`adminShell: true`。

## 7. 明确不做（本轮）

- 例会签到 / 缺席邮件
- member 只读工作室 API
- 游客公开成员/项目 API
- 关于页自动接库
- 项目封面图 / 文件上传
- casbin 细粒度 ACL

## 8. 建议后续票据（to-tickets）

1. **BE-Studio-01** 迁移 0013–0015 + domain/repo（可先于 handler 合入）
2. **BE-Studio-02** members CRUD + from-candidate
3. **BE-Studio-03** projects + participation
4. **BE-Studio-04** knowledge + revisions
5. **FE-Studio-01** 导航 + 成员页
6. **FE-Studio-02** 项目页
7. **FE-Studio-03** 知识库页
8. （可选）公开只读 / 关于页接数

阻塞边：02–04 依赖 01；FE 各自依赖对应 BE。

## 9. 风险

- `birth_year` 唯一键：同人改名会成两条；可接受，管理端人工合并。
- 项目名全局唯一可能过严；若实战冲突改为 `(name, year)` 唯一。
- 知识库 `author_id` 无 FK：用户注销后仍显示历史作者 id，列表需容错缺名。
- 迁移已 embed：`go:embed *.sql` 会自动带上，**勿在未实现 handler 前对生产 up**，除非接受空表。

## 10. 检票结论

**可以进入 `/to-spec` 或 `/to-tickets`。**  
本报告 + ADR + 迁移 SQL + `studio.ts` 构成实现基线；**尚未写 handler/页面，不算功能完成。**
