#!/usr/bin/env bash
#
# 镜像冒烟测试：对一个已经跑起来的容器做黑盒断言。
#
# 用法：  deploy/smoke.sh [BASE_URL]        默认 http://127.0.0.1:8080
#
# 被两处调用：
#   - ci.yml   PR 阶段跑，给早期信号，也用来验证这个脚本本身
#   - deploy.yml  构建之后、scp 之前跑，坏镜像到不了服务器
#
# 为什么两边都跑：auto-merge.yml 不等 CI 就合并 PR，所以只放 CI 拦不住坏合并；
# 只放 deploy 又没法在不碰生产的前提下验证脚本自身。
#
# 断言的是「配置合法时镜像能正常工作」，不保证生产配置正确 ——
# 后者由部署时的健康检查 + 自动回滚兜底，两者互补。

set -uo pipefail

BASE_URL="${1:-http://127.0.0.1:8080}"

PASS=0
FAIL=0

ok()   { printf '  PASS  %s\n' "$1"; PASS=$((PASS + 1)); }
bad()  { printf '  FAIL  %s\n' "$1"; printf '        %s\n' "$2"; FAIL=$((FAIL + 1)); }

# 等待服务就绪。容器刚起来时连接会被拒，这不算失败。
wait_ready() {
  local i
  for i in $(seq 1 30); do
    if curl -fsS -o /dev/null "${BASE_URL}/health" 2>/dev/null; then
      printf '服务就绪（第 %s 次尝试）\n\n' "$i"
      return 0
    fi
    sleep 2
  done
  printf '服务在 60 秒内没有就绪，后续断言无意义\n' >&2
  return 1
}

# 取 HTTP 状态码，连接失败时返回 000
status_of() {
  curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$@" 2>/dev/null || echo 000
}

printf '冒烟测试目标：%s\n\n' "$BASE_URL"

if ! wait_ready; then
  exit 1
fi

# --- 1. 健康检查 -------------------------------------------------------------
# 覆盖面最广的一条：进程活着、路由正常、Postgres 与 Redis 都连上了、配置合法。
body=$(curl -fsS --max-time 10 "${BASE_URL}/health" 2>&1)
if [ $? -eq 0 ] && printf '%s' "$body" | grep -q '"status"'; then
  ok "GET /health 返回 200 且含 status 字段"
else
  bad "GET /health" "响应：${body}"
fi

# --- 2. 业务路由确实挂载了 ---------------------------------------------------
# 断言「不是 404」而非某个具体状态码：请求体是垃圾数据，400/401/422 都算正常，
# 只有 404 说明路由压根没注册（或者被静态文件规则吃掉了 —— 引入 nginx 后这条会变得关键）。
code=$(status_of -X POST -H 'Content-Type: application/json' -d '{}' "${BASE_URL}/api/v1/login")
if [ "$code" != "404" ] && [ "$code" != "000" ]; then
  ok "POST /api/v1/login 已挂载（HTTP ${code}，非 404）"
else
  bad "POST /api/v1/login 未挂载" "HTTP ${code}"
fi

# --- 3. 未知路径不应被无差别放行 ---------------------------------------------
# 现在应当是 404。引入 nginx + SPA fallback 之后，这条要改成「返回 index.html」，
# 届时 /health 与 /api/ 必须有各自的 location，否则会被 fallback 吃掉并永远返回 200。
code=$(status_of "${BASE_URL}/__definitely_not_a_route__")
if [ "$code" = "404" ]; then
  ok "GET /__definitely_not_a_route__ 返回 404"
else
  bad "未知路径返回了 ${code}，期望 404" "无差别 200 会让健康检查失去意义"
fi

printf '\n通过 %s 项，失败 %s 项\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
