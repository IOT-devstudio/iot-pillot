# MinIO 图片存储接入调研（Go + Gin）

> 选址说明：`docs/` 现有子目录只有 `superpowers/{specs,plans}`（按 `YYYY-MM-DD-<topic>-design.md` 命名的设计/计划）、`reviews/`（issue-NN）、`agents/`、`adr/`、`demos/`，没有承载「只调研、不出方案」这类笔记的槽位；本文件名被固定为 `minio-image-storage.md`，不套日期前缀，塞进 `superpowers/specs` 会破坏该目录的命名惯例，因此按任务约定兜底放到新建的 `docs/research/`。

调研时间：2026-10-30。所有结论只依据一手资料（上游仓库源码、官方文档、Go 标准库源码、nginx 官方文档），每节末尾给出 URL。

---

## 0. 结论速览

1. **依赖锁 `github.com/minio/minio-go/v7 v7.3.0`，不要取 latest。** 当前 latest 是 v7.4.1，但 v7.4.x 的 `go.mod` 已声明 `go 1.26.0`，而本仓库 `apps/api/go.mod:3` 是 `go 1.25.0`、`Dockerfile:28` 用 `golang:1.25-alpine`；v7.3.0 的 `go.mod` 才是 `go 1.25.0`。
2. **图片走「私有 bucket + Go 代理转发」，不走预签名直连。** 预签名 URL 是 bearer 凭证、7 天上限、不可撤销，且 SigV4 会把 `host` 强制签进 `SignedHeaders`——一旦走 nginx 反代 MinIO，转发后的 Host 与签名时的 Host 不一致就直接 `SignatureDoesNotMatch`；再加上跨域需要 bucket CORS，而 bucket CORS 在社区版是否可用存疑。代理转发则完全同源、鉴权继续走现有 JWT 中间件。
3. **上传走服务端中转（浏览器 → Go → MinIO），不做客户端直传 presigned PUT。** 体量小（单图几 MB）、部署是同源单镜像、无 CORS 需求，中转能在服务端做真实类型嗅探与体积闸门；直传省的那点带宽换不来这些。
4. **上线前必须同时改三处配置，否则大图必然失败**：`deploy/nginx.conf` 加 `client_max_body_size`（默认 **1m**）、Gin 侧 `http.MaxBytesReader` + `file.Size` 校验（`MaxMultipartMemory` 只是内存阈值 **32MB**，不是上传上限）、以及给 MinIO 补数据卷（`docker-compose.yml` 加 volume，`.github/workflows/deploy.yml` 的 `docker run` 目前**没有任何 `-v`**）。
5. **MinIO 服务端本身有变数**：`github.com/minio/minio` 仓库已标记不再维护，社区版改为只发源码；官方现行容器文档只推 `quay.io/minio/aistor/minio`，而 AIStor 需要 license。自建时要么接受这个许可成本，要么改用其他 S3 兼容实现（本项目代码只依赖 S3 API，切换成本低）。

---

## 1. minio-go v7：版本、导入路径与最小代码

### 1.1 版本选择

| 版本 | `go.mod` 的 `go` 指令 | 能否直接用在本仓库（`go 1.25.0`） |
| --- | --- | --- |
| v7.4.1（latest，2026-10-12） | `go 1.26.0` | 否，会要求把工具链升到 1.26 |
| v7.4.0 | `go 1.26.0` | 否 |
| **v7.3.0** | **`go 1.25.0`** | **是，推荐** |
| v7.2.0 | `go 1.25.0` | 是 |
| v7.1.0 | `go 1.25` | 是 |
| v7.0.95 / v7.0.90 | `go 1.23.0` | 是 |

`go get` 时不写版本会拉到 v7.4.1，然后 Go 会提示需要升级 `go` 指令/工具链。正确写法：

```bash
cd apps/api
go get github.com/minio/minio-go/v7@v7.3.0
```

导入路径：`github.com/minio/minio-go/v7`，凭证子包 `github.com/minio/minio-go/v7/pkg/credentials`。

来源：
- 版本列表与最新版：https://proxy.golang.org/github.com/minio/minio-go/v7/@v/list 、https://proxy.golang.org/github.com/minio/minio-go/v7/@latest
- 各版本 `go` 指令（把版本号替换进 URL 即可核对）：https://proxy.golang.org/github.com/minio/minio-go/v7/@v/v7.3.0.mod 、https://proxy.golang.org/github.com/minio/minio-go/v7/@v/v7.4.1.mod
- 包文档与 README（`go get github.com/minio/minio-go/v7`）：https://pkg.go.dev/github.com/minio/minio-go/v7
- 仓库：https://github.com/minio/minio-go

### 1.2 关键签名（v7.3.0 / v7.4.1 一致）

```go
func New(endpoint string, opts *Options) (*Client, error)
func (c *Client) BucketExists(ctx context.Context, bucketName string) (bool, error)
func (c *Client) MakeBucket(ctx context.Context, bucketName string, opts MakeBucketOptions) error
func (c *Client) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts PutObjectOptions) (UploadInfo, error)
func (c *Client) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
func (c *Client) PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error)
func (c *Client) RemoveObject(ctx context.Context, bucketName, objectName string, opts RemoveObjectOptions) error
func NewStaticV4(id, secret, token string) *Credentials
```

