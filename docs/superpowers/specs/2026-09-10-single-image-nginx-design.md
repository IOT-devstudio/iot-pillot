# 单镜像 nginx + Go 部署设计

- 日期:2026-09-10
- 状态:待评审
- 相关:PR #16(CD 触发链路)、PR #17(缓存权限)、PR #18(配置缺陷)、Issue #19(Deploy 已停用)

## Context

当前仓库只部署后端。`apps/web`(Vue 3 + Element Plus + Vite)没有 Dockerfile、没有 nginx 配置、`dist/` 不入库,完全不参与部署。后端容器直接占用宿主 80 端口。

目标:把前后端合并为**单个镜像**,nginx 作为入口托管前端静态产物并把 API 反代给同镜像内的 Go 进程。

选择单镜像而非双容器,是为了保住三件事:

1. **前后端版本永远对齐** —— 一个镜像一个版本。仓库有 `packages/shared-types` 共享 DTO,契约错配会造成难查的运行时错误。
2. **现有 CD 形状不变** —— `docker save` → scp → `docker load` → `docker run` 这条链路刚在 PR #16/#17 建成并验证,双容器需要引入 compose 编排,等于重写。
3. **回滚是原子单位** —— 已实测三次的「抓 image ID → 健康检查失败即回滚」滚的是一个东西。

已知代价,明确接受:

- 改一行前端 CSS 也要重建整个镜像、全量传输、重启后端
- 容器内两个进程,生命周期要自己管
- 镜像从约 20MB 涨到约 45MB

选 nginx 而非把 dist `embed` 进 Go 二进制,理由是希望缓存策略、压缩、重写规则通过改配置文件调整,而不是改 Go 代码重新编译。

## 关键决策

### D1:entrypoint 用 `wait -n`,不用「nginx 前台 + api 后台」

常见写法是:

```sh
api &
nginx -g "daemon off;"
```

**这个写法必须避免。** `api` 退出后 nginx 仍在前台运行,容器状态是 running,`--restart unless-stopped` 永不触发,站点静默返回 502 直到人工介入。tini 不解决这个问题——它只回收僵尸进程、向自己的直接子进程转发信号,不会因为后台孙子进程退出而让容器退出。

2026-09-10 当天 Go 因缺少配置崩溃循环三轮。若采用上述写法,现象会从「容器反复重启、日志明显」退化为「站点静默 502、容器显示健康」。

采用 `wait -n`:任一进程退出 → PID 1 退出 → 容器退出 → docker 重启策略把两个一起拉起。

代价:busybox `ash` 不支持 `wait -n`,需 `apk add bash`(约 2MB)。

### D2:nginx 必须显式路由 `/health`

后端健康检查挂在**根级** `/health`,不在 `/api/` 下。若 nginx 只配 `location /` 与 `location /api/`,`/health` 会落入 `try_files ... /index.html`,**返回 200 与 SPA 的 HTML**。

后果:`deploy.yml` 的 `curl -f .../health` 在 Go 完全死亡时依然拿到 200,自动回滚失效,坏版本被判定成功并推进 `deployed` tag。

必须有 `location = /health` 显式反代。

### D3:运行时基底用 `alpine + apk add nginx tini`

比 `nginx:alpine` 更小(约 45MB vs 约 70MB)。传输量对本项目重要——当初放弃 GHCR 改用 `docker save + scp` 的原因就是服务器到 registry 的网络慢。

### D4:PG / Redis 通过用户自定义网络接入(推荐)

服务器上建 `iot-pilot-net`,Postgres 与 Redis 作为长驻容器接入,应用容器 `docker run --network iot-pilot-net`,靠容器名寻址(`IOT_PILOT_DB_HOST=postgres`)。

CD 形状完全不变,服务器上无需引入 compose。

**备选**:改用 compose 编排三个服务。代价是 deploy.yml 要从 `docker run` 改成传 compose 文件 + `docker compose up -d`,与 PR #16 建立的链路冲突。注意 `--network host` 与 compose 内部网络互斥,不能混用。

## 镜像结构

三阶段构建,context 为**仓库根**。

