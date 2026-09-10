#!/usr/bin/env bash
#
# 镜像冒烟测试：对一个已经跑起来的容器做黑盒断言。
#
# 用法：  deploy/smoke.sh [BASE_URL]        默认 http://127.0.0.1:8080
#
# 被两处调用：
#   - ci.yml      PR 阶段跑，给早期信号，也用来验证这个脚本本身
#   - deploy.yml  构建之后、scp 之前跑，坏镜像到不了服务器
#
# 为什么两边都跑：auto-merge.yml 不等 CI 就合并 PR，所以只放 CI 拦不住坏合并；
# 只放 deploy 又没法在不碰生产的前提下验证脚本自身。
#
# 只做 HTTP 断言，不依赖 docker，因此可以用一个假服务器在本地验证它自己有没有牙。
# 进程监督语义（杀掉 api 后容器是否退出）需要 docker，放在 workflow 里单独一步。
#
# 断言的是「配置合法时镜像能正常工作」，不保证生产配置正确 ——
# 后者由部署时的健康检查 + 自动回滚兜底，两者互补。

set -uo pipefail

BASE_URL="${1:-http://127.0.0.1:8080}"

PASS=0
FAIL=0
SPA_MARKER='<div id="app"'

ok()  { printf '  PASS  %s\n' "$1"; PASS=$((PASS + 1)); }
bad() { printf '  FAIL  %s\n' "$1"; printf '        %s\n' "$2"; FAIL=$((FAIL + 1)); }

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

status_of() {
  curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$@" 2>/dev/null || echo 000
}

printf '冒烟测试目标：%s\n\n' "$BASE_URL"
wait_ready || exit 1

# --- 1. 健康检查真的打到了后端 ----------------------------------------------
# 覆盖面最广的一条：nginx 起来了、反代通、Go 起来了、Postgres 与 Redis 连上了、
# 配置合法。
health=$(curl -fsS --max-time 10 "${BASE_URL}/health" 2>&1)
if printf '%s' "$health" | grep -q '"status"'; then
  ok "GET /health 打到后端（响应含 status 字段）"
else
  bad "GET /health 未返回后端响应" "响应：${health}"
fi

# --- 2. /health 绝不能是 SPA fallback 的产物 --------------------------------
# 这是整套配置里最危险的失效模式：后端健康检查挂在根级 /health，
# nginx 若没给它独立的 location，请求会落进 try_files ... /index.html，
# 于是后端死透时 /health 依然返回 200 + 前端页面。
# deploy.yml 的健康检查会因此永远通过，自动回滚被悄悄废掉，
# 坏版本还会被判定成功并推进 deployed tag。
if printf '%s' "$health" | grep -qF "$SPA_MARKER"; then
  bad "/health 返回的是前端页面" "说明它被 SPA fallback 吃掉了，自动回滚已失效"
else
  ok "/health 未被 SPA fallback 吃掉"
fi

# --- 3. 业务 API 确实反代给了后端 -------------------------------------------
# 断言「不是 404」而非某个具体状态码：请求体是垃圾数据，400/401/422 都算正常。
# 404 则意味着路由没挂载，或者 /api/ 被静态规则吃掉了。
code=$(status_of -X POST -H 'Content-Type: application/json' -d '{}' "${BASE_URL}/api/v1/login")
if [ "$code" != "404" ] && [ "$code" != "000" ]; then
  ok "POST /api/v1/login 反代正常（HTTP ${code}，非 404）"
else
  bad "POST /api/v1/login 未反代到后端" "HTTP ${code}"
fi

# --- 4. 前端静态页面能出来 ---------------------------------------------------
index=$(curl -fsS --max-time 10 "${BASE_URL}/" 2>&1)
if printf '%s' "$index" | grep -qF "$SPA_MARKER"; then
  ok "GET / 返回前端页面"
else
  bad "GET / 未返回前端页面" "静态文件挂载路径可能不对。响应：${index:0:200}"
fi

# --- 5. SPA fallback 生效 ----------------------------------------------------
# 前端是 history 模式路由，未知路径必须交回 index.html 而不是 404。
fallback=$(curl -fsS --max-time 10 "${BASE_URL}/some/client/side/route" 2>&1)
if printf '%s' "$fallback" | grep -qF "$SPA_MARKER"; then
  ok "未知路径回落到 index.html（SPA fallback 生效）"
else
  bad "SPA fallback 未生效" "try_files 配置可能有误"
fi

# --- 6. 静态资源长缓存 -------------------------------------------------------
# 从 index.html 里提取真实的带指纹资源路径（文件名每次构建都变，不能写死）。
asset=$(printf '%s' "$index" | grep -oE '/assets/[A-Za-z0-9._-]+\.(js|css)' | head -1)
if [ -z "$asset" ]; then
  bad "index.html 里没有 /assets/ 资源引用" "构建产物结构可能变了"
else
  headers=$(curl -sS -o /dev/null -D - --max-time 10 "${BASE_URL}${asset}" 2>&1)
  acode=$(printf '%s' "$headers" | head -1)
  if printf '%s' "$headers" | grep -qi 'cache-control:.*immutable'; then
    ok "静态资源 ${asset} 命中长缓存规则"
  else
    bad "静态资源 ${asset} 缺少 immutable 缓存头" "${acode}"
  fi
fi

printf '\n通过 %s 项，失败 %s 项\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
