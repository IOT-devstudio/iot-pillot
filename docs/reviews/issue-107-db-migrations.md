# Issue #107 本地交付与验证

## 范围

基于 `origin/main` 的 `99a50480f761e35b78b59ce859fa2f939bf2814d`，独立分支 `feat/issue-107-db-migrations`。原工作区仍在 `feat/issue-85-password-flows`，HEAD 为 `965aade87aa0a2353a4eb310b4ffcf6a091c8593`，未改变其提交或文件。

新增独立 `iot-migrate` 工具，采用 golang-migrate v4.18.3。0001–0009 up/down SQL 未修改，业务代码未修改。已有库先明确清单并比对参考结构，再记录接管元数据；工具跳过已验证迁移，补齐遗漏版本。部署在停止旧容器前执行同镜像内的工具，失败不切换应用、不推进部署水位。

## 已验证（2026-10-14）

- Go `vet ./...`、`test ./...`、`build ./...` 通过；全量测试显式设置临时 PostgreSQL DSN，集成测试实际执行而非 skip。
- 最终回归使用 Docker 隔离的 PostgreSQL 16.15：9 组迁移测试（含 4 个结构拒绝、2 个种子拒绝子例）通过。覆盖空库和重复 up；缺 0006/0008、已有 0007/0009 的非连续清单；只有 0007/0009 的清单；已有用户与修改后的种子数据保留；已执行 0005 的索引 OID 不变。
- 负例覆盖：错误宣称已执行 0006、列类型漂移、唯一索引被换成普通索引、0003 部分重命名、0007 未播种/方向标识不完整、重复邮箱导致 0005 失败、dirty 重试拒绝、并发操作拒绝、接管校验和不符。
- 部署脚本先添加负例，未接入迁移时新增两例失败；接入后 `bash deploy/test/run.sh` 全部 13 项通过，包括失败保留旧容器、不写 `DEPLOYED_SHA`、迁移先于 stop，以及原有跨端口镜像回滚。
- CI/部署 workflow YAML 解析通过，Bash 语法通过，`git diff --check` 通过。
- `CGO_ENABLED=0 GOOS=linux go build ./tools/migrate` 通过。
- 最终单镜像 `iot-pillot-api:issue107-test` 构建通过，`docker image inspect` 返回 image ID `sha256:99ab088b51593fe08150dfaa3ffa6cfe0aadd4be9e3e5efc4bef4c13724d77d6`；构建输出中的 config digest 为 `sha256:0df0a0e854e5c936a42e6a04927e5c655548889199af166629f293dc494c4bc2`。镜像内真实 CLI 验证 3 组端到端场景：空库+重复 up；拒绝未纳管库再 check/adopt 非连续历史并保留用户、修改后的轮次名称和索引 OID；重复邮箱迁移失败留下 `5,true`，再次 up 拒绝 dirty。
- 最终镜像 HTTP 冒烟 7 项通过，生命周期冒烟通过（杀 API 后第 1 秒容器退出）。Windows Git Bash 首次把 `/usr/local/bin/api` 转成 `E:/Git/usr/local/bin/api` 导致 pkill 未匹配；本地设置 `MSYS_NO_PATHCONV=1` 后使用同一原脚本通过，不需要改变 Linux CI 脚本。
- 构建仍有原前端 bundle 大于 500KB 的 Vite 提示，构建成功，未修改前端源码。
- Docker Desktop 更新至 4.95.0、Engine 29.8.2 后，本轮复验再次通过镜像 CLI 三组场景、HTTP 冒烟 7 项和生命周期冒烟（第 1 秒退出）。测试复用了本任务隔离的 PostgreSQL/Redis，未启动或修改其他项目容器。

## 双轴审查

- Standards：发现新增迁移调用将密码展开到 Docker argv，已改为只传环境变量名；打桩确认参数不含密码、环境仍可读取配置。复查通过。
- Spec：发现 0007 只有 DDL、遗漏种子记录时可能被接管，已增加必要种子标识检查和两个先失败后通过的回归子例；允许保留名称、开放状态和排序等业务修改。复查通过。

## 验证边界

- Docker Desktop 在构建期间曾失联：两个引擎入口超时、WSL 仍运行但无 dockerd，常规重启也超时；重新启动失联的 Desktop 应用后引擎恢复、复用缓存构建成功。日志显示 bootstrap 收到中断，未证明中断来源。没有删除原有容器、数据卷或镜像缓存。
- 测试使用合成的已有库，未取得真实生产库备份副本，不能声称已满足生产副本验收或线上缺列已恢复。
- 未连接/修改生产数据库，未重启生产应用，未发布分支或 PR，未关闭 Issue #107。

## 上线前操作

按 `apps/api/migrations/README.md` 备份并在已有库副本验证实际清单，再用同一待发布镜像执行生产 check/adopt。第一次发布遇未纳管已有库会有意中止并保留旧容器；接管后重跑部署自动补齐迁移。数据库默认 roll-forward，应用镜像回滚不自动执行破坏性的 down。