```dockerfile
# ---- 阶段 1：前端 ----
FROM node:20-alpine AS frontend
RUN corepack enable && corepack prepare pnpm@8 --activate
WORKDIR /src
# 先只拷 manifest，让依赖层在源码变动时可复用。
# 注意：仓库根没有 package.json（workspace 靠 pnpm-workspace.yaml 定义），
# 不要在这里 COPY 一个不存在的文件。apps/api 也不是 workspace 成员（无 package.json）。
COPY pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json              apps/web/
COPY packages/shared-types/package.json packages/shared-types/
RUN pnpm install --frozen-lockfile
COPY apps/web        apps/web
COPY packages        packages
RUN pnpm --filter @iot-pillot/web build     # → /src/apps/web/dist（vite 未覆盖 outDir，用默认值）

# ---- 阶段 2：后端 ----
FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api .
# 必须是 ./cmd（包），不是 cmd/main.go：
# cmd/ 下还有 wire.go / wire_gen.go，单文件编译会 undefined: InitializeApp
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd

# ---- 阶段 3：运行时 ----
FROM alpine:3.20
RUN apk add --no-cache nginx tini bash ca-certificates tzdata \
 && mkdir -p /run/nginx
COPY deploy/nginx.conf   /etc/nginx/nginx.conf
COPY deploy/entrypoint.sh /entrypoint.sh
COPY --from=frontend /src/apps/web/dist /var/www/html
COPY --from=backend  /out/api           /usr/local/bin/api
RUN chmod +x /entrypoint.sh
EXPOSE 80
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/entrypoint.sh"]
```

Go 版本必须是 **1.25**(`apps/api/go.mod` 要求),前端必须走 **pnpm**(仓库只有 `pnpm-lock.yaml`,`npm ci` 会失败)。

## entrypoint

`deploy/entrypoint.sh`:

```bash
#!/bin/bash
set -euo pipefail

term() {
  nginx -s quit 2>/dev/null || true
  kill -TERM "$API_PID" 2>/dev/null || true
}
trap term SIGTERM SIGINT

/usr/local/bin/api & API_PID=$!
nginx -g 'daemon off;' &

wait -n                 # 任一进程退出即返回
EXIT=$?
term
wait || true
exit $EXIT
```

## nginx 配置

`deploy/nginx.conf` 是**完整配置文件**(替换 `/etc/nginx/nginx.conf`),必须含 `events` 与 `http` 块。

```nginx
user  nginx;
worker_processes auto;
error_log /dev/stderr warn;
pid /run/nginx/nginx.pid;

events { worker_connections 1024; }

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;
    access_log    /dev/stdout;
    sendfile      on;
    keepalive_timeout 65;

    gzip on;
    gzip_min_length 1024;
    gzip_types text/css application/javascript application/json image/svg+xml;

    server {
        listen 80;
        server_name _;
        root  /var/www/html;
        index index.html;

        # 运维探针：必须显式反代，否则会被 SPA fallback 吃掉并永远返回 200（见 D2）
        location = /health {
            proxy_pass http://127.0.0.1:8080;
            access_log off;
        }

        # 业务 API。proxy_pass 不带尾斜杠 → 保留完整 URI，
        # /api/v1/login 原样送达 Go 的 /api/v1/login
        location /api/ {
            proxy_pass http://127.0.0.1:8080;
            proxy_set_header Host              $host;
            proxy_set_header X-Real-IP         $remote_addr;
            proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        # Vite 产物带内容指纹，可长缓存
        location /assets/ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }

        location = /index.html { add_header Cache-Control "no-cache"; }

        location / { try_files $uri $uri/ /index.html; }
    }
}
```

`X-Forwarded-Proto` 当前无消费方,但将来在前面加 TLS 层时后端需要它判断真实协议,现在带上零成本。

## 端口布局

```
宿主 :80  ──►  容器 :80  nginx  ──►  127.0.0.1:8080  Go
```

Go 沿用其默认端口 8080(`viper.SetDefault("port", 8080)`),无需额外配置。

## CD 连带改动

`.github/workflows/deploy.yml`:

| 位置 | 现状 | 改为 |
|---|---|---|
| `context` | `./apps/api` | `.` |
| `file` | `./apps/api/Dockerfile` | `./Dockerfile` |
| `CONTAINER_PORT` | `8080` | `80` |
| `docker run` | 无网络参数 | 加 `--network iot-pilot-net` 与 `IOT_PILOT_*` 环境变量 |
| gate 路径判定 | 仅 `apps/api/` | 见下 |

gate 的 `git diff` 路径必须覆盖所有影响镜像的位置,漏一个就会导致对应改动被静默判定为「无需部署」:

```
apps/api/  apps/web/  packages/  pnpm-lock.yaml  pnpm-workspace.yaml  Dockerfile  deploy/
```

