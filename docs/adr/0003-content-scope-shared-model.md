# ADR 0003：招新问答与内部论坛共用一套内容模型（内容域 scope）

## 状态

已确认（2026-10-30）。来自地图 #207「招新问答与学习路线」的决策票 #208「[问答] 内容模型与权限矩阵定稿」。

**未落地**：本文只记录内容模型与权限矩阵的决策，建表见后续迁移票。

## 上下文

用户需求原话是「提问（可以发图片）（这个和工作室人员内部论坛放一起用权限隔开，两个大体逻辑相同）」。也就是说：两块功能在交互上是同构的（标题 + 正文 + 回复 + 图片），区别只在**谁能看、谁能写**。

调研确认的事实：

- 全仓「论坛 / 提问 / 问答 / post / topic / comment / attachment / upload」**0 命中**，`packages/shared-types` 无任何草案 —— 本域从零开始。
- 仓内**没有「工作室人员」身份**：`member` 是任何注册用户（`apps/api/internal/service/auth_usec.go:141`），`MEMBER_ONLY` 实际等于 `["admin"]`（`apps/web/src/router/route-specs.ts:9`）。「工作室成员」是独立表 `studio_members`，`user_id` 可空。
- 最接近的既有模型是知识库（`apps/api/migrations/0015_studio_knowledge.up.sql`）：它是**单向的、带版本修订的文档**，没有回复、没有作者-读者互动，不能复用。

本票的取舍全部围绕一个判断：**把「两块功能」做成两个模块，还是做成一个模型的两个分区**。

## 决策

### 1. 一套内容模型，用 `scope` 分区

`forum_posts.scope` 取值 `recruitment` | `studio`，带 `CHECK` 约束。回复不单独存 scope，经 `post_id` 继承。

`scope` 是**分区维度**，不是权限等级 —— 它与知识库的 `permission`（`admin`/`member`/`public`）不是同一个概念，两者不可互相套用。

### 2. 帖子与回复是两张表，不是一张自引用表

`forum_posts`（`title NOT NULL`、`scope NOT NULL`、`resolved_at`、`resolved_by`）与 `forum_replies`（`post_id NOT NULL`，**没有** `title` / `scope` / `resolved_*`）。

一张自引用表会让 `title` / `scope` / `resolved_at` / `resolved_by` 四列在回复行上必须可空，且所有「只看主帖」的查询都要带 `parent_id IS NULL`——漏写一次就把回复混进列表。两张表让每一列都能 `NOT NULL` 在该在的地方。

**代价**：将来要做「楼中楼」得给 `forum_replies` 加一列 `parent_reply_id`。本期已明确不做嵌套回复，这个代价换得的是当下的模型清晰。

### 3. 权限的粗粒度真源是路由分组，细粒度在 service

| 粒度 | 真源 | 内容 |
| --- | --- | --- |
| 粗：哪个 scope 要什么角色 | `apps/api/internal/handler/router.go` 的路由分组 | `recruitment` 挂 `apiV1.Group("/forum", authRequired)`；`studio` 挂 `admin.Group("/studio/forum")` |
| 中：handler 怎么知道自己是哪个 scope | `middleware.WithScope(scope)` 写入 gin context | handler 从 context 读，**永不解析 URL** |
| 细：谁能改/删/标记这一条 | `internal/service` | 只有作者本人或管理员；中间件看不到具体资源 |

两条并存不矛盾：中间件管「这个 scope 要什么角色」，service 管「这条记录归不归你」。`CLAUDE.md` 的「权限判断统一走 `AuthModule` 的守卫/中间件，不在 handler 里 if-else 角色」说的是前者。

**将来开放 `studio` 给普通成员**：把那一组从 `admin.Group` 挪到 `apiV1.Group`，**一处改动**。这是选 A 而非「路径带 `:scope` + service 权限矩阵」的核心理由。

### 4. 状态列成对出现，都不加原因

| 语义 | 列 | 规则 |
| --- | --- | --- |
| 已解决 | `resolved_at` + `resolved_by` | 有值即已解决；撤销 = 置 NULL；两 scope 通用，后端不按 scope 拒绝，UI 只在招新问答暴露 |
| 软删除 | `deleted_at` + `deleted_by` | 作者可删自己的，管理员可删任何人的；不加 `deleted_reason` |
| 已编辑 | `edited_at` | 作者编辑后置位，界面显示「已编辑」；编辑**不影响** `resolved_*` |

`resolved_*` 与 `deleted_*` 正交：软删的帖子保留它原来的已解决状态，只是不再出现在列表里。

不设 `status` 枚举列：它丢失「何时/谁」，且与 `deleted_at` 两个状态列并存时容易产生非法组合。不加 `deleted_reason`：本期没有申诉或复核流程会消费它。