`Options` 里本项目会用到的字段：`Creds`、`Secure`（TLS）、`Transport`（自定义 `http.RoundTripper`，可用于设超时/连接池）、`Region`、`BucketLookup`（`BucketLookupAuto`/`DNS`/`Path`）。

**endpoint 不带 scheme**：`minio.New("minio:9000", ...)`，协议由 `Secure` 决定；写成 `http://minio:9000` 会解析失败。

### 1.3 最小可用代码（建桶 / 上传 / 预签名 / 删除）

```go
package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc     *minio.Client
	bucket string
}

func NewClient(endpoint, accessKey, secretKey, bucket string, secure bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}
	return &Client{mc: mc, bucket: bucket}, nil
}

// EnsureBucket 幂等建桶：MakeBucket 失败后必须再查一次存在性，
// 否则并发启动的多个实例会有一个直接报 BucketAlreadyOwnedByYou 退出。
func (c *Client) EnsureBucket(ctx context.Context) error {
	err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
	if err == nil {
		return nil
	}
	exists, existsErr := c.mc.BucketExists(ctx, c.bucket)
	if existsErr != nil {
		return fmt.Errorf("MakeBucket: %w (BucketExists 也失败: %v)", err, existsErr)
	}
	if !exists {
		return fmt.Errorf("MakeBucket: %w", err)
	}
	return nil
}

func (c *Client) Put(ctx context.Context, objectName string, r io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, objectName, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (c *Client) PresignedGet(ctx context.Context, objectName string, expires time.Duration) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, objectName, expires, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *Client) Remove(ctx context.Context, objectName string) error {
	return c.mc.RemoveObject(ctx, c.bucket, objectName, minio.RemoveObjectOptions{})
}
```

### 1.4 预签名有效期硬上限

`isValidExpiry` 的实现：秒数 < 1 报 `"Expires cannot be lesser than 1 second."`，> 604800 报 `"Expires cannot be greater than 7 days."`。所以 `expires` 只能取 1 秒 ~ 7 天，代码里不要出现 `30*24*time.Hour`。

来源（源码）：
- `api-presigned.go:32` 注释 `Expires maximum is 7days - ie. 604800 and minimum is 1.`
- `utils.go:216-225` `isValidExpiry`
- https://pkg.go.dev/github.com/minio/minio-go/v7@v7.3.0#Client.PresignedGetObject

---

## 2. 预签名 URL vs 后端代理转发（Gin + nginx）

### 2.1 三种形态

| 形态 | 浏览器请求地址 | 鉴权 | CORS | nginx |
| --- | --- | --- | --- | --- |
| A 预签名直连 | `http://minio:9000/bucket/key?X-Amz-...`（需暴露 9000） | 靠 URL 签名 | **需要 bucket CORS** | 不需要，但要把 9000 暴露到公网 |
| B 后端代理 | `/api/v1/images/:id` → Go `GetObject` 流式写响应 | 复用现有 JWT 中间件 | 同源，不需要 | 不需要改 |
| C nginx 直反代 MinIO | `/media/bucket/key?X-Amz-...` → MinIO | 靠 URL 签名 | 同源，不需要 | 需要新 location，且要处理 Host |

### 2.2 为什么本仓库应选 B（后端代理）

**(a) 预签名 URL 一旦签发就是可转发的凭证。** 它不绑定 Cookie、不绑定 Referer、不能被服务端撤销，任何人拿到就能读到过期为止。对「登录用户上传的提问配图」这类内容，泄漏面等于把对象直接公开一段时间。

**(b) SigV4 强制签 `host`，导致 nginx 反代极易 `SignatureDoesNotMatch`。** minio-go 的 `getCanonicalHeaders` 里 `if !headerExists("host", headers) { headers = append(headers, "host") }`，`host` 永远进 `SignedHeaders`。也就是说签名时用的 Host 必须与浏览器实际发出的 Host 完全一致。若 Go 侧 client 的 endpoint 是内网 `minio:9000`，签出来的 Host 就是 `minio:9000`；浏览器却用 `example.com` 访问，nginx 转发时 `proxy_set_header Host $host` 传的是 `example.com` → 签名校验失败。要么把 client endpoint 设成对外域名（那就得让 Go 能解析并直连对外域名，且内网绕一圈），要么放弃这条路径。

**(c) 跨域需要 bucket CORS。** 浏览器直连 9000 端口是跨源请求，GET 图片本身虽是「简单请求」，但 `fetch` 读响应、以及 PUT 上传都要预检。MinIO 的 bucket 级 CORS 用 `mc cors set ALIAS/BUCKET cors.xml` 配置（XML 遵循 S3 CORS schema，最多 100 条规则，未配置时回退全局 CORS），但社区版是否支持 bucket CORS 存在争议（GitHub discussion 20841 指向商业版）。本仓库是单镜像同源部署，没必要引入这个不确定性。

