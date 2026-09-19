# 独立的 API 镜像：只有 Go 二进制，不含前端产物与 nginx。
#
# 用途：单独跑后端做联调/排障（判断问题出在 Go 服务还是 nginx 与前端），
# 以及需要"纯 API"容器时直接 docker run。
#
# **生产部署不走这个文件**：deploy.yml 与 docker-compose.yml 的 app 服务都用
# 仓库根的 Dockerfile（单镜像 = 前端静态产物 + nginx + 同一个 Go 二进制）。
# 两个文件都编译 ./cmd，改 Go 版本、构建参数或构建目标时要一起改。
#
# 构建（context 必须是 apps/api，不是仓库根）：
#   docker build -f apps/api/Dockerfile -t iot-pillot-api:local apps/api
#
# 配置全部来自环境变量（IOT_PILOT_*），镜像里不含 config.yaml。
# DB 与 Redis 是启动期硬依赖：连不上容器会直接退出，不做降级启动。

# ---- 阶段 1：构建 ----
# 版本要与 apps/api/go.mod 的 go 指令（1.25.0）以及根 Dockerfile 的 backend 阶段一致。
FROM golang:1.25-alpine AS builder

# Go module 源。默认官方源（CI/境外可用）；国内网络访问 proxy.golang.org 会被拒，
# 本地构建时覆盖：--build-arg GOPROXY=https://goproxy.cn,direct
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src

# 先只拷 manifest：源码变动时依赖层仍可复用。
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 构建目标是 ./cmd（包），不是仓库根的 "."，也不是 cmd/main.go：
# apps/api 根目录没有任何 .go 文件，`go build .` 会以 "no Go files" 失败；
# 单文件编译会漏掉同包的 wire_gen.go，变成 undefined: InitializeApp。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd

# ---- 阶段 2：运行时 ----
FROM alpine:3.20

# ca-certificates 是 SMTP over TLS 的必需品；tzdata 让日志时间与部署环境一致。
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 appuser

ENV TZ=Asia/Shanghai
WORKDIR /app

COPY --from=builder /out/api /usr/local/bin/api

# 非 root 运行：镜像里没有任何需要写权限的路径。
USER appuser
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
