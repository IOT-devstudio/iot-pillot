# ADR 0001：工作室成员、项目与知识库数据模型

## 状态

已确认（2026-10-26/27）。来自工作室功能访谈；与 `GLOSSARY.md`「工作室运营域」一致。

## 上下文

iot-pillot 当前以招新为主：方向、报名、邀请、候选人、表单、邮件、用户权限。`candidates.hired` 是招新终态，但：

- 已毕业/无账号人员无法进入系统名录；
- 项目成果需要多人参与展示；
- 内部知识需要版本与权限，而不是公开静态文案。

权限拍板：**本轮工作室运营能力仅 admin 可见可改**（member / 游客不进管理页）。关于页公开内容仍可继续用静态 fixture；是否以后读工作室 API 另议。

## 决策

### 1. 独立 `studio_members`（不复用 candidates）

- `user_id` **可空**：允许无账号外部/校友成员。
- `hired → 成员` 只提供管理端**手动**「转为成员」，不自动插入。
- 毕业/退出用 `archived_at` + `is_active`，不物理删除，以保住项目参与历史。
- 唯一性：`(name, birth_year)`（同名不同届可共存）。

### 2. 项目 + 多对多参与

- `studio_projects` + `studio_project_members`。
- 参与角色：`lead` | `contributor`。
- `studio_project_members.name` **姓名快照**必填；`member_id` 可空（外部参与者）。
- 项目状态：`active` | `completed` | `archived`。

### 3. 知识库（内部，带版本）

- `studio_knowledge_entries` 存当前正文与权限。
- `studio_knowledge_revisions` 存每次修订快照（`version` 递增）。
- 不做自动回滚 UI：恢复 = 管理员基于旧修订再写一版。
- 首期 `permission`：`admin` | `member` | `public` 字段保留，**路由仅挂 admin**；member/public 读取后续再开。

### 4. 例会

- **本轮不做** Sessions/Attendance 系统。需要时用飞书外链或知识库备注。

### 5. 迁移编号

仓库已有 `0011_user_soft_delete`、`0012_user_freeze`。工作室表从：

- `0013_studio_members`
- `0014_studio_projects`
- `0015_studio_knowledge`

起。

### 6. API 前缀（草案）

全部挂在 `admin` 组：

- `/api/v1/admin/studio/members*`
- `/api/v1/admin/studio/projects*`
- `/api/v1/admin/studio/knowledge*`
- `POST /api/v1/admin/studio/members/from-candidate`（hired 转成员）

ID 继续用 number（与现有后端一致），不引入 UUID。

## 拒绝的方案

| 方案 | 原因 |
| --- | --- |
| `hired` 即成员 | 无法收录无账号校友；混淆招新与名录 |
| 项目只挂一人 | 不满足宣讲多人成果 |
| 完整例会签到 | 过重；本轮无硬需求 |
| 知识库自动回滚 | 复杂度高；手动再发版即可 |

## 后果

- 新增 Studio 域模块（后端 + 共享类型 + 管理端页面），不改招新状态机。
- 关于页成员/成果若要接真数据，需另开「公开只读」决策（与本 ADR 的 admin-only 不冲突，可后置）。

## 参考

- `GLOSSARY.md` 工作室运营域
- `apps/api/migrations/0013_*.sql` 起
- 检票草案：`docs/superpowers/specs/2026-10-27-studio-ops-check-report.md`