**(d) 鉴权与审计。** 代理路径天然能回答「谁看了哪张图」：handler 里已有 `middleware.AuthRequired(tokenManager)`（`apps/api/internal/handler/router.go:110`），可以顺手做归属校验（只有上传者/管理员能看），预签名 URL 做不到——除非每次访问都重新签发，那又退化成代理。

### 2.3 推荐实现（B）

```go
// GET /api/v1/images/:id  —— 挂在 apiV1 + authRequired 下
func (h *Handler) GetImage(c *gin.Context) {
	obj, err := h.storage.Get(c.Request.Context(), c.Param("id")) // 内部 minio.GetObject
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", info.ContentType)
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	// 用 c.DataFromReader 而不是先 ReadAll：避免把整图读进内存
	c.DataFromReader(http.StatusOK, info.Size, info.ContentType, obj, nil)
}
```

nginx 侧不需要新增 location：`/api/` 已经整体反代到 `127.0.0.1:8080`（`deploy/nginx.conf:52-60`），代理路径自动同源。

### 2.4 如果确实要用预签名（例如将来图片量级变大）

- 有效期取短（展示类 5~15 分钟），绝不用满 7 天。
- 客户端每次进入页面重新取一次 URL，不要长期缓存到 localStorage。
- 必须给 MinIO 配 bucket CORS（`mc cors set`），且 9000 端口要能被浏览器访问到——这会把对象存储变成第二个公网入口，与本仓库「单镜像 + 80 端口」的部署模型冲突。

来源：
- minio-go `pkg/signer/request-signature-v4.go` `getCanonicalHeaders`（host 必签）：https://github.com/minio/minio-go/blob/master/pkg/signer/request-signature-v4.go
- `mc cors set`：https://docs.min.io/aistor/reference/cli/mc-cors/mc-cors-set/
- 预签名有效期上限：https://pkg.go.dev/github.com/minio/minio-go/v7@v7.3.0#Client.PresignedGetObject
- MDN CORS（预检与简单请求）：https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS

---

## 3. 服务端中转 vs 客户端直传（presigned PUT）

### 3.1 对照

| 维度 | 服务端中转（浏览器 → Go → MinIO） | 客户端直传（Go 签 presigned PUT，浏览器 → MinIO） |
| --- | --- | --- |
| Go 侧代码量 | 一个 multipart handler + 一次 `PutObject` | 两个接口：`POST /uploads/sign` 签 URL、`POST /uploads/confirm` 落库 |
| 前端代码量 | `FormData` + `fetch`，几乎零成本 | 需要处理 CORS 预检、PUT 的 `Content-Type` 必须与签名时一致、失败重试 |
| 带宽 | 双倍（客户端上行到 Go，Go 再上行到 MinIO） | 单倍（直连对象存储） |
| 服务端能否校验真实内容 | **能**（可以嗅探前 512 字节、可以解码图片） | **基本不能**（只能靠 presigned POST policy 的 `content-length-range` / `Content-Type` 条件约束，签名一旦发出，内容由客户端决定） |
| 体积限制 | `http.MaxBytesReader` + `file.Size`，服务端说了算 | 靠签名时的 policy 条件，服务端只能事后校验 `StatObject` 的大小 |
| CORS | 不需要 | 必须配 bucket CORS |
| 孤儿对象 | 无（写完才落库） | 有（签了名但用户取消上传，桶里留空对象，需要生命周期规则清理） |
| 内存/临时盘 | Go 进程承担（见第 6 节） | Go 进程几乎不承担 |
| 与现有架构契合度 | 高（同源、复用 JWT 中间件、复用统一响应格式） | 低（多一个公网入口） |

### 3.2 推荐：服务端中转

理由，按权重排序：

1. **安全边界清晰**。文件先到 Go，可以在落盘前就判定真实类型（`http.DetectContentType` 只看前 512 字节，见第 5 节），非法内容根本不会进对象存储；直传方案里 MinIO 先收下了，服务端再删是事后补救，中间存在可访问窗口。
2. **部署模型不变**。仍然是「80 端口一个入口」，不需要把 9000 暴露到公网，也不需要维护 bucket CORS。
3. **规模匹配**。招新报名配图是单张几 MB 的量级，双倍带宽在 80 端口后面的单机 nginx 上完全不是瓶颈；直传省带宽的收益要到「用户批量上传几百 MB」才显现。
4. **错误处理统一**。超限、类型非法都能返回 `{code, message, data}` 业务错误；直传方案里浏览器拿到的是 MinIO 的 XML 错误（`EntityTooLarge`），前端要单独解析。

**什么时候应该反过来选直传**：单文件 > 50MB、并发上传量大、或对象存储与业务服务不在同一网络区域。届时改造点只有「加两个签名接口 + 配 bucket CORS」，不会推翻数据模型。

来源：
- 预签名 PUT：https://pkg.go.dev/github.com/minio/minio-go/v7@v7.3.0#Client.PresignedPutObject
- `PresignHeader` 的注释明确写了「使用该 URL 的请求必须带上完全相同的 header，否则签名校验失败」：https://github.com/minio/minio-go/blob/master/api-presigned.go

---

## 4. MinIO 容器：镜像、命令、端口、健康检查、初始化、数据卷

### 4.1 重要前提：上游已转向 AIStor

