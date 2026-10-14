# 数据库迁移

Schema 由本目录 SQL 管理，API 不运行 `AutoMigrate`。部署在停止旧容器之前自动执行 `iot-migrate up`，失败中止发布。工具使用固定版本 `golang-migrate v4.18.3`（兼容仓库 Go 1.25），SQL 编入同版本镜像的独立二进制，不需要 Redis、JWT 或 API 启动。

0001–0009 SQL 保持原样，文件中的 `psql` 说明是历史说明。**0003 列重命名不幂等，0005 会重建索引，不能从头重放已执行迁移。**

## 本地运行

在 `apps/api` 执行。先将 `IOT_PILOT_DB_HOST/PORT/USER/PASSWORD/NAME` 导出到进程环境，`.env` 不会自动加载。可选 `IOT_PILOT_DB_SSLMODE`，默认 `disable`（当前同机 Docker 内网部署）；远程连接按数据库要求配置 TLS。凭据不接受命令行参数。

```bash
go run ./tools/migrate up
```

空库按序执行，已纳管库只执行新版本，重复执行为成功的空操作。`public.schema_migrations` 记录版本和 dirty 状态。仅支持 `public` schema。账号需要创建表/索引的权限；首次接管还需要在该数据库创建、删除参考 schema 的权限。工具设置 15 秒锁等待、120 秒 SQL 执行超时，同库并发操作立即拒绝。

## 已有手工迁移库：首次接管

**先备份，在从已有库备份恢复的隔离副本上验证，再对生产操作。** 合成已有库的集成测试不等同于生产副本。不要根据最大编号推断前序迁移齐全，不要直接 `force 9`。表单三表已存在时，`users.detail_qq` 仍可能缺失。

1. 核对人工记录和实际结构，列出确已执行的版本。可非连续，例如 `1,2,3,4,5,7,9` 表示遗漏 0006/0008。这只是示例，不能照抄用于生产。
2. 在副本运行 `check`：事务内创建唯一参考 schema，只用该 schema 作为搜索路径，执行清单中原 SQL，比对业务表的列类型、默认值、非空/主键约束、索引定义（含唯一性、谓词）。事务始终回滚，不写接管记录。
3. 清单不成立、列/索引漂移、结构未完整落地、额外业务列/索引都会拒绝。0007 还检查轮次 `round_num=1` 和 `front-end/back-end/agent` 三个方向标识是否齐全；缺少标识（包括有意删除）需先人工查明原因。无关表不参与比对；种子的名称、开放标记和排序允许业务修改，不会被重置。不要为了通过检查删除生产结构。
4. `adopt` 重新检查后，原子记录已执行版本和 SQL SHA-256 到 `public.migration_adoptions`，不执行目标库业务 DDL。随后 `up` 跳过清单中的 SQL，按序补齐缺失版本，不直接跳到最大版本。

```bash
# 在 apps/api；环境变量先指向隔离副本
go run ./tools/migrate check --applied 1,2,3,4,5,7,9
go run ./tools/migrate adopt --applied 1,2,3,4,5,7,9
go run ./tools/migrate up
# 再次执行不重复 DDL/种子数据
go run ./tools/migrate up
```

清单需满足表依赖：0003/0004 依赖 0002，0005 依赖 0001/0002，0006 依赖 0001。仅执行过 0007/0009 的库也可用 `--applied 7,9` 接管。单份 SQL 只执行了一部分时，工具不会猜测和修补。

接管是一次性操作，已有迁移元数据时拒绝再次 `adopt`。已接管 SQL 校验和变化会阻止 `up`；后续变化必须增加新迁移。工具的 source 将已验证版本视为已执行；**不要绕过本工具直接运行通用 migrate CLI 的 up/down**，否则无法正确处理非连续历史。

### 服务器首次上线

首次带工具的部署先加载镜像，再对未纳管已有库安全失败，旧容器继续运行，`deployed` tag 和 `DEPLOYED_SHA` 不推进。待发布镜像仍在服务器，可供接管。

准备权限为 `600` 的环境文件，内容为本节列出的 `IOT_PILOT_DB_*`，不要提交、粘贴到聊天或写入日志。副本通过且备份可恢复后，使用**同一待发布镜像**执行：

```bash
# 替换为待发布 tag、安全配置路径和核实过的版本清单
IMAGE=iot-pillot-api:待发布的12位SHA
ENV_FILE=/secure/migration.env
APPLIED=1,2,3,4,5,7,9
docker run --rm --network iot-pilot-net --env-file "$ENV_FILE" \
  --entrypoint /usr/local/bin/iot-migrate "$IMAGE" check --applied "$APPLIED"
docker run --rm --network iot-pilot-net --env-file "$ENV_FILE" \
  --entrypoint /usr/local/bin/iot-migrate "$IMAGE" adopt --applied "$APPLIED"
```

随后重跑部署 workflow，自动 `up` 补齐遗漏迁移再切换应用。生产 workflow 不接收未经审核的 baseline 参数。核对最终列/表与业务请求：`users.detail_qq`、0007 招新四表、0008 邀请/候选人三表、0009 表单三表。找回密码此前失败会消费验证码，验证恢复需要重新取码。

## 失败与回滚

- 迁移失败：不停止旧容器，不启动新 API，不推进部署水位。已成功的前序迁移保持提交；失败版本原 SQL 有 `BEGIN/COMMIT`，其业务变更回滚，版本保持 dirty，重试不会自动清除。
- dirty 修复：检查版本表和数据库日志，在副本复现并确认失败事务完整回滚，修复数据问题（例如重复邮箱）后，由维护者依据最后实际成功版本修复元数据，再重试。不要无条件清除 dirty 或凭最大文件编号 force。工具错误不输出失败 SQL 或行数据。
- 迁移成功、应用健康失败：沿用镜像回滚，**不自动 down 数据库**。旧代码须兼容新增结构；未来迁移遵循 expand/contract，删除字段与应用发布分阶段进行。
- 默认数据库恢复策略是 roll-forward：新建修复迁移。历史 `.down.sql` 可能删除真实数据，只能由维护者在备份验证后进行事故恢复。镜像回滚不等于数据库回滚。

## 验证

```bash
# 在 apps/api：只指向可销毁的 PostgreSQL，账号须有 CREATEDB 权限。
# 每例创建独立 migration_test_* 库，结束后删除；绝不能指向生产。
IOT_MIGRATION_TEST_DSN='postgres://test_user:test_password@127.0.0.1:5432/test_db?sslmode=disable' \
  go test -count=1 -v ./internal/migration
```

未配置测试 DSN 时数据库用例明确 skip。CI 和部署 runner 显式连接临时 PostgreSQL 16，覆盖空库、非连续接管、低编号遗漏、结构漂移、数据保留、索引 OID 不变、dirty 重试、并发锁与校验和保护；镜像冒烟另执行镜像内迁移命令。`bash deploy/test/run.sh` 验证迁移失败保留旧容器及原有镜像回滚。
