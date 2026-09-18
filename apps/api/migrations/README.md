# 数据库迁移

本目录是 iot-pillot 后端 schema 的唯一变更入口。**不要手工改表**，所有变更都往这里加文件。

## 为什么现在还没有迁移工具

`CLAUDE.md` 把「数据库迁移工具」列为未决项（golang-migrate 为首选、goose 为备选），
本轮先只交 SQL，因此：

- 文件命名刻意采用 **golang-migrate 的规范**（`<序号>_<描述>.up.sql` / `.down.sql`），
  将来引入工具时可以直接指定本目录为 source，不需要重命名或搬移。
- 当前没有 `schema_migrations` 版本表，**执行历史需要人工记录**。
  引入工具后，第一件事应该是把已执行的版本补进版本表。

## 怎么执行

```bash
# 一个个按顺序执行（连接串自行替换）
psql "$DSN" -v ON_ERROR_STOP=1 -f apps/api/migrations/0001_users_recruitment_fields.up.sql
psql "$DSN" -v ON_ERROR_STOP=1 -f apps/api/migrations/0002_mail_tables.up.sql
```

- `-v ON_ERROR_STOP=1` 必须加：否则 psql 遇到错误会继续往下跑，
  一个失败的迁移会被当成"执行过了"。
- 每个文件都用 `BEGIN; ... COMMIT;` 包住，失败会整体回滚，不会留下半截 schema。
- 文件都写成幂等的（`IF NOT EXISTS` / `DROP ... IF EXISTS`），重复执行不报错。

执行前建议先看清现有表结构，尤其是 0001 里对 `users` 的假设：

```sql
\d users
```

## 文件清单

| 文件 | 作用 |
| --- | --- |
| `0001_users_recruitment_fields.up.sql` | `users` 补 `detail_*` 四列与 `parse`；放开 `name` 唯一约束；补索引 |
| `0002_mail_tables.up.sql` | 新建 `mail_models`（模板）与 `mails`（发信记录） |

回滚用同名的 `.down.sql`。

## 执行前必须确认的三处

SQL 里已经用 `⚠️` 标出，这里再集中列一次，因为它们都会改既有语义：

1. **`users.name` 的唯一约束会被删除**（`0001`）。
   依据是 `domain/user.go` 去掉了 `uniqueIndex`。但 `name` 是登录标识，
   `GetByName` 用 `First()`，重名会导致"密码对却登不上"。若不是有意为之，
   删掉 `ALTER TABLE users DROP CONSTRAINT ...` 那两行再执行。
2. **`mail_models.type` 建成了唯一索引**（`0002`）。
   等于「每种类型只能有一个模板」，与 `CLAUDE.md` 里「支持扩展新增」的要求可能冲突。
3. **`parse` 的默认值是 0 = 工作室成员**。
   既有行回填成 0 是正确的（它们本来就是成员），但注册流程必须**显式**写招新轮次，
   否则新招的人会静默变成"工作室成员"。

## 与 GORM 的关系

项目里**没有任何 `AutoMigrate` 调用**（`internal/repository/database.go` 只负责连库），
所以表结构完全由本目录的 SQL 决定。DDL 的列类型按 GORM 的约定写
（`int → bigint`、`string → text`、`time.Time → timestamptz`、非主键列可空），
将来若真的启用 `AutoMigrate`，不会因为类型不一致而反复 ALTER。
