# ADR 0002：图片存储选型（MinIO / S3 兼容）

## 状态

已确认（2026-10-30）。来自地图 #207「招新问答与学习路线」的调研票 #209；调研证据见 `docs/research/minio-image-storage.md`。

**未落地**：本文只记录选型与服务端发行版决策，实施见 #213（部署）与后续上传链路票。

## 上下文

招新问答与内部论坛都需要「正文/回复可配图」（地图 #207 的 Q14/Q15：正文与回复均可插图，单张 ≤ 10MB、每次最多 9 张、格式限 jpg/png/webp）。

仓库现状（调研确认）：

- `apps/api` **完全没有上传能力**：`multipart` / `FormFile` / `MaxMultipartMemory` / `r.Static` / `http.FileServer` 全零命中，`go.mod` 无任何对象存储 SDK。
- `docker-compose.yml` 只有 postgres、redis、app、registry；`deploy/nginx.conf` 全文无 `client_max_body_size`（默认 1m）。
- `.github/workflows/deploy.yml` 的生产 `docker run` **没有任何 `-v`**。

## 决策

### 1. 存储用 MinIO，走 S3 API

客户端 `github.com/minio/minio-go/v7`。**业务代码只依赖 S3 API**，不依赖任何 MinIO 私有协议——这是本 ADR 最重要的可逆性保证：换 Garage / SeaweedFS / 云厂商 OSS 只需改 endpoint 与凭证。

### 2. 依赖锁 v7.3.0，不取 latest

| 版本 | `go.mod` 的 `go` 指令 | 可用 |
| --- | --- | --- |
| v7.4.1（latest） | `go 1.26.0` | 否，会要求升工具链 |
| v7.4.0 | `go 1.26.0` | 否 |
| **v7.3.0** | **`go 1.25.0`** | **是** |

本仓库 `apps/api/go.mod:3` 是 `go 1.25.0`，`Dockerfile:28` 是 `golang:1.25-alpine`。`go get` 不带版本会拉到 v7.4.1 并触发工具链升级，必须显式 `go get github.com/minio/minio-go/v7@v7.3.0`。

### 3. 服务端用 AIStor Free（免费 license），不用归档的社区版

`github.com/minio/minio` 仓库**已被 GitHub 标记 archived**（核实：`gh api repos/minio/minio` → `archived: true`，最后 push 2026-04-24），README 首行是 "THIS REPOSITORY IS NO LONGER MAINTAINED."，社区版改为只发源码、不再提供预编译二进制。

官方当前唯一背书的容器镜像是 `quay.io/minio/aistor/minio`，需挂载 license 文件。**接受这个许可成本**，换取仍在维护的服务端。

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
| 沿用归档的 `minio/minio` 社区镜像 | 仓库 archived、无安全更新；且调研时 Docker Hub 取证失败（404），镜像是否还能拉取未证实 |
| 本地磁盘 + 持久卷 | 本轮已由用户明确否掉（Q4） |
| 白名单 SVG | SVG 可内嵌 `<script>`，同源返回即存储型 XSS |

## 后果

**上线前必须同时改三处，否则大图必然失败**（调研第 6 节）：

1. `deploy/nginx.conf` 加 `client_max_body_size 6m`（默认 1m，超限 413，且官方文档明确说浏览器无法正确显示该错误）。nginx 限制必须**大于**应用层限制，否则用户拿到的是 nginx 的纯 HTML 413 而不是 `{code, message}`。
2. Go 侧真正的闸门是 `http.MaxBytesReader` + `fh.Size`。Gin 的 `MaxMultipartMemory` 默认 32MB **只是「多少进内存」的阈值，完全不限制上传大小**——超出部分整体落 `os.TempDir()` 临时文件。
3. MinIO 容器必须挂数据卷。`deploy.yml` 现在的 `docker run` 没有任何 `-v`，照现状跑每次发布都会丢图。

其他后果：

- 生产部署多一个容器 + 一个 license 文件挂载，`deploy.yml` 与 `smoke.sh` 都要相应扩展。
- 对象名用 `crypto/rand` 生成、扩展名从嗅探结果推导，**绝不用用户文件名做 key**（路径穿越 / 同名覆盖 / 泄漏真实姓名）。
- 业务表存 `object_key` 而非完整 URL，换 endpoint 时不用刷数据。
- 孤儿对象回收策略**尚未定**（删帖/删回复后 MinIO 对象谁回收、何时回收），已记录在地图 #207 的 Not yet specified。

## 参考

- `docs/research/minio-image-storage.md`（一手来源 21 条）
- `docs/adr/0001-studio-member-project-model.md`
- 地图：#207；调研票：#209；部署票：#213
