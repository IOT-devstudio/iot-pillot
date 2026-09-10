#!/usr/bin/env bash
#
# 单镜像内两个进程的生命周期管理。
#
# 关键：绝不能写成下面这种常见形式 ——
#
#     api &
#     nginx -g "daemon off;"
#
# 那样 nginx 在前台，api 死掉之后容器仍是 running 状态，
# --restart unless-stopped 永不触发，站点静默返回 502 直到有人手动介入。
# tini 救不了这个：它只向自己的直接子进程转发信号，不管后台孙子进程的死活。
#
# 用 `wait -n` 取而代之：任一进程退出 → PID 1 退出 → 容器退出 → docker 重启策略
# 把两个一起拉起来。代价是需要 bash（busybox ash 没有 wait -n）。

set -uo pipefail

API_BIN=/usr/local/bin/api

term() {
  nginx -s quit 2>/dev/null || true
  [ -n "${API_PID:-}" ] && kill -TERM "$API_PID" 2>/dev/null || true
}
trap term SIGTERM SIGINT

echo "[entrypoint] 启动 API"
"$API_BIN" &
API_PID=$!

echo "[entrypoint] 启动 nginx"
nginx -g 'daemon off;' &
NGINX_PID=$!

echo "[entrypoint] api pid=${API_PID} nginx pid=${NGINX_PID}，等待任一进程退出"

wait -n
EXIT=$?

echo "[entrypoint] 有进程退出（code=${EXIT}），关闭另一个并退出容器"
term
wait 2>/dev/null || true
exit "$EXIT"