`github.com/minio/minio` 的 README 首行就是 **"THIS REPOSITORY IS NO LONGER MAINTAINED."**，并说明社区版现在**只以源码形式分发**（不再提供预编译二进制），推荐 `go install github.com/minio/minio@latest`（最低 go1.24）；`dl.min.io` 上的历史二进制不再维护。官方现行容器文档给的是 AIStor 镜像 `quay.io/minio/aistor/minio`，而 **AIStor 需要 license**（免费层可申请，但仍是许可约束）。

这带来一个必须写进决策的现实：**本仓库引 minio-go 客户端没有许可问题，但自建服务端有**。三条路：
- (a) 用 AIStor Free 镜像 + 免费 license（合规、但多一个 license 文件要挂载）；
- (b) 用仍在分发的社区版镜像标签（历史 `minio/minio`），接受「不再维护」的风险；
- (c) 换成其他 S3 兼容实现（Garage、SeaweedFS、Ceph RGW、云厂商 OSS/S3）。**因为代码只依赖 S3 API，切换只改 endpoint/凭证，业务代码不动**——这是建议保留的退路。

**证据缺口（如实标注）**：本次调研尝试核对 Docker Hub 上的 `minio/minio` 镜像页与 tag 列表，`https://hub.docker.com/r/minio/minio` 返回 404、`https://hub.docker.com/v2/repositories/minio/minio/tags` 返回 `{"message":"object not found"}`；Docker Hub 搜索 `minio` 时首条是标记为 "HARDENED IMAGE / RECOMMENDED" 的 MinIO 条目（AGPLv3 描述），但未取到其确切 namespace。**因此「`minio/minio` 是官方镜像名」这一说法本次没有取得一手证据**，只能作为社区历史习惯使用；官方文档当前只背书 `quay.io/minio/aistor/minio`。

### 4.2 官方（AIStor）启动命令

```bash
docker pull quay.io/minio/aistor/minio

docker run -dt \
  -p 9000:9000 -p 9001:9001 \
  -v $HOME/minio/data:/mnt/data \
  -v $HOME/minio/minio.license:/minio.license \
  -v $HOME/minio/certs:/etc/minio/certs \
  --name "aistor-server" \
  quay.io/minio/aistor/minio:latest minio server /mnt/data \
  --license /minio.license \
  --certs-dir /etc/minio/certs
```

社区版（历史写法，仅作对照；官方文档已不再给出）：

```bash
docker run -d --name minio \
  -p 9000:9000 -p 9001:9001 \
  -v minio_data:/data \
  -e MINIO_ROOT_USER=... -e MINIO_ROOT_PASSWORD=... \
  minio/minio server /data --console-address ":9001"
```

### 4.3 端口

| 端口 | 用途 |
| --- | --- |
| **9000** | S3 API（minio-go 连这个） |
| **9001** | 控制台 Web UI |

README 明确写了「MinIO runs console on random port by default, if you wish to choose a specific port use `--console-address`」——**不传 `--console-address` 控制台端口是随机的**，编排里必须显式固定。

### 4.4 默认凭据

`MINIO_ROOT_USER` 省略时默认 `minioadmin`；文档原文 "**NEVER** use the default credentials in production environments."，并要求 root 用户用唯一、长、随机的值。这两个值对应 minio-go 的 `credentials.NewStaticV4(accessKey, secretKey, "")`。

### 4.5 健康检查端点

| 端点 | 语义 | 返回码 |
| --- | --- | --- |
| `GET /minio/health/live` | 存活：进程在线且可服务 | 200 OK / 429 Too Many Requests |
| `GET /minio/health/ready` | 就绪：节点已初始化、可接受请求 | 200 / 429 / 503（如 etcd 不可达） |
| `/minio/health/cluster` 系列 | 集群级读写 quorum | 见文档 |

三者都**不需要鉴权**。容器 healthcheck 建议用 `/minio/health/live`（存活）或 `/minio/health/ready`（就绪）。注意文档的提醒：探针失败只能说明「当前这台机器连不上它」，不能单独用来判定服务端已下线。

compose 写法（与本仓库现有 `healthcheck` 风格一致）：

```yaml
healthcheck:
  test: ["CMD", "curl", "-fsS", "http://127.0.0.1:9000/minio/health/live"]
  interval: 10s
  timeout: 5s
  retries: 6
  start_period: 10s
```

（若镜像里没有 `curl`，改用 `mc ready local` 或 `wget -q --spider`；具体以镜像内容为准，不要照抄。）

### 4.6 bucket 初始化

三条路，**推荐第 3 条**：

1. **`mc` 命令行（一次性/运维用）**
   ```bash
   mc alias set local http://localhost:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
   mc mb local/iot-pillot
   # 公开 bucket（本方案不采用，见第 5 节）：
   mc anonymous set download local/iot-pillot
   ```
   `mc anonymous set` 的 POLICY 取值：`none | private | public | download | upload`。
2. **compose 里加一个 init 容器**（`minio/mc` 镜像 + entrypoint 脚本），缺点是又多一个镜像依赖。
3. **应用启动时 `EnsureBucket`**（第 1.3 节的代码）。与现有 `Config.validate` 失败即拒绝启动的风格一致，多实例并发也幂等，不需要额外容器。

