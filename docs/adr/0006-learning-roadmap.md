# ADR 0006：学习路线是单表 + JSONB 阶段，带稳定阶段 id 与个人进度

## 状态

已确认（2026-10-30）。来自地图 #207「[探索地图] 招新问答与学习路线」的决策票 #211「[路线] 学习路线数据模型与页面形态定稿」。

**未落地**：本文只记录数据模型与页面形态的决策，建表见后续迁移票，页面见实施票。

## 上下文

需求里「学习路线推荐」是独立于问答的第二块能力。ADR 0003 已经定了它的**定位**（不带 `scope`，只挂 `direction_code`，附件共用 `content_attachments` 的 `roadmap_id` 列），但没定它的形状。

调研确认的事实：

- `recruitment_directions` 是**表**不是枚举：`code` 唯一、`name` 可空、`open` 控制是否可报名（`apps/api/migrations/0007_recruitment_tables.up.sql:36-47`）。而 `users.detail_direction` 的枚举还多出 `all` / `game` / `other`（`apps/api/internal/domain/user.go` 的 `Direction` 常量）。
- 公开的 `GET /api/v1/recruitment/directions` **只返回 `open = true` 的方向**（`apps/api/internal/service/recruitment_usec.go:100-113`，注释写明「只返回当前开放的方向」）。招新结束后方向全关，它会是空列表。
- 知识库（#196）是仓内唯一的内容编辑范式：单页编辑、`version` 乐观锁、每次保存写修订快照（`apps/api/internal/service/studio_knowledge_usec.go:64,112,156-158`）。
- 前端路由清单集中在 `apps/web/src/router/route-specs.ts`：`MEMBER_ONLY = ["admin"]`、`ANY_SIGNED_IN = ["admin","member"]`、`PUBLIC = []`。侧边栏三个分组「工作台 / 工作室 / 管理」在 `apps/web/src/components/common/AdminLayout.vue:38-111`，**没有「招新」分组**。
- ADR 0005 已定 Markdown 渲染器在 `apps/web/src/components/markdown/`、链接协议白名单是 `http:` / `https:` / 站内 `/` 开头。

本票的取舍围绕一个问题：**「阶段 → 资源」这两层到底需不需要各自成为一张表**。

## 决策

### 1. 单表 + JSONB 存阶段与资源

```
learning_roadmaps(
  id, title, body, direction_code, stages jsonb,
  version integer NOT NULL DEFAULT 1,
  author_id, created_at, updated_at, deleted_at, deleted_by
)
CHECK (jsonb_typeof(stages) = 'array')
```

`stages` 是 `[{ id, title, summary, resources: [{ title, url }] }]`。

理由：阶段与资源**没有独立的查询需求**——没有任何页面会问「所有包含某资源链接的阶段」，也没有跨路线的资源复用。它们只作为一条路线的组成部分被整体读写。拆成三张表的代价是管理端保存要处理三层 diff、要防「阶段被删而资源还在」，换来的能力本期用不上。

**代价**：不能对单个资源做统计或加外键；将来真需要按资源聚合时要写一次数据迁移。这个代价与「路线是精选清单、数量级几十条」的定位匹配。

### 2. 方向是单值软引用，不加外键

`direction_code text NOT NULL`，存 `recruitment_directions.code` 的值，**不建 FK**。

不建 FK 的原因：`recruitment_directions` 的主键是 `id`，`code` 上只有唯一索引；且方向表是**招新运营数据**，而路线是**长期内容**——一次招新结束后方向可能被清理，路线不该跟着悬空失败。软引用让路线在方向消失后仍能展示（回落显示原始 code）。

**代价**：可能出现指向已删除方向的路线。这是**可见的**降级（列表里显示 `front-end` 而不是「前端」），不是静默错误。

不做多对多：一条路线服务多个方向听起来合理，但会让管理端多一个多选控件，而「通用基础」这类内容完全可以复制一条路线。ADR 0003 已经写了 `learning_roadmaps` 只挂 `direction_code`，本票不推翻它。

### 3. 每个阶段带服务端生成的稳定 id

阶段 id 形如 `s_` + 8 位 base36 随机串，由后端在保存时生成，**前端只当不透明字符串**。

这一条是为决策 4 服务的：用户的学习进度记的是阶段 id，不是序号。管理员把第 2 阶段拖到第 1 位时，用户的进度仍然指向原来那个阶段；用序号的话，重排会让进度**静默**指到别的阶段，且没有任何提示。