### 5. 附件独立表 `content_attachments`，允许孤儿，24 小时清理

```
content_attachments(
  id, object_key, uploader_id, content_type, size_bytes,
  post_id, reply_id, roadmap_id,   -- 三列全可空
  created_at
)
CHECK (num_nonnulls(post_id, reply_id, roadmap_id) <= 1)
```

PostgreSQL 16 内置 `num_nonnulls`，不需要写三个 `IS NULL` 的组合。

- 上传时三列全空 = **孤儿态**，这是支撑「边写边传」的前提。
- 发帖 / 回复 / 建路线时绑定，**只允许绑定自己上传的、尚未归属的附件**（防止把别人的图挂到自己帖子下）。
- 内容软删时**附件行与 MinIO 对象都保留**——软删可逆的前提就是对象还在。
- 定时清理任务只看附件自身三列：`post_id / reply_id / roadmap_id` 全空且 `created_at < now() - interval '24 hours'` → 删行 + 删对象。**不 JOIN 任何内容表**，因此不会因内容表结构变化而失效，也不会误删软删帖子的图。

表名不用 `forum_attachments`：它也服务学习路线（见决策 6 的反面——路线**内容**不用 scope，但**附件**共用这张表）。

### 6. 学习路线不带 `scope`

`learning_roadmaps` 只挂 `direction_code`（与 `recruitment_directions.code` 对齐）。

`scope` 是「帖子分区」的概念。路线不是帖子：它没有作者-读者双向互动，没有回复，没有已解决。为了复用一张附件表而把 `scope` 扩散到第三张表，只会让一个本期永远取单值的列被误读成权限。

## 拒绝的方案

| 方案 | 原因 |
| --- | --- |
| 两个独立模块（`forum` + `qa`） | 交互同构，「列表 / 详情 / 回复 / 附件 / 软删 / 已解决」整套逻辑复制两份 |
| 一张自引用表 `parent_id` | `title` / `scope` / `resolved_*` 在回复行必须可空；所有主帖查询都要带过滤条件 |
| 路径带 `:scope` + service 层权限矩阵 | 把角色判定从中间件挪进 service，违反 `CLAUDE.md` 工作约定 |
| handler 内 `if-else` 角色 | `CLAUDE.md` 明确禁止；新增 scope 要改每个 handler |
| `status` 枚举列代替 `resolved_at` | 丢失「何时/谁」；与 `deleted_at` 并存易产生非法状态组合 |
| 独立事件表记录状态变更 | 本期只有两态，属于过度设计 |
| 加 `deleted_reason` 列 | 本期没有申诉/复核流程消费它，管理员每次删帖多一步输入 |
| 正文内嵌对象名（`![](object:xxx)`） | 无法枚举帖子附件、无法安全清理对象；正文解析失败则图片永久泄漏在 bucket |
| 附件表禁止孤儿（先建帖再传图） | 与「边写边传」直接冲突，用户必须先发一个空帖 |
| 学习路线独立附件表 | 「上传 / 绑定 / 孤儿清理 / 代理转发」整套逻辑复制一份 |
| 学习路线带 `scope` | 本期永远单值，且会被误读成权限 |
| 用「知识库条目」承载问答 | 知识库是单向文档 + 版本修订，没有回复与作者-读者互动 |

## 后果

**新增 4 张表**：`forum_posts`、`forum_replies`、`content_attachments`、`learning_roadmaps`（含索引）。迁移号接 `0016` 起。

**新增 1 个中间件**：`middleware.WithScope` 与对应的 context key，风格对齐现有 `ContextUserID` / `ContextRole`。

**新增 1 个后台任务**：24 小时孤儿附件清理，进程内 ticker 模式参考 `0012` 的 `FreezeScheduler`（放在 `Router` 里随 HTTP 服务同生共死）。

**术语**：`GLOSSARY.md` 新增「内容域」「帖子」「回复」「附件」「招新问答」「内部论坛」「学习路线」7 个词。

**可逆性边界**：开放 `studio` 给普通成员只需改 `router.go` 一处；但「帖子分区」这个概念已经扩散到路由分组、`WithScope` 中间件与 `content_attachments` 的三列。**若将来要三个以上 scope，需要重新审视分组方式**（分组会退化成 `if` 分支，届时该换成一个 scope → 角色要求的表驱动配置）。

## 参考

- 地图：#207「[探索地图] 招新问答与学习路线」；本票：#208
- 图片存储：`docs/adr/0002-image-storage-minio.md`、`docs/research/minio-image-storage.md`
- 仓库约束：`CLAUDE.md`「工作约定」、`GLOSSARY.md`
- 既有迁移范式：`apps/api/migrations/0013_studio_members.up.sql`、`0015_studio_knowledge.up.sql`