**生命周期（可选）**：给「上传了但业务没落库」的孤儿对象设过期规则，`mc ilm rule add --expire-days 90 myaistor/mydata`（还支持 `--prefix` 只作用于某个前缀）。

### 4.7 数据卷：必须

对象数据落在 `/data`（AIStor 示例是 `/mnt/data`）。**不挂卷 = 容器重建即丢图**。本仓库现状：

- `docker-compose.yml` 的 `volumes:` 目前只有 `postgres_data`（文件末尾），加 MinIO 时要同时加 service 与 named volume。
- `.github/workflows/deploy.yml` 的生产 `docker run` **没有任何 `-v`**，只注入 `-e IOT_PILOT_*`。这意味着：MinIO 若也这样跑，图片在每次发布后全部丢失。必须给 MinIO 单独挂宿主目录或 named volume，并在部署脚本里保证该卷不被 `docker rm` 波及。

来源：
- 仓库状态与「只发源码」：https://github.com/minio/minio
- AIStor 容器安装（镜像、run 命令、9001 控制台、默认凭据）：https://docs.min.io/aistor/installation/container/install/
- 健康检查端点：https://docs.min.io/community/minio-object-store/operations/monitoring/healthcheck-probe.html
- root 凭据与默认值：https://docs.min.io/aistor/reference/aistor-server/settings/root-credentials/
- `mc mb`：https://docs.min.io/community/minio-object-store/reference/minio-mc/mc-mb.html
- `mc anonymous set`：https://docs.min.io/community/minio-object-store/reference/minio-mc/mc-anonymous-set.html
- `mc ilm rule add`：https://docs.min.io/community/minio-object-store/reference/minio-mc/mc-ilm-rule-add.html
- 对象生命周期管理：https://docs.min.io/aistor/administration/object-lifecycle-management/

---

## 5. 图片安全

### 5.1 内容类型白名单：不能只看 `Content-Type`

multipart 里的 `Content-Type` 完全由客户端填写，可以随便伪造（`Content-Type: image/png` 后面跟一段 HTML 或 SVG 脚本）。正确做法是**嗅探实际字节**：

```go
func sniffImage(f multipart.File) (string, string, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil { // 必须回绕，否则后续上传读到空
		return "", "", err
	}
	detected := http.DetectContentType(head[:n])

	switch detected {
	case "image/jpeg":
		return detected, ".jpg", nil
	case "image/png":
		return detected, ".png", nil
	case "image/gif":
		return detected, ".gif", nil
	case "image/webp":
		return detected, ".webp", nil
	default:
		return "", "", errUnsupportedType
	}
}
```

要点：
- `http.DetectContentType` 按 WHATWG mimesniff 算法**只看前 512 字节**，无法判定时返回 `application/octet-stream`。
- **不要白名单 SVG**：SVG 是 XML，可以内嵌 `<script>`，直接以 `image/svg+xml` 返回会在同源下执行脚本（存储型 XSS）。若要支持，必须单独做 XML 清洗或强制 `Content-Disposition: attachment`。
- 更严格可再加一层「真解码」：用 `image.Decode`（`image/jpeg`、`image/png` 注册后）或 `golang.org/x/image/webp` 解一遍，能拦住「头部像 PNG、后面是垃圾」的构造文件。注意 `github.com/gabriel-vasile/mimetype` 目前在 `apps/api/go.mod` 里是**间接依赖**（gin 依赖它），直接 import 会把它变成直接依赖，需要走一次依赖评审。
- 白名单判定要用**嗅探结果**决定存储的 `ContentType` 与扩展名，**不要沿用客户端传的**。

### 5.2 大小限制

三层，缺一不可：

1. **nginx**（外层闸门，防未读 body 就耗尽连接）：`client_max_body_size`。
2. **Go 入口**：`c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)`，超限时读 body 返回 `*http.MaxBytesError`（`Error()` 为 "http: request body too large"），可以精确映射成 413 + 业务错误。
3. **业务层**：`fileHeader.Size` 再校验一次（`ParseMultipartForm` 已经知道每个 part 的大小，能挡住「多文件合计超限」以外的单文件超限；多文件场景要自己累加）。

三层的关系要理清：nginx 的限制**必须大于**应用层限制（例如 nginx 6m、应用 5MiB），否则用户拿到的是 nginx 的 413 纯 HTML，而不是 `{code, message}`；但 nginx 又必须有值，否则 1m 默认值会先于应用层拦掉。

### 5.3 随机对象名

- **绝不用用户上传的文件名做 key**：路径穿越（`../`）、同名覆盖、非 ASCII 与超长名、以及信息泄漏（文件名里常带真实姓名/学号）。
- 用 `crypto/rand` 生成 16 字节再 hex/base32，或 `github.com/google/uuid`（已在 `go.mod` 间接依赖里）。
- 扩展名**从嗅探结果推导**，不从客户端取。
- 建议按前缀分目录，便于生命周期与权限收敛：`images/<userID>/<yyyy>/<mm>/<random>.<ext>`。
- 业务表里存 `object_key` 而不是完整 URL（endpoint 换域名时不用刷数据）。

