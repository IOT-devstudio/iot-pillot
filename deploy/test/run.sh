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

python deploy/test/extract-server-script.py "$WORK/server.sh" "$WORK/gate.sh" || exit 1

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

assert_rejected_before_docker() {
  label="$1"
  variable="$2"
  expected="$3"
  value="${4-__UNSET__}"
  safe_label=$(printf '%s' "$label" | tr -c '[:alnum:]' '_')
  case_dir="$WORK/rejected_$safe_label"
  case_log="$WORK/rejected_$safe_label.log"
  case_out="$WORK/rejected_$safe_label.out"

  (
    export PATH="$PWD/$STUB_DIR:$PATH"
    export HOME="$case_dir" STUB_LOG="$case_log"
    export STUB_PREV_IMAGE="" STUB_PREV_CPORT="" STUB_HEALTH="ok"
    if [ "$value" = "__UNSET__" ]; then
      unset "$variable"
    else
      export "$variable=$value"
    fi
    mkdir -p "$HOME/app"; printf 'd' | gzip > "$HOME/app/app.tar.gz"
    bash "$WORK/server.sh" > "$case_out" 2>&1
  )
  case_rc=$?

  if [ "$case_rc" != "0" ] \
     && grep -qF "$expected" "$case_out" \
     && [ ! -s "$case_log" ]; then
    ok "$label"
  else
    bad "$label" \
      "exit=${case_rc}，docker=$(cat "$case_log" 2>/dev/null)，输出：$(head -3 "$case_out" 2>/dev/null)"
  fi
}

printf '服务器部署脚本 · 打桩测试\n\n'

# 应用配置环境变量（与 GitHub secrets/variables 一一对应）
export DB_HOST=stub-postgres DB_PORT=5432 DB_USER=stub_user DB_PASSWORD=stub_pw DB_NAME=iot_pillot
export REDIS_HOST=stub-redis REDIS_PORT=6379 REDIS_PASSWORD=stub_redis_pw REDIS_DB=1
export JWT_SECRET=stub-jwt-secret-not-for-production
export ADMIN_USERS=1
export SMTP_HOST=stub-smtp SMTP_PORT=587 SMTP_USERNAME=stub_user
export SMTP_PASSWORD=stub_smtp_pw SMTP_FROM=stub@example.com

# --- 场景 0：应用配置注入 ----------------------------------------------------
run_deploy "sha256:OLDIMAGE" "8080" "ok"
last_run=$(runs_of | tail -1)
missing=""
for kv in 'IOT_PILOT_MODE=release' 'IOT_PILOT_DB_HOST=stub-postgres' \
          'IOT_PILOT_DB_USER=stub_user' 'IOT_PILOT_DB_PASSWORD=stub_pw' \
          'IOT_PILOT_DB_NAME=iot_pillot' 'IOT_PILOT_REDIS_HOST=stub-redis' \
          'IOT_PILOT_REDIS_DB=1' \
          'IOT_PILOT_JWT_SECRET=stub-jwt-secret-not-for-production' \
          'IOT_PILOT_AUTH_ADMIN_USERS=1' \
          'IOT_PILOT_SMTP_HOST=stub-smtp' 'IOT_PILOT_SMTP_PORT=587' \
          'IOT_PILOT_SMTP_USERNAME=stub_user' \
          'IOT_PILOT_SMTP_PASSWORD=stub_smtp_pw' \
          'IOT_PILOT_SMTP_FROM=stub@example.com'; do
  printf '%s' "$last_run" | grep -qF -- "$kv" || missing="$missing $kv"
done
if [ -z "$missing" ]; then
  ok "docker run 注入了全部应用配置（mode/db/redis/jwt/admin/smtp）"
else
  bad "docker run 缺少应用配置" "缺：$missing"
fi

if printf '%s' "$last_run" | grep -q -- '--network iot-pilot-net'; then
  ok "docker run 加入了 iot-pilot-net 网络（DB/Redis 以容器名寻址的前提）"
else
  bad "docker run 缺少 --network"
fi

# --- 场景 0b：配置错误时必须在操作 Docker 前快速失败 -------------------------
assert_rejected_before_docker \
  "缺少 secret 时在操作 Docker 前明确报错（DB_HOST）" \
  DB_HOST "缺少配置：DB_HOST"
assert_rejected_before_docker \
  "缺少 Redis DB 时在操作 Docker 前明确报错（REDIS_DB）" \
  REDIS_DB "缺少配置：REDIS_DB"
assert_rejected_before_docker \
  "非法 Redis DB 在操作 Docker 前被拒绝" \
  REDIS_DB "配置错误：REDIS_DB 必须为 1" invalid
assert_rejected_before_docker \
  "缺少管理员名单时在操作 Docker 前明确报错（ADMIN_USERS）" \
  ADMIN_USERS "缺少配置：ADMIN_USERS"
assert_rejected_before_docker \
  "管理员名单不含 user_id 1 时在操作 Docker 前被拒绝" \
  ADMIN_USERS "配置错误：ADMIN_USERS 必须包含 user_id 1" admin

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

# --- 场景 4：部署 workflow 自身变化 -------------------------------------------
# workflow 决定了镜像如何启动和注入配置；即使镜像内容没变，部署脚本变化也必须
# 重新执行。这里在临时 git 仓库里真实运行 gate 的 decide 脚本，而不是只 grep YAML。
GATE_REPO="$WORK/gate-repo"
mkdir -p "$GATE_REPO/.github/workflows"
(
  cd "$GATE_REPO" || exit 1
  git init -q
  git config user.name "deploy-test"
  git config user.email "deploy-test@example.com"
  printf 'name: old\n' > .github/workflows/deploy.yml
  git add .github/workflows/deploy.yml
  git commit -qm "initial deploy workflow"
  git tag deployed
  printf 'name: new\n' > .github/workflows/deploy.yml
  git add .github/workflows/deploy.yml
  git commit -qm "change deploy workflow"
  : > "$WORK/gate_output"
  : > "$WORK/gate_summary"
  FORCE=false EVENT=workflow_run \
    GITHUB_OUTPUT="$WORK/gate_output" \
    GITHUB_STEP_SUMMARY="$WORK/gate_summary" \
    bash "$WORK/gate.sh" > "$WORK/gate_stdout" 2>&1
)
GATE_RC=$?
if [ "$GATE_RC" = "0" ] && grep -qx 'deploy=true' "$WORK/gate_output"; then
  ok "部署 workflow 自身变化会触发自动部署"
else
  bad "部署 workflow 自身变化没有触发自动部署" \
    "exit=${GATE_RC}，输出：$(cat "$WORK/gate_output" 2>/dev/null)"
fi

printf '\n通过 %s 项，失败 %s 项\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