不用 `s_1` / `s_2` 这类顺序号：管理员删掉中间阶段后再新增会**复用**已删除的 id，进度就会指到新阶段上——这比序号错位更难发现。

保存时的对齐规则（Go service 层）：请求里已有 id 且存在于旧数据的 → 保留；没有 id 的 → 生成新 id；旧数据里有而请求里没有的 → 丢弃。**前端不需要也不允许自己生成 id。**

### 4. 进度是新表，一人一路线一行

```
user_roadmap_progress(
  user_id, roadmap_id, stage_id, updated_at
)
UNIQUE (user_id, roadmap_id)
```

「标记学到这」= UPSERT；再点同一个阶段 = 清除（DELETE 或把 `stage_id` 置 NULL 后删除行，两种等价）。

不塞进 `users` 表加一列 JSONB：`users` 已经承载认证、冻结、资料完整性三套语义，再塞学习进度会污染它，且查询要解析 JSONB。

**进度只属于本人**：接口只返回当前登录用户的进度，没有「查看某人进度」的端点。管理员也看不到——少一套权限判定，也少一个隐私面。

### 5. 路线只有软删，不做状态机

`deleted_at` + `deleted_by`，与内容域（ADR 0003）的治理口径一致。

不加 `published` 布尔：本期管理员写完就是要给用户看的，草稿态会让每个查询都多一个 `WHERE published`，也让「为什么用户看不到」变成一个需要排查的问题。真要草稿，编辑时先不保存即可。

不硬删：用户进度行指向已删除的路线，硬删会留下悬空引用；软删后若恢复，进度还在。

### 6. 管理端单页整体编辑，整体提交，version 乐观锁

沿用知识库范式（`studio_knowledge_usec.go:156-158`）：一个页面里改标题、正文、阶段列表，一次 `PUT` 全量提交，带 `version` 防并发覆盖；缺 `version` 返回明确的参数错误而不是假装成功。

阶段排序用**上下移动按钮**，前端改数组顺序后随整体 PUT 提交。不做拖拽：仓内无拖拽库，自写一套拖拽逻辑的复杂度远大于它带来的体验收益。

**不做版本历史**。知识库要修订快照是因为它是「团队文档，改错了要能追责」；路线是「推荐内容，改错了直接改回来」。少一张表、少两个接口、少一个历史面板。

### 7. 用户端两个路由，列表页按方向筛选

`/user/learning-paths`（卡片列表 + 方向筛选 + 分页）与 `/user/learning-paths/:id`（阶段与资源）。

两个路由都标 `ANY_SIGNED_IN`。游客从首页点进来直接被守卫拦去登录页——与 `/user/settings`、`/user/invitations` 完全一致，**不新增任何公开路由与未登录分支**。

列表卡片显示：标题 + 方向名 + 阶段数 + 正文摘要（ADR 0005 的 `markdownExcerpt`）。不显示阶段标题——那要求列表接口把整个 `stages` 列读出来，与「列表页不渲染正文」的取向相反。

两种空状态分开：无路线 → 「暂无学习路线」；筛选后为空 → 「该方向暂无路线」。合并成一个会让用户以为整站没内容。

### 8. 方向筛选项由路线接口自带，不查招新方向接口

路线列表响应里一并返回 `directions: [{ code, name, count }]`，**只含有路线的方向**，不看 `open`。

这一条是本票最容易踩的坑：公开的招新方向接口只返回 `open = true` 的方向（`recruitment_usec.go:100-113`）。招新结束后方向全关，若拿它当筛选项，**路线页会一个方向都不剩**。而路线的生命周期跨越多轮招新，它不该被招新的开关状态绑架。

方向名取 `recruitment_directions.name`，为空时回落 `code`（`0007` 里 `name` 是可空的）。用 LEFT JOIN，方向不存在也能出结果。

### 9. 分页，不做关键词搜索

`page` / `page_size`，上限 100，与仓内既有分页范式一致（`admin_handle.go:67`、`form_handle.go:104`、`mail_handle.go:227` 的 `parseBoundedInt`）。

不做关键词搜索：路线是管理员维护的精选内容，数量级是「几十条」而不是「几千条」，方向筛选 + 分页足够。地图 Not yet specified 里那条「分页/搜索性能边界」由本决策收口——**答案不是「定一个索引策略」，而是「不做搜索，所以不存在这个边界」**。将来路线真的多到需要搜索时再开票。