```go
func newObjectKey(userID int64, ext string, now time.Time) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("images/%d/%s/%s%s", userID, now.Format("2006/01"), hex.EncodeToString(buf), ext), nil
}
```

### 5.4 私有 bucket + 预签名 vs 公开 bucket

| | 私有 bucket + 预签名（或代理） | 公开 bucket（`mc anonymous set download`） |
| --- | --- | --- |
| 未登录能否访问 | 否 | **能**，任何人拿到 key 就能读 |
| URL 是否会泄漏 | 预签名 URL 泄漏即失效前可读；代理路径不产生外部 URL | key 一旦出现在日志/截图/Referer 里就永久公开 |
| 撤销能力 | 预签名：删对象或轮换密钥；代理：改代码即可 | 只能改 bucket policy，已扩散的 key 仍可被猜 |
| CORS | 直连需配 bucket CORS | 同样需要（除非同源反代） |
| 缓存 | 预签名 URL 每次不同 → CDN/浏览器缓存基本失效；代理可设 `Cache-Control` | 可长缓存 |
| 适用 | **登录用户私有内容（本项目）** | 公开素材、官网图片 |

**本仓库结论**：bucket 保持私有（默认即私有，不要执行 `mc anonymous set`），读取走第 2.3 节的 Go 代理；handler 里做归属校验（上传者本人或 admin）。若将来要接 CDN，再把「公开可缓存」的图迁到独立公开 bucket，两者不要混用。

### 5.5 其他必做项

- 响应头 `X-Content-Type-Options: nosniff`（防止浏览器把图片当 HTML 嗅探执行）。
- 上传接口按用户限流（现有 Redis 已用于验证码/限流计数，可复用）。
- 删除对象时**先删库记录再删对象**或反过来都要有幂等补偿；`RemoveObject` 对不存在的 key 不报错（S3 语义），可以安全重试。
- 日志里不要打完整对象 URL（若走预签名，URL 即凭证）。

来源：
- `http.DetectContentType`（只看前 512 字节，mimesniff）：https://pkg.go.dev/net/http#DetectContentType
- `http.MaxBytesReader` / `http.MaxBytesError`：https://pkg.go.dev/net/http#MaxBytesReader
- 公开 bucket 策略：https://docs.min.io/community/minio-object-store/reference/minio-mc/mc-anonymous-set.html
- MDN `X-Content-Type-Options`：https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/X-Content-Type-Options

---

## 6. Gin 接收 multipart：内存、临时磁盘与 nginx 同步

### 6.1 Gin 的默认值：32MB，但它是「内存阈值」不是「上传上限」

`gin@v1.10.0`（本仓库版本）源码：

```go
// gin.go:24
const defaultMultipartMemory = 32 << 20 // 32 MB

// gin.go:206（New() 里）
MaxMultipartMemory: defaultMultipartMemory,
```

`Context.FormFile` 的实现（`context.go:589-601`）：

```go
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(c.engine.MaxMultipartMemory); err != nil {
			return nil, err
		}
	}
	f, fh, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	f.Close()
	return fh, err
}
```

即：`c.FormFile` / `c.MultipartForm` 会把 `Engine.MaxMultipartMemory` 原样传给 `http.Request.ParseMultipartForm`。想改就在 `gin.New()`/`gin.Default()` 之后写 `r.MaxMultipartMemory = N`（本仓库在 `apps/api/internal/handler/router.go:56` 用 `gin.Default()`，可在其后加一行）。

### 6.2 `ParseMultipartForm` 到底怎么用内存

`net/http` 的文档注释原文：

> ParseMultipartForm parses a request body as multipart/form-data. **The whole request body is parsed and up to a total of maxMemory bytes of its file parts are stored in memory, with the remainder stored on disk in temporary files.**

`net/http` 自己的默认值也是 32MB：`request.go:36 defaultMaxMemory = 32 << 20 // 32 MB`（`ParseForm` 内部调用 `ParseMultipartForm(defaultMaxMemory)` 时用）。

`mime/multipart.ReadForm` 的注释更精确：

> It stores up to **maxMemory bytes + 10MB (reserved for non-file parts)** in memory. **File parts which can't be stored in memory will be stored on disk in temporary files.** It returns ErrMessageTooLarge if all non-file parts can't be stored in memory.

推论（关键，容易踩）：

- **`MaxMultipartMemory = 32MB` 时，一个 200MB 的文件不会报错，而是整个落进 `os.TempDir()` 的临时文件。** 也就是说这个值只控制「多少进内存」，**完全不限制上传大小**。
- 非文件字段（普通 form 字段）全部进内存，且超限报 `ErrMessageTooLarge`——所以不要让客户端往 form 字段里塞大段 base64。
- 临时文件的位置由 `os.TempDir()`（`$TMPDIR`/`/tmp`）决定；容器里 `/tmp` 通常是可写层，**大文件会把容器写满**，因此应用层的 `MaxBytesReader` 是必须的，不是可选的优化。
- 临时文件何时清理：`net/http` 在请求收尾时会调用 `req.MultipartForm.RemoveAll()`（`server.go:1682-1683`），正常结束的请求会自动删除。但如果 handler 把 `*multipart.FileHeader` 或已打开的 `multipart.File` 传到请求生命周期之外（goroutine、异步任务），就必须自己 `defer form.RemoveAll()`，否则临时文件会一直留在盘上。
- `c.FormFile` 内部已经 `f.Close()`，你拿到的是 `*multipart.FileHeader`，要读内容需再 `fh.Open()`。