文件移动:

- `apps/api/Dockerfile` → 仓库根 `Dockerfile`(它构建的已是整个应用,不再只是后端)
- **新增根 `.dockerignore`**,必须排除 `node_modules`(根与各 package),否则 build context 达数百 MB
- **删除 `apps/api/.dockerignore`** —— dockerignore 只在 context 根被读取,context 上移后它会变成看似有用实则无人读取的死文件,内容并入根文件

`CLAUDE.md` 的目录结构需同步新增 `deploy/` 与根 `Dockerfile`。

## 冒烟测试

放在 `deploy.yml` 的 `build-and-ship`,构建之后、scp 之前。

**不放 ci.yml 的理由**:`auto-merge.yml` 不等 CI 就合并,放在 CI 里的检查拦不住坏合并,只能事后告知。放在 deploy 里坏镜像到不了服务器。

job 增加 service container:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    env: { POSTGRES_USER: iot_pillot, POSTGRES_PASSWORD: iot_pillot, POSTGRES_DB: iot_pillot }
    options: >-
      --health-cmd "pg_isready -U iot_pillot -d iot_pillot"
      --health-interval 5s --health-retries 5
  redis:
    image: redis:7-alpine
    options: --health-cmd "redis-cli ping" --health-interval 5s --health-retries 5
```

镜像以 `--network host` 启动,`IOT_PILOT_DB_HOST=127.0.0.1`、`IOT_PILOT_JWT_SECRET=ci-only-secret`。

断言:

| 断言 | 挡住的故障 |
|---|---|
| `GET /health` → 200 | nginx 启动、反代通、Go 启动、DB+Redis 连通、配置合法 |
| `GET /` 含 `<div id="app"` | 静态文件挂载路径错误 |
| `GET /some/unknown/route` → 200 且为 index.html | SPA fallback 未生效 |
| `POST /api/v1/login` → **非 404** | `/api/` 未真正反代,被静态规则吃掉 |
| `/assets/*` 响应头含 `immutable` | 缓存规则未生效 |
| `pkill -f /usr/local/bin/api` 后容器 15s 内退出 | **D1 的 `wait -n` 语义**——本地无法验证的核心行为 |

失败时输出完整 `docker logs`。

**边界(必须诚实说明)**:冒烟使用测试凭据,证明的是「配置合法时镜像可运行」,**不证明生产配置正确**;不覆盖真实 schema(尚无迁移文件);不覆盖 SMTP。生产配置正确性仍依赖部署时健康检查 + 自动回滚。两者互补而非替代。

## 验证

开发机**没有 docker**,镜像构建与运行无法本地验证。合并前可做:

- `bash -n deploy/entrypoint.sh`
- python 解析 `deploy.yml` / `ci.yml` 确认 YAML 合法,并对内嵌 shell 逐段 `bash -n`
- `pnpm --filter @iot-pillot/web build`,确认产物落在 `apps/web/dist` 且结构与 nginx `root` 一致
- `cd apps/api && go vet ./... && go test ./... && go build ./cmd`

`nginx.conf` 语法本地无法验证,只能依赖冒烟测试——这正是它存在的理由。

## 顺带修复

- `apps/web/src/views/Home.vue:14` 请求 `/api/health`,但后端健康检查在根级 `/health`,`/api/v1` 下无此路由,当前为 404。建议后端补 `/api/v1/health` 供业务页面使用,根级 `/health` 保留给运维探针,职责分离。
- 同源部署后生产环境不再需要 CORS。`IOT_PILOT_CORS_ALLOWED_ORIGINS` 仅对本地 `vite dev` 有意义,`.env.example` 补充说明。

## 风险

1. **(高)** 无法本地验证镜像,首次正确性完全依赖冒烟测试的覆盖度。若冒烟本身写错,问题会直达服务器(仍有自动回滚兜底)。
2. **(中)** 传输量从约 8MB(gzip 后)增至约 20MB。服务器网络若本就吃力会有感知。
3. **(中)** 两进程容器的日志混合在同一 stdout,排查时需按前缀区分。
4. **(低)** 前端改动会触发后端重建与重启,与「增量部署」目标存在张力。这是单一制品的固有代价,已在选型时接受。

## 不在本次范围

- HTTPS / TLS 终止(将来在容器前加一层反代即可,与本设计无冲突)
- 数据库迁移工具
- 前端独立部署能力