### 10. JSONB 只保证是数组，内容校验全在 Go

数据库层只有 `CHECK (jsonb_typeof(stages) = 'array')`。逐字段校验（`title` 非空、`url` 过协议白名单、阶段 id 唯一且格式合法、数量上限）全在 Go 的 service 层做。

理由：PostgreSQL 对 JSONB 做完整结构校验要么装扩展、要么写超长 `CHECK`，而它的**错误信息对用户不可读**。Go 侧校验能和 DTO binding 一起给出「第 2 个阶段的第 3 个资源链接不是 http/https」这种能直接改的错误。

**数量上限**：阶段 ≤ 20，每阶段资源 ≤ 20。这不是随意取的数——单行 JSONB 若被塞到几百 KB，列表页查询要把整行读出来，而列表页只需要摘要。

### 11. 资源 url 用 ADR 0005 的同一份协议白名单

只允许 `http:` / `https:` / 以 `/` 开头的站内相对路径，不合规**直接拒绝保存**。

不在渲染层兜底：管理员录错协议要等用户点到才发现，那时错误已经上线。校验层与渲染层共用同一份白名单定义，避免两套规则漂移。

### 12. 路线正文允许插图，复用同一套重算逻辑

保存路线时从 `body` 里正则提取 `/api/v1/attachments/(\\d+)`，重算绑定集合——与发帖、回复**共用同一条 service 逻辑**（ADR 0004 决策 3），只是绑定的目标列换成 `roadmap_id`。

不让路线绕开重算：那会让「正文是绑定唯一真源」这个口径分裂成两套，图片从正文删掉后仍绑在路线上。

### 13. 正文长度上限沿用 20000

与帖子正文同一口径（ADR 0005 决策 9），迁移票的 `CHECK` 可以照抄，前端校验复用同一个常量。本票只定数值，`CHECK` 由迁移票落地。

### 14. 接口路径不挂 `/studio` 也不挂 `/recruitment`

- 用户侧：`GET /api/v1/learning-paths`、`GET /api/v1/learning-paths/:id`
- 进度：`PUT /api/v1/learning-paths/:id/progress`、`DELETE /api/v1/learning-paths/:id/progress`
- 管理端：`/api/v1/admin/learning-paths`（`GET` / `POST` / `GET :id` / `PUT :id` / `DELETE :id`）

学习路线不属于工作室运营域（它不是 `/studio` 那组），也不是招新的子资源（它跨越多轮招新）。它是**面向登录用户的顶层内容**，所以是顶层路径。

进度不挂 `/me`：`/me` 前缀暗示「我的个人数据集合」，而进度是某条路线的附属状态，路径上带路线 id 更自然。

### 15. 管理端页面放「工作台」分组，路径 `/recruitment/learning-paths`

侧边栏现有三个分组没有「招新」组，招新相关页（申请审核、候选人处理）都在「工作台」组里。票面说的「管理端『招新』分组下新增页面」，落地为**放在「工作台」组、用 `/recruitment/` 路径前缀**——语义正确，且不需要重排现有 6 个菜单项。

不新建「招新」分组：那是与本次需求无关的导航重构。

## 拒绝的方案

