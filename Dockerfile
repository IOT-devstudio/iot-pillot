# 单镜像：nginx 托管前端静态产物 + 反代同容器内的 Go API。
#
# 设计见 docs/superpowers/specs/2026-09-10-single-image-nginx-design.md
#
# build context 是仓库根（不是 apps/api），因为要同时看到 apps/web 与 apps/api。
# 根 .dockerignore 必须排除 node_modules，否则 context 会有几百 MB。
#
# 注意：本镜像是单平台的。deploy.yml 用 `outputs: type=docker` 导出 tar，
# 那个 exporter 不支持多平台，若将来加 platforms: linux/amd64,arm64 会失败。

# ---- 阶段 1：前端 ----
FROM node:20-alpine AS frontend
RUN npm i -g pnpm@8
WORKDIR /src
# 先只拷 manifest，让依赖层在源码变动时可复用。
# 仓库根没有 package.json（workspace 由 pnpm-workspace.yaml 定义）；
# apps/api 也不是 workspace 成员（无 package.json）。不要 COPY 不存在的文件。
COPY pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json              apps/web/
COPY packages/shared-types/package.json packages/shared-types/
RUN pnpm install --frozen-lockfile
COPY packages packages
COPY apps/web apps/web
RUN pnpm --filter @iot-pillot/web build
# → /src/apps/web/dist（vite 未覆盖 outDir 与 base，用默认值）

# ---- 阶段 2：后端 ----
FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api .
# 构建目标必须是 ./cmd（包），不是 cmd/main.go：
# cmd/ 下还有 wire.go 与 wire_gen.go，单文件编译会 undefined: InitializeApp。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd

# ---- 阶段 3：运行时 ----
# alpine + apk 装 nginx 比 nginx:alpine 小一半（约 45MB vs 约 70MB）。
# 传输量对本项目重要——当初放弃 GHCR 改用 docker save + scp 就是因为服务器网络慢。
# bash 是必需的：entrypoint 依赖 `wait -n`，busybox ash 不支持。
FROM alpine:3.20
RUN apk add --no-cache nginx tini bash curl ca-certificates tzdata \
 && mkdir -p /run/nginx /var/www/html

COPY deploy/nginx.conf    /etc/nginx/nginx.conf
COPY deploy/entrypoint.sh /entrypoint.sh
COPY --from=frontend /src/apps/web/dist /var/www/html
COPY --from=backend  /out/api           /usr/local/bin/api
RUN chmod +x /entrypoint.sh

# nginx 监听 80；Go 用它的默认端口 8080，仅在 127.0.0.1 上被 nginx 反代，不对外暴露。
EXPOSE 80

# tini 作 PID 1 负责回收僵尸进程与转发信号。
# 但容器的存活语义由 entrypoint 的 `wait -n` 决定——tini 不管后台孙子进程的死活。
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/entrypoint.sh"]