### 6.3 推荐的上传 handler 骨架

```go
const (
	maxImageBytes = 5 << 20  // 5 MiB
	maxFormSlack  = 512 << 10 // 给 form 字段与 multipart 边界留余量
)

func (h *Handler) UploadImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageBytes+maxFormSlack)

	fh, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, apiErr(4130, "图片超过 5MB 限制"))
			return
		}
		c.JSON(http.StatusBadRequest, apiErr(4001, "缺少文件字段 file"))
		return
	}
	if fh.Size > maxImageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, apiErr(4130, "图片超过 5MB 限制"))
		return
	}

	f, err := fh.Open()
	if err != nil { /* 500 */ }
	defer f.Close()

	contentType, ext, err := sniffImage(f) // 见 5.1，内部已 Seek 回 0
	if err != nil {
		c.JSON(http.StatusUnsupportedMediaType, apiErr(4150, "仅支持 jpg/png/gif/webp"))
		return
	}

	key, err := newObjectKey(userIDFromCtx(c), ext, time.Now())
	if err != nil { /* 500 */ }
	if err := h.storage.Put(c.Request.Context(), key, f, fh.Size, contentType); err != nil {
		c.JSON(http.StatusInternalServerError, apiErr(5000, "图片保存失败"))
		return
	}
	// 再落业务表，最后返回 {id, url:"/api/v1/images/"+id}
}
```

注意 `fh.Size` 来自 multipart 头里的 `Content-Length`（客户端声明值），所以**它只能当"早退"用，不能当唯一防线**；真正的防线是 `MaxBytesReader` 与实际读取的字节数。

### 6.4 nginx 的 `client_max_body_size`：默认 1m，必须同步改

nginx 官方文档原文：

> **Syntax:** `client_max_body_size size;`
> **Default:** `client_max_body_size 1m;`
> **Context:** http, server, location
> Sets the maximum allowed size of the client request body. If the size in a request exceeds the configured value, **the 413 (Request Entity Too Large) error is returned to the client. Please be aware that browsers cannot correctly display this error.** Setting size to 0 disables checking of client request body size.

本仓库 `deploy/nginx.conf` **全文没有这条指令**，所以实际生效就是 **1m**。后果：用户传一张 2MB 的手机照片，请求在 nginx 层就被 413 拦掉，Go 根本没收到；浏览器显示的是一个"无法正确显示"的空白错误页，前端拿不到任何业务错误码。

为什么必须"同步"调整（而不是只改一边）：

1. **nginx 默认值 1m < 图片常见大小**，不改就必然失败。
2. **nginx 的限制必须略大于应用层限制**，否则超限请求由 nginx 拦截、返回 HTML，前端无法按 `{code, message}` 展示，也拿不到 413 的业务语义。
3. **两边都必须在**：只改 nginx，恶意请求仍可让 Go 把整个大 body 落到 `/tmp`；只改 Go，请求根本到不了 Go。
4. 建议值：应用层 5 MiB，nginx 设 `6m`（留出 multipart 边界与 form 字段余量）。

```nginx
http {
    # 上传接口的业务上限是 5MiB，这里留余量到 6m；
    # 不能设 0（=不限制），否则大文件会一路落到 Go 的临时目录。
    client_max_body_size 6m;

    server {
        ...
        location /api/ {
            proxy_pass http://127.0.0.1:8080;
            proxy_http_version 1.1;
            proxy_set_header Host              $host;
            proxy_set_header X-Real-IP         $remote_addr;
            proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            # 默认 60s 对慢速手机上传偏短，按需放宽
            proxy_read_timeout 60s;
        }
    }
}
```

**相关但容易忽略的一点：`proxy_request_buffering` 默认 `on`。** 官方文档："When buffering is enabled, the entire request body is read from the client before sending the request to a proxied server." 也就是说上传时 nginx 会先把整个 body 收完（超过内存缓冲的部分落 nginx 临时文件）再转给 Go。对 5MB 图片无所谓；若将来放宽到几十 MB，可考虑对该 location 设 `proxy_request_buffering off`，让 Go 直接流式接收。

### 6.5 与现有路由的挂载点