| 方案 | 原因 |
| --- | --- |
| 三张表 `roadmaps` / `roadmap_stages` / `roadmap_resources` | 阶段与资源没有独立查询需求；管理端保存要处理三层 diff、要防孤儿资源 |
| 两张表，资源仍 JSONB | 折中方案换来的是「阶段可单独查」这个本期用不上的能力 |
| 阶段用自增序号做主键 | 删中间阶段后再新增会复用已删 id，进度指到新阶段 |
| 阶段不带 id，进度存数组下标 | 管理员重排后进度**静默**指到别的阶段 |
| 阶段 id 用 `s_1` / `s_2` | 可读但会复用；不可猜测的随机串在两个方向上都安全 |
| 进度塞进 `users` 表的 JSONB 列 | 污染已承载认证/冻结/资料三套语义的表；查询要解析 JSONB |
| 进度存百分比 | 用户要自己算，实际不会有人填 |
| 管理员可查看他人进度 | 多一套权限判定与聚合接口，本期无消费场景 |
| 方向加硬外键 | 方向是招新运营数据、路线是长期内容，一次招新结束不该让路线悬空失败 |
| 路线与方向多对多 | 管理端多一个多选控件；通用内容可以复制一条路线 |
| 加 `published` 草稿态 | 每个查询多一个 `WHERE`，且「为什么用户看不到」变成排查问题 |
| 硬删路线 | 用户进度行悬空，误删不可恢复 |
| 版本历史（照搬知识库 revisions） | 路线是推荐内容不是团队文档，改错直接改回来即可 |
| 拖拽排序 | 仓内无拖拽库，自写复杂度远大于体验收益 |
| 每个阶段手填序号 | 要校验重号与空洞，填错要逐个改 |
| 用户端复用招新方向接口做筛选 | 招新结束后方向全关，路线页会一个方向都不剩 |
| 前端硬编 code → 中文名映射 | 与方向表的 `name` 可能不一致，两处名字漂移 |
| 关键词搜索 | 路线数量级几十条，方向筛选 + 分页足够；地图那条「搜索性能边界」由此消解 |
| 数据库层做完整 JSONB 结构校验 | 要装扩展或写超长 `CHECK`，错误信息对用户不可读 |
| 不设阶段/资源数量上限 | 一行 JSONB 可能被塞到几百 KB，列表页要整行读出 |
| 资源 url 只在渲染层兜底 | 管理员录错要等用户点到才发现 |
| 路线正文不允许插图 | `content_attachments.roadmap_id` 白留；而学习路线恰恰最需要配图 |
| 路线绕开正文重算 | 「正文是绑定唯一真源」口径分裂成两套 |
| 接口挂 `/studio/learning-paths` | 路线不属于工作室运营域 |
| 接口挂 `/me/learning-paths` | 路线是公共内容不是个人数据，`/me` 会误导 |
| 进度接口挂 `/me/progress` | 进度是路线附属状态，路径带路线 id 更自然 |
| 新建「招新」侧边栏分组 | 与本次需求无关的导航重构 |
| 路线正文长度另定数值 | 多一个要维护的常量，与帖子正文没有区别 |

## 后果

**新增 2 张表**：`learning_roadmaps`、`user_roadmap_progress`（含索引与唯一键）。迁移号接 `0016` 起，与 ADR 0003 的另外三张表同批。

**新增 1 组模块**（按仓内 8 处范式）：domain / dto request / dto response / repository / service / handler / `wire.go` / `router.go`。

**前端新增 1 个模块** `apps/web/src/modules/learning-paths/`（`routes.ts` + `views/` + `components/` + 纯函数 model + 就近单测），API 放 `apps/web/src/api/learning-paths.ts`。

**复用既有资产**：Markdown 渲染器（ADR 0005）、附件重算逻辑（ADR 0004）、分页参数解析范式、`SignInPrompt` 不需要（路由直接拦）。

**术语**：`GLOSSARY.md` 的「学习路线」词条要补上阶段 / 资源 / 进度三个概念。

**下游约束**：

- 迁移票：`learning_roadmaps` 的 `CHECK (jsonb_typeof(stages) = 'array')` 与 `CHECK (char_length(body) <= 20000)`；`user_roadmap_progress` 的 `UNIQUE (user_id, roadmap_id)`。
- 实施票（后端）：阶段 id 生成与对齐规则、资源 url 白名单、正文附件重算、分页。
- 实施票（前端）：两个路由 + 管理端单页编辑器 + 上下移动排序 + 详情页进度标记。

**可逆性边界**：JSONB 换成三张表是一次纯数据迁移，不改接口契约——这是选它的一个理由：**接口形状不因存储方式而变**。但阶段 id 一旦发出去就不能再收回：它是进度的外键，改格式要迁移所有进度行。这是本文最难逆转的一条，也是唯一必须在建表前定下来的。

## 参考

- 地图：#207「[探索地图] 招新问答与学习路线」；本票：#211
- 内容模型与权限矩阵：`docs/adr/0003-content-scope-shared-model.md`
- 附件上传链路：`docs/adr/0004-attachment-upload-chain.md`
- Markdown 渲染：`docs/adr/0005-markdown-rendering.md`
- 知识库编辑范式：`apps/api/internal/service/studio_knowledge_usec.go:64,112,156-158`
- 方向表：`apps/api/migrations/0007_recruitment_tables.up.sql:36-47`；开放方向过滤：`apps/api/internal/service/recruitment_usec.go:100-113`
- 前端路由清单：`apps/web/src/router/route-specs.ts`；侧边栏：`apps/web/src/components/common/AdminLayout.vue:38-111`
