# ADR 0002：图片存储选型（S3 兼容对象存储）

## 状态

已确认（2026-10-30）。来自地图 #207「招新问答与学习路线」的调研票 #209；调研证据见 `docs/research/minio-image-storage.md`。

**2026-10-30 复核（部署票 #213）**：服务端发行版由 MinIO AIStor 改为 **RustFS 1.0.4**。MinIO 已撤下 Docker Hub 镜像（`minio/minio` 与 `minio/mc` 均 404），Quay 匿名拉取返回 401，`dl.min.io` 社区版二进制 410 Gone。决策 1（业务代码只依赖 S3 API）不变，变的只是发行版。文件名保留历史名，内容以本文为准。

**未落地**：本文只记录选型与服务端发行版决策，实施见 #213（部署）与后续上传链路票。

## 上下文

招新问答与内部论坛都需要「正文/回复可配图」（地图 #207 的 Q14/Q15：正文与回复均可插图，单张 ≤ 10MB、每次最多 9 张、格式限 jpg/png/webp）。

仓库现状（调研确认）：

- `apps/api` **完全没有上传能力**：`multipart` / `FormFile` / `MaxMultipartMemory` / `r.Static` / `http.FileServer` 全零命中，`go.mod` 无任何对象存储 SDK。
- `docker-compose.yml` 只有 postgres、redis、app、registry；`deploy/nginx.conf` 全文无 `client_max_body_size`（默认 1m）。
- `.github/workflows/deploy.yml` 的生产 `docker run` **没有任何 `-v`**。

## 决策

### 1. 存储走 S3 API，服务端可替换

客户端 `github.com/minio/minio-go/v7`。**业务代码只依赖 S3 API**，不依赖任何 MinIO 私有协议——这是本 ADR 最重要的可逆性保证：换 Garage / SeaweedFS / 云厂商 OSS 只需改 endpoint 与凭证。

### 2. 依赖锁 v7.3.0，不取 latest

| 版本 | `go.mod` 的 `go` 指令 | 可用 |
| --- | --- | --- |
| v7.4.1（latest） | `go 1.26.0` | 否，会要求升工具链 |
| v7.4.0 | `go 1.26.0` | 否 |
| **v7.3.0** | **`go 1.25.0`** | **是** |

本仓库 `apps/api/go.mod:3` 是 `go 1.25.0`，`Dockerfile:28` 是 `golang:1.25-alpine`。`go get` 不带版本会拉到 v7.4.1 并触发工具链升级，必须显式 `go get github.com/minio/minio-go/v7@v7.3.0`。

### 3. 服务端用 RustFS 1.0.4，不用已撤下发行渠道的 MinIO

`github.com/minio/minio` 仓库**已被 GitHub 标记 archived**（核实：`gh api repos/minio/minio` → `archived: true`，最后 push 2026-04-24），README 首行是 "THIS REPOSITORY IS NO LONGER MAINTAINED."。

本轮（#213）实测确认它已经**连镜像都拿不到**：`minio/minio` 与 `minio/mc` 在 Docker Hub 均 `not found`（2026-09 中旬撤下），`quay.io/minio/minio` 匿名 HEAD 返回 401，`dl.min.io/server/minio/release/linux-amd64/minio` 返回 410 Gone。同一台机器上 `alpine:3.20`、`redis:7-alpine`、`quay.io/prometheus/busybox` 都能拉，排除网络问题。

三个候选后端用 `minio-go v7.3.0` 写了真实探针（`BucketExists` → `MakeBucket` → `PutObject` → `GetObject` 比对 → `StatObject`），三个全绿，最终选 **RustFS**（`ghcr.io/rustfs/rustfs:1.0.4`）：

- **不需要任何初始化步骤**：全新实例直接可用，`MakeBucket` 现场成功 —— 这是选它的决定性理由，应用启动时的 `EnsureBucket` 才能在空存储上跑通。
- 单二进制、镜像内自带 `curl`（healthcheck 可用）、`/health/ready` 返回结构化就绪信息，并兼容 MinIO 的 `/minio/health/*` 路径。
- manifest 同时含 amd64 与 arm64；`1.0.4` 与 `latest` 同 digest。

