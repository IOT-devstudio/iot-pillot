#!/usr/bin/env bash
#
# 服务器部署脚本的打桩测试。
#
# deploy.yml 里 ssh-action 的那段脚本是整条流水线上最难验证的部分：
# 它只在真实服务器上、真实故障时才执行，而那正是最不该出错的时刻。
# 这个装置把它抽出来，用假的 docker / curl / sleep 在本地跑一遍，
# 断言它在各种情况下发出的 docker 调用是对的。
#
# 用法：  bash deploy/test/run.sh        （在仓库根执行）
#
# 依赖：bash、python + PyYAML（只用来从 YAML 里抽出脚本）

set -uo pipefail

cd "$(dirname "$0")/../.." || exit 1

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

STUB_DIR="deploy/test/stub"
chmod +x "$STUB_DIR"/* 2>/dev/null || true

python deploy/test/extract-server-script.py "$WORK/server.sh" || exit 1

PASS=0
FAIL=0
ok()  { printf '  PASS  %s\n' "$1"; PASS=$((PASS + 1)); }
bad() { printf '  FAIL  %s\n' "$1"; printf '        %s\n' "$2"; FAIL=$((FAIL + 1)); }

# 跑一次部署脚本。$1=旧镜像ID $2=旧容器端口 $3=健康检查行为
# 结果：$RC 退出码，$WORK/log 里是所有 docker 调用
run_deploy() {
  rm -rf "$WORK/home" "$WORK/log" "$WORK/log.curlcount"
  mkdir -p "$WORK/home/app"
  printf 'dummy' | gzip > "$WORK/home/app/app.tar.gz"
  (
    export PATH="$PWD/$STUB_DIR:$PATH"
    export HOME="$WORK/home" STUB_LOG="$WORK/log"
    export STUB_PREV_IMAGE="$1" STUB_PREV_CPORT="$2" STUB_HEALTH="$3"
    bash "$WORK/server.sh" > "$WORK/stdout" 2>&1
  )
  RC=$?
}

runs_of() { grep '^RUN_ARGS' "$WORK/log" 2>/dev/null; }

printf '服务器部署脚本 · 打桩测试\n\n'

# 应用配置环境变量（与 GitHub secrets 一一对应）
export DB_HOST=stub-postgres DB_PORT=5432 DB_USER=stub_user DB_PASSWORD=stub_pw DB_NAME=iot_pillot
export REDIS_HOST=stub-redis REDIS_PORT=6379 REDIS_PASSWORD=stub_redis_pw
export JWT_SECRET=stub-jwt-secret-not-for-production

# --- 场景 0：应用配置注入 ----------------------------------------------------
run_deploy "sha256:OLDIMAGE" "8080" "ok"
last_run=$(runs_of | tail -1)
missing=""
for kv in 'IOT_PILOT_MODE=release' 'IOT_PILOT_DB_HOST=stub-postgres' \
          'IOT_PILOT_DB_USER=stub_user' 'IOT_PILOT_DB_PASSWORD=stub_pw' \
          'IOT_PILOT_DB_NAME=iot_pillot' 'IOT_PILOT_REDIS_HOST=stub-redis' \
          'IOT_PILOT_JWT_SECRET=stub-jwt-secret-not-for-production'; do
  printf '%s' "$last_run" | grep -qF -- "$kv" || missing="$missing $kv"
done
if [ -z "$missing" ]; then
  ok "docker run 注入了全部应用配置（mode/db/redis/jwt）"
else
  bad "docker run 缺少应用配置" "缺：$missing"
fi

# --- 场景 0b：secrets 缺失时的快速失败 ---------------------------------------
# 少配一项时，脚本必须在起容器之前就失败，并报出缺的是哪一项，
# 而不是起一个必死的容器、跑完 60 秒健康检查、再回滚。
(
  export PATH="$PWD/$STUB_DIR:$PATH"
  export HOME="$WORK/missing_home" STUB_LOG="$WORK/nothing"
  export STUB_PREV_IMAGE="" STUB_PREV_CPORT="" STUB_HEALTH="ok"
  unset DB_HOST    # 模拟 GitHub secrets 少配这一项
  mkdir -p "$HOME/app"; printf 'd' | gzip > "$HOME/app/app.tar.gz"
  bash "$WORK/server.sh" > "$WORK/missing_out" 2>&1
)
MISSING_RC=$?
if [ "$MISSING_RC" != "0" ] && grep -q "缺少配置：DB_HOST" "$WORK/missing_out"; then
  ok "缺少 secrets 时在使用前明确报错（DB_HOST）"
else
  bad "缺少 secrets 时的行为不对" "exit=${MISSING_RC}，输出：$(head -3 "$WORK/missing_out" 2>/dev/null)"
fi

# --- 场景 1：跨端口回滚 ------------------------------------------------------
# 旧容器监听 8080（纯 api 镜像），新镜像监听 80（nginx）。
# 新镜像健康检查失败时，回滚必须用旧容器自己的端口，
# 否则会把宿主端口映射到一个没人监听的容器端口上，回滚本身也失败、服务躺下。
run_deploy "sha256:OLDIMAGE" "8080" "fail_then_ok"
rollback=$(runs_of | grep 'sha256:OLDIMAGE')
if [ "$RC" != "1" ]; then
  bad "健康检查失败时应以非零退出" "实得 exit=${RC}"
elif [ -z "$rollback" ]; then
  bad "未发生回滚" "期望以旧镜像重新启动容器"
elif printf '%s' "$rollback" | grep -q -- '-p 80:8080'; then
  ok "跨端口回滚使用旧容器自己的端口（80:8080）"
else
  bad "回滚端口错误" "$rollback"
fi

# --- 场景 2：健康检查通过 ----------------------------------------------------
run_deploy "sha256:OLDIMAGE" "8080" "ok"
n=$(runs_of | wc -l)
if [ "$RC" = "0" ] && [ "$n" -eq 1 ]; then
  ok "健康检查通过时只启动一次、不回滚"
else
  bad "健康检查通过时的行为不对" "exit=${RC}，docker run 次数=${n}"
fi

# --- 场景 3：没有可回滚的旧容器 ----------------------------------------------
run_deploy "" "" "fail"
n=$(runs_of | wc -l)
if [ "$RC" = "1" ] && [ "$n" -eq 1 ]; then
  ok "无旧容器时不尝试回滚，如实失败"
else
  bad "无旧容器时的行为不对" "exit=${RC}，docker run 次数=${n}"
fi

printf '\n通过 %s 项，失败 %s 项\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
