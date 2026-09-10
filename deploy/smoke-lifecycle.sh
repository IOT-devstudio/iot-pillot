#!/usr/bin/env bash
#
# 容器生命周期断言：验证 entrypoint 的 `wait -n` 语义。
#
# 用法：  deploy/smoke-lifecycle.sh [容器名]      默认 smoke
#
# 与 smoke.sh 分开的原因：这一条需要 docker，没法用假服务器在本地验证，
# 而 smoke.sh 保持纯 HTTP 以便本地自测。
#
# 这条断言守的是整个单镜像方案里最容易写错的地方。常见的错误写法：
#
#     api &
#     nginx -g "daemon off;"
#
# 那样 api 死掉之后 nginx 还在前台，容器仍是 running，重启策略永不触发，
# 站点静默 502 直到有人手动介入。正确行为是任一进程退出则容器退出，
# 交给 --restart unless-stopped 把两个一起拉起来。

set -uo pipefail

CONTAINER="${1:-smoke}"
TIMEOUT=15

printf '杀掉容器内的 api 进程，容器应当在 %s 秒内退出\n' "$TIMEOUT"

if ! docker exec "$CONTAINER" pkill -f /usr/local/bin/api; then
  printf '  FAIL  无法在容器内结束 api 进程\n'
  exit 1
fi

for i in $(seq 1 "$TIMEOUT"); do
  running=$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null || echo gone)
  if [ "$running" != "true" ]; then
    printf '  PASS  容器已退出（第 %s 秒）——后端进程死亡会带走容器，重启策略可以接手\n' "$i"
    exit 0
  fi
  sleep 1
done

printf '  FAIL  容器在 %s 秒后仍在运行\n' "$TIMEOUT"
printf '        entrypoint 没有正确处理后端退出：线上表现将是站点静默 502 且永不自愈\n'
exit 1