Garage 与 SeaweedFS 探针同样通过，但都被否决：Garage 全新实例必须先手工 `layout assign` / `layout apply` / `bucket create` / `key create` 才有可用端点（否则 S3 一律 403），`EnsureBucket` 必然失败；SeaweedFS 不挂 `s3.json` 时**匿名可读写**（`curl` 能直接列出全部 bucket），挂配置又凭空多一个要维护的文件。

### 4. 私有 bucket + Go 代理转发，不用预签名 URL

预签名是 7 天硬上限的 bearer 凭证且不可撤销；更关键的是 SigV4 的 `getCanonicalHeaders` 强制把 `host` 签进 `SignedHeaders`——经 nginx 反代后 Host 与签名时不一致就直接 `SignatureDoesNotMatch`。代理路径同源、复用现有 `authRequired` 中间件、可做归属校验。

### 5. 上传走服务端中转，不做客户端直传

浏览器 → Go → MinIO。体量小（单图几 MB）、同源单镜像、无 CORS 需求；中转能在服务端做真实字节嗅探（`http.DetectContentType`）与体积闸门，直传省下的带宽换不来这些。

## 拒绝的方案

| 方案 | 原因 |
| --- | --- |
| 预签名 URL 直连 | 7 天硬上限 + 不可撤销；nginx 反代后 Host 与签名不一致直接失败 |
| 客户端直传 presigned PUT | 无法在服务端做真实类型嗅探与大小闸门 |
| 公开 bucket（`mc anonymous set download`） | key 一旦进日志/截图即永久公开，登录用户的私有内容不能这样放 |
| 沿用 `minio/minio` 社区镜像 | 2026-09 中旬起 Docker Hub 上 `minio/minio` 与 `minio/mc` 均 404，Quay 匿名拉取 401，`dl.min.io` 社区版二进制 410 Gone；仓库本身也已 archived |
| Garage | 全新实例必须先手工建 layout 与 key 才有可用 S3 端点，否则一律 403；应用启动时 `EnsureBucket` 会直接失败 |
| SeaweedFS | 不挂 `s3.json` 时匿名可读写（能直接列出全部 bucket），挂配置则多一个要维护的文件 |
| 本地磁盘 + 持久卷 | 本轮已由用户明确否掉（Q4） |
| 白名单 SVG | SVG 可内嵌 `<script>`，同源返回即存储型 XSS |

## 后果

**上线前必须同时改三处，否则大图必然失败**（调研第 6 节）：

1. `deploy/nginx.conf` 加 `client_max_body_size 6m`（默认 1m，超限 413，且官方文档明确说浏览器无法正确显示该错误）。nginx 限制必须**大于**应用层限制，否则用户拿到的是 nginx 的纯 HTML 413 而不是 `{code, message}`。
2. Go 侧真正的闸门是 `http.MaxBytesReader` + `fh.Size`。Gin 的 `MaxMultipartMemory` 默认 32MB **只是「多少进内存」的阈值，完全不限制上传大小**——超出部分整体落 `os.TempDir()` 临时文件。
3. MinIO 容器必须挂数据卷。`deploy.yml` 现在的 `docker run` 没有任何 `-v`，照现状跑每次发布都会丢图。

其他后果：

- 生产部署多一个常驻容器（无 license 文件、无额外配置文件），`deploy.yml` 要相应扩展；`smoke.sh` 按 #213 的决策**不加** S3 断言 —— 存储不可用时应用启动期 `EnsureBucket` 失败会让 `/health/ready` 直接变红，现有断言自动兜住。
- 对象名用 `crypto/rand` 生成、扩展名从嗅探结果推导，**绝不用用户文件名做 key**（路径穿越 / 同名覆盖 / 泄漏真实姓名）。
- 业务表存 `object_key` 而非完整 URL，换 endpoint 时不用刷数据。
- 孤儿对象回收策略已由 `docs/adr/0004-attachment-upload-chain.md` 定稿：未绑定附件超过 24 小时由进程内 ticker（每小时、单批 500）回收；内容软删时附件行与对象都**保留**，不做回收。

## 参考

- `docs/research/minio-image-storage.md`（一手来源 21 条）
- `docs/adr/0001-studio-member-project-model.md`
- 地图：#207；调研票：#209；部署票：#213