- 路由：在 `apps/api/internal/handler/router.go` 的 `apiV1`（`:116`）下挂 `POST /api/v1/uploads/images`、`GET /api/v1/images/:id`，用已有的 `authRequired`（`:110`）包住——招新场景要求「登录用户上传」，与现有鉴权模型一致。
- 配置：`apps/api/internal/config/config.go` 加 `MinioConfig{Endpoint, AccessKey, SecretKey, Bucket, Secure, PresignExpire}` 字段（结构体在 `:15-25`），在 `setDefaults()`（`:339`）给 endpoint/bucket 默认值，在 `validate()`（`:328`）要求 access/secret 非空（与 JWT secret 同样的理由：缺凭证时不该静默降级）。env 前缀已由 `viper.SetEnvPrefix("IOT_PILOT")`（`:154`）提供，对应 `IOT_PILOT_MINIO_ENDPOINT` 等。
- 装配：`apps/api/cmd/wire.go` 加 `storage.NewClient` 的 provider，由 wire 注入 handler。
- 迁移：业务表（图片归属）走 `apps/api/migrations/NNNN_*.up.sql` + `migrations/embed.go`，不要手改库。
- 编排：`docker-compose.yml` 加 `minio` service（9000/9001 + named volume + healthcheck）与 `volumes.minio_data`；生产 `deploy.yml` 的 `docker run` 必须补 `-v`。

来源：
- Gin `MaxMultipartMemory` 与 `FormFile`：https://pkg.go.dev/github.com/gin-gonic/gin#Engine 、https://pkg.go.dev/github.com/gin-gonic/gin#Context.FormFile （本仓库 `gin@v1.10.0` 源码 `gin.go:24,206`、`context.go:589-601`）
- `Request.ParseMultipartForm`：https://pkg.go.dev/net/http#Request.ParseMultipartForm
- `multipart.Reader.ReadForm`：https://pkg.go.dev/mime/multipart#Reader.ReadForm
- `client_max_body_size`：https://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size
- `proxy_request_buffering`：https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_request_buffering

---

## 7. 落地检查清单

按依赖顺序，勾完即可上线：

- [ ] `apps/api` 执行 `go get github.com/minio/minio-go/v7@v7.3.0`（**不要**不带版本号）
- [ ] `config.go`：`MinioConfig` 字段 + `setDefaults` + `validate`（access/secret 缺失拒绝启动）
- [ ] `storage` 包：`NewClient` / `EnsureBucket` / `Put` / `Get` / `Remove`
- [ ] 启动时 `EnsureBucket`（幂等，多实例安全）
- [ ] handler：`MaxBytesReader` + `fh.Size` + `DetectContentType` 白名单 + 随机 key
- [ ] 路由挂在 `apiV1` + `authRequired` 下
- [ ] 读取走 Go 代理（私有 bucket），响应带 `nosniff` 与 `Cache-Control`
- [ ] `deploy/nginx.conf`：`client_max_body_size 6m`（默认 1m 必然失败）
- [ ] `docker-compose.yml`：`minio` service + `minio_data` volume + healthcheck `/minio/health/live`
- [ ] `deploy.yml`：给 MinIO 容器补 `-v`，否则每次发布丢图
- [ ] `MINIO_ROOT_USER/PASSWORD` 用随机值（默认 `minioadmin` 明确禁止用于生产）
- [ ] 迁移文件：图片归属表
- [ ] 决策记录：MinIO 社区版「不再维护」+ AIStor 需 license 的风险，写进 `docs/adr/`

---

## 附：一手来源汇总

| 主题 | URL |
| --- | --- |
| minio-go 版本列表 | https://proxy.golang.org/github.com/minio/minio-go/v7/@v/list |
| minio-go 最新版 | https://proxy.golang.org/github.com/minio/minio-go/v7/@latest |
| minio-go v7.3.0 的 go 指令 | https://proxy.golang.org/github.com/minio/minio-go/v7/@v/v7.3.0.mod |
| minio-go 包文档 | https://pkg.go.dev/github.com/minio/minio-go/v7 |
| minio-go 仓库 | https://github.com/minio/minio-go |
| MinIO 服务端仓库（不再维护声明） | https://github.com/minio/minio |
| AIStor 容器安装 | https://docs.min.io/aistor/installation/container/install/ |
| 健康检查端点 | https://docs.min.io/community/minio-object-store/operations/monitoring/healthcheck-probe.html |
| root 凭据默认值 | https://docs.min.io/aistor/reference/aistor-server/settings/root-credentials/ |
| `mc mb` | https://docs.min.io/community/minio-object-store/reference/minio-mc/mc-mb.html |
| `mc anonymous set` | https://docs.min.io/community/minio-object-store/reference/minio-mc/mc-anonymous-set.html |
| `mc cors set` | https://docs.min.io/aistor/reference/cli/mc-cors/mc-cors-set/ |
| `mc ilm rule add` | https://docs.min.io/community/minio-object-store/reference/minio-mc/mc-ilm-rule-add.html |
| Gin `Context.FormFile` | https://pkg.go.dev/github.com/gin-gonic/gin#Context.FormFile |
| `net/http` `ParseMultipartForm` | https://pkg.go.dev/net/http#Request.ParseMultipartForm |
| `net/http` `DetectContentType` | https://pkg.go.dev/net/http#DetectContentType |
| `net/http` `MaxBytesReader` | https://pkg.go.dev/net/http#MaxBytesReader |
| `mime/multipart` `ReadForm` | https://pkg.go.dev/mime/multipart#Reader.ReadForm |
| nginx `client_max_body_size` | https://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size |
| nginx `proxy_request_buffering` | https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_request_buffering |
| MDN CORS | https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS |
