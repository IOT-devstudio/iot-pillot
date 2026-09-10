# 多阶段构建：Go 1.25 → 静态二进制 → Alpine 运行时镜像。
FROM golang:1.25-alpine AS builder
WORKDIR /src

# 先复制 go.mod / go.sum 利用 Docker 缓存。
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并编译。
# 入口是 ./cmd（cmd/main.go + wire_gen.go），不是 ./cmd/api —— PR #13 引入 wire DI
# 时挪过位置。ci.yml 的 `go build ./...` 编译所有包，捕捉不到这里的路径漂移，
# 唯一的守门人是部署时的 docker build。改动 cmd 目录结构时务必同步这一行。
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd

# 运行时镜像：极简，仅含 ca-certificates 与时区数据。
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/api /app/api

EXPOSE 8080
ENTRYPOINT ["/app/api"]
