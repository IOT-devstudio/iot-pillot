#!/usr/bin/env bash
#
# 全接口连通性巡检（issue #102）：对已运行的 API 做黑盒契约断言。
#
# 用法：  deploy/api-test.sh [BASE_URL]      默认 http://127.0.0.1:8080
#
# 可选环境变量：
#   REDIS_CMD     读写验证码/管理员集合/限流 key 的 redis 命令前缀
#                 默认 "docker exec iot-pillot-redis redis-cli"
#   SKIP_DOCS=1   跳过 /docs /openapi.json 断言（debug-only 端点）
#
# 断言策略（与 issue #102 的裁决一致）：
#   - 判据 = HTTP 状态码 + 统一响应体 {code,message,data} 的业务码；
#   - 外发/破坏性接口（mails/send*、invitations/send、candidates/advance）
#     只打「鉴权 + 参数校验」路径，不触发真实 SMTP；
#   - 认证链真实走通：注册（从 Redis 读验证码，不依赖邮件送达）→ login →
#     /me → 成员 403 → 提权走管理端 → 改密/refresh/logout 生命周期。
#
# KNOWN：已确认缺陷、当前实现与契约不符的条目（见 issue #105 #106）。
# 它们如实打印但不计入 FAIL，让本脚本能当回归门禁长期跑；
# 缺陷修复后本脚本行为自动回到 assert 路径，届时把 KNOWN 改回 assert 即可。
#
# 本地跑一次全栈的前提（README + migrations/README）：
#   docker compose up -d postgres redis
#   psql 依序执行 apps/api/migrations/*.up.sql
#   带 IOT_PILOT_* 环境变量启动 API（debug 模式，docs 开启）
#
# 只依赖 curl + grep/sed，可在 Git Bash 与 CI 直接跑。
# 断言用的载荷值一律 ASCII：Windows 控制台/管道编码会破坏 CJK 字节，
# 造成假 FAIL（issue #102 首轮巡检实测）。

set -uo pipefail

BASE_URL="${1:-http://127.0.0.1:8080}"
REDIS_CMD="${REDIS_CMD:-docker exec iot-pillot-redis redis-cli}"

PASS=0
FAIL=0
SKIP_N=0
KNOWN_N=0

ok()    { printf '  PASS  %s\n' "$1"; PASS=$((PASS + 1)); }
bad()   { printf '  FAIL  %s\n        %s\n' "$1" "$2"; FAIL=$((FAIL + 1)); }
skip()  { printf '  SKIP  %s (%s)\n' "$1" "$2"; SKIP_N=$((SKIP_N + 1)); }
# known <name> <说明> —— 已知缺陷命中：打印并计数，不计入 FAIL
known() { printf '  KNOWN %s\n        %s\n' "$1" "$2"; KNOWN_N=$((KNOWN_N + 1)); }

wait_ready() {
  local i
  for i in $(seq 1 30); do
    if curl -fsS -o /dev/null --max-time 5 "${BASE_URL}/health/ready" 2>/dev/null; then
      printf '服务就绪（第 %s 次尝试）\n\n' "$i"
      return 0
    fi
    sleep 2
  done
  printf '服务在 60 秒内没有就绪，后续断言无意义\n' >&2
  return 1
}

# ── HTTP 工具 ──────────────────────────────────────────────────────
# REQ <method> <quoted-path> [json-body] [bearer-token]；结果写进 CODE / RBODY
CODE=""
RBODY=""
REQ() {
  local method="$1" path="$2" body="${3:-}" token="${4:-}"
  local url="${BASE_URL}${path}"
  local args=(-s -X "$method" --max-time 15 -w '\n%{http_code}')
  [[ -n "$body" ]] && args+=(-H 'Content-Type: application/json' -d "$body")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  RBODY=$(curl "${args[@]}" "$url")
  CODE=$(printf '%s' "$RBODY" | tail -n1)
  RBODY=$(printf '%s' "$RBODY" | sed '$d')
}

biz_of() { printf '%s' "$RBODY" | grep -o '"code":[0-9-]*' | head -1 | cut -d: -f2; }

# assert <name> <expected-http> [expected-biz]
assert() {
  local name="$1" want="$2" wantbiz="${3:-}" biz
  biz=$(biz_of)
  if [[ "$CODE" != "$want" ]]; then
    bad "$name" "期望 HTTP $want，实际 $CODE；body: $(printf '%.160s' "$RBODY")"
  elif [[ -n "$wantbiz" && "$biz" != "$wantbiz" ]]; then
    bad "$name" "HTTP $CODE 对，但业务码期望 $wantbiz 实际 ${biz:-无}；body: $(printf '%.160s' "$RBODY")"
  else
    ok "$name (HTTP $CODE${wantbiz:+, code=$wantbiz})"
  fi
}

# assert_any <name> "c1 c2 c3" —— HTTP 码落在允许集合即通过（环境依赖端点用）
assert_any() {
  local name="$1" wants="$2" c hit=0
  for c in $wants; do [[ "$CODE" == "$c" ]] && hit=1; done
  if [[ "$hit" == 1 ]]; then
    ok "$name (HTTP $CODE ∈ {$wants})"
  else
    bad "$name" "HTTP $CODE 不在允许集合 {$wants}；body: $(printf '%.160s' "$RBODY")"
  fi
}

# assert_or_known <name> <期望http> <已知缺陷号> —— 期望码 assert；
# 命中 500 且是缺表类错误时标 KNOWN（缺陷修复后自动回到正常断言路径）
assert_or_known() {
  local name="$1" want="$2" ticket="$3"
  if [[ "$CODE" == "500" && "$RBODY" == *"does not exist"* ]]; then
    known "$name" "命中缺陷 ${ticket}（relation 缺失导致 500）；$(printf '%.120s' "$RBODY")"
  else
    assert "$name" "$want"
  fi
}

jstr() { printf '%s' "$RBODY" | grep -o "\"$1\":\"[^\"]*\"" | head -1 | cut -d'"' -f4; }
jnum() { printf '%s' "$RBODY" | grep -o "\"$1\":[0-9-]*"  | head -1 | cut -d: -f2; }

# 每次运行用唯一邮箱后缀，避开「邮箱唯一」与 60s 冷却的重跑冲突
STAMP="$(date +%m%d%H%M%S)"
EMAIL_A="api-test-a-${STAMP}@test.local"
EMAIL_B="api-test-b-${STAMP}@test.local"
PW='Api-test-Pw1'
PW2='Api-test-Pw2'

# 清掉验证码限流 key，保证重跑不被 IP 配额（10/小时）卡住。
# 只删本脚本约定前缀，不动业务数据；redis 不可用时静默跳过（断言仍能容错）。
for k in $( { $REDIS_CMD --scan --pattern 'limit:verify_code:*' 2>/dev/null || true; } | grep 'limit:verify_code:' ); do
  $REDIS_CMD DEL "$k" >/dev/null 2>&1 || true
done

# register_via_redis <email> <name> —— 发码（SMTP 可达与否都接受）、
# 从 Redis 读码、注册；stdout 输出 "access refresh user_id"，失败返回非 0。
# 函数整体在 $(...) 里执行，断言输出走 stderr，只把令牌行留在 stdout。
register_via_redis() {
  local email="$1" name="$2" code
  REQ POST '/api/v1/send-verify-code' "{\"verifier_type\":\"email\",\"verifier\":\"${email}\"}"
  # 先写 Redis 再发信：本地 SMTP 不可达会 500，但码已落进 Redis；429=配额
  assert_any "POST /send-verify-code ${email%%@*}" '200 500 429' >&2
  code=$($REDIS_CMD GET "public:verify_code:${email}:register" 2>/dev/null || true)
  [[ -z "$code" || "$code" == "(nil)" ]] && return 1
  REQ POST '/api/v1/register' "{\"name\":\"${name}\",\"password\":\"$PW\",\"email\":\"${email}\",\"code\":\"$code\"}"
  assert "POST /register ${email%%@*}" 200 0 >&2
  local at rt uid
  at=$(jstr access_token); rt=$(jstr refresh_token); uid=$(jnum user_id)
  [[ -z "$at" ]] && return 1
  printf '%s %s %s' "$at" "$rt" "$uid"
}

# login <email> <password> —— stdout 输出 "access refresh"
login_as() {
  REQ POST '/api/v1/login' "{\"username\":\"$1\",\"password\":\"$2\"}"
  assert "POST /login ${1%%@*}" 200 0 >&2 || return 1
  printf '%s %s' "$(jstr access_token)" "$(jstr refresh_token)"
}

# ── 0. 基础与文档端点 ─────────────────────────────────────────────
printf '巡检目标：%s\n\n' "$BASE_URL"
wait_ready || exit 1

REQ GET '/health'
assert 'GET /health' 200
REQ GET '/health/ready'
assert 'GET /health/ready' 200

if [[ "${SKIP_DOCS:-0}" != "1" ]]; then
  REQ GET '/openapi.json'
  assert 'GET /openapi.json (200-only)' 200
  REQ GET '/docs'
  assert_any 'GET /docs (200/302)' '200 302'
fi

# ── 1. 未鉴权契约：参数校验 400 / 需登录 401 ──────────────────────
REQ POST '/api/v1/login' '{"username":"nobody-x","password":"***"}'
assert 'POST /login 不存在用户 → 401' 401
REQ POST '/api/v1/login' '{"username":""}'
assert 'POST /login 参数缺失 → 400' 400
REQ POST '/api/v1/register' '{"name":"ab","password":"***","email":"bad","code":"1"}'
assert 'POST /register 参数非法 → 400' 400

REQ POST '/api/v1/refresh' '{"refresh_token":"***"}'
# 缺陷 #105：契约（swag @Failure 401 与 README）承诺无效令牌返回 401，
# 实际 failAuth 未映射 utils.ErrTokenInvalid，落到 failInternal 返回 500。
if [[ "$CODE" == "500" ]]; then
  known 'POST /refresh 坏令牌 → 401（已知缺陷 #105，现返回 500）' '修复后本条转 PASS，再把 KNOWN 分支删掉'
else
  assert 'POST /refresh 坏令牌 → 401' 401
fi

REQ GET '/api/v1/me'
assert 'GET /me 无令牌 → 401' 401 4002
REQ PUT '/api/v1/me' '{"class":"c"}'
assert 'PUT /me 无令牌 → 401' 401
REQ POST '/api/v1/change-password' '{"old_password":"a","new_password":"bbbbbb"}'
assert 'POST /change-password 无令牌 → 401' 401
REQ GET '/api/v1/recruitment/directions'
assert 'GET /recruitment/directions 无令牌 → 401' 401
REQ GET '/api/v1/admin/users'
assert 'GET /admin/users 无令牌 → 401' 401
REQ GET '/api/v1/admin/mails'
assert 'GET /admin/mails 无令牌 → 401' 401
REQ GET '/api/v1/admin/invitations'
assert 'GET /admin/invitations 无令牌 → 401' 401
REQ GET '/api/v1/me/invitations'
assert 'GET /me/invitations 无令牌 → 401' 401
# logout 是公开路由：先绑 body 的 refresh_token，再由服务层验令牌。
# 无 header 时按实现返回 400（无效令牌）或 401 都算守住鉴权语义。
REQ POST '/api/v1/logout' '{"refresh_token":"***"}'
assert_any 'POST /logout 无令牌 → 400/401' '400 401'

# ── 2. 真实注册两个用户（A→候选管理员，B→普通成员）───────────────
REG_A=$(register_via_redis "$EMAIL_A" "apitest-a-${STAMP}") || REG_A=""
if [[ -z "$REG_A" ]]; then
  printf '\n注册 A 失败，认证链无法继续，终止\n' >&2
  printf '汇总：PASS=%d FAIL=%d SKIP=%d KNOWN=%d\n' "$PASS" "$FAIL" "$SKIP_N" "$KNOWN_N"
  exit 1
fi
read -r TOKEN_A REFRESH_A UID_A <<<"$REG_A"
ok "Redis 取码完成注册 A（userID=${UID_A}），不依赖邮件送达"

REQ POST '/api/v1/register' "{\"name\":\"apitest-a2-${STAMP}\",\"password\":\"$PW\",\"email\":\"${EMAIL_A}\",\"code\":\"000000\"}"
assert 'POST /register 邮箱重复/坏码 → 400' 400

REQ POST '/api/v1/login' "{\"username\":\"${EMAIL_A}\",\"password\":\"$PW\"}"
assert 'POST /login（邮箱登录）→ 200' 200 0
TOKEN_A=$(jstr access_token); REFRESH_A=$(jstr refresh_token)

REQ GET '/api/v1/me' '' "$TOKEN_A"
assert 'GET /me → 200' 200 0
UID_A=$(jnum user_id)
[[ "$(jstr role)" == "member" ]] && ok 'GET /me 新用户 role=member' || bad 'GET /me role=member' "实际 role=$(jstr role)"

REQ PUT '/api/v1/me' '{"class":"xj-class-1","qq":"12345"}' "$TOKEN_A"
assert 'PUT /me 更新资料 → 200' 200 0
REQ GET '/api/v1/me' '' "$TOKEN_A"
if printf '%s' "$RBODY" | grep -q 'xj-class-1'; then
  ok 'PUT /me 回显验证（载荷用 ASCII，避开控制台编码假阴性）'
else
  bad 'PUT /me 回显验证' "GET /me 看不到新 class；body: $(printf '%.160s' "$RBODY")"
fi

REG_B=$(register_via_redis "$EMAIL_B" "apitest-b-${STAMP}") || REG_B=""
if [[ -n "$REG_B" ]]; then
  read -r TOKEN_B REFRESH_B UID_B <<<"$REG_B"
  REQ GET '/api/v1/admin/users' '' "$TOKEN_B"
  assert 'GET /admin/users 成员 → 403' 403 4003
else
  skip '成员 403 断言' 'B 注册未成功（配额/冷却），改由 A 撤权路径覆盖'
  TOKEN_B="$TOKEN_A"; UID_B="$UID_A"
fi

# ── 3. 提权 A 为管理员（Redis 真源 SADD）→ 管理端 ─────────────────
$REDIS_CMD SADD auth:admins "$UID_A" >/dev/null 2>&1 || true
REQ GET '/api/v1/me' '' "$TOKEN_A"
if [[ "$(jstr role)" == "admin" ]]; then
  ok 'SADD 后 /me role=admin（现查 Redis 名单）'
else
  skip 'SADD 后 /me role=admin' '下一步管理端 200 才是真正的权限验证'
fi

REQ GET '/api/v1/admin/users?page=1&page_size=10' '' "$TOKEN_A"
assert 'GET /admin/users 管理员 → 200' 200 0
REQ GET '/api/v1/admin/admins' '' "$TOKEN_A"
assert 'GET /admin/admins → 200' 200 0

# 授予/撤销管理员闭环。注意：Grant/Revoke 按设计会撤销目标用户全部会话
# （AdminMutationResp.session_revoked），所以 B 令牌之后必然 401 ——
# 这是契约行为，不是缺陷；403 判定必须在授予之前（上一段）或 B 重新登录之后。
if [[ -n "$REG_B" ]]; then
  REQ POST '/api/v1/admin/admins' "{\"user_id\":${UID_B}}" "$TOKEN_A"
  assert 'POST /admin/admins 授予 B → 200' 200 0
  REQ GET '/api/v1/admin/admins' '' "$TOKEN_B"
  assert '授予后 B 旧令牌 → 401（会话按契约被撤销）' 401
  REQ DELETE "/api/v1/admin/admins/${UID_B}" '' "$TOKEN_A"
  assert 'DELETE /admin/admins/:user_id 撤销 → 200' 200 0
  # B 重新登录：此刻 B 已不在 Redis 名单，管理端应 403
  REG_B2=$(login_as "$EMAIL_B" "$PW") || REG_B2=""
  if [[ -n "$REG_B2" ]]; then
    read -r TOKEN_B _ <<<"$REG_B2"
    REQ GET '/api/v1/admin/admins' '' "$TOKEN_B"
    assert '撤销后 B 新令牌访问管理端 → 403（Redis 实时判定）' 403 4003
  else
    skip '撤销后 403 断言' 'B 无法重新登录'
  fi
else
  REQ POST '/api/v1/admin/admins' '{"user_id":999999}' "$TOKEN_A"
  assert_any 'POST /admin/admins 坏 ID（B 缺席回退路径）' '200 400'
fi

# 邮件模板 CRUD
REQ POST '/api/v1/admin/mail-templates' '{"name":"api-test-invitation","type":"invitation","title":"Hello {{name}}","mail_model":"Hi {{name}}, welcome.","mail_example":"Hi Bob, welcome."}' "$TOKEN_A"
assert 'POST /admin/mail-templates 创建 → 200' 200 0
TPL_ID=$(jnum id)
REQ GET '/api/v1/admin/mail-templates' '' "$TOKEN_A"
assert 'GET /admin/mail-templates 列表 → 200' 200 0
REQ PUT "/api/v1/admin/mail-templates/${TPL_ID:-1}" '{"name":"api-test-invitation","type":"invitation","title":"Hello {{name}} v2","mail_model":"Hi {{name}}, welcome v2."}' "$TOKEN_A"
assert 'PUT /admin/mail-templates/:id 更新 → 200' 200 0
REQ PUT '/api/v1/admin/mail-templates/999999' '{"name":"api-test-missing","type":"custom","title":"t","mail_model":"m"}' "$TOKEN_A"
assert 'PUT /admin/mail-templates/:id 不存在 → 400' 400

# 发信类：只验鉴权+校验路径（Q3 裁决：不触发真实 SMTP）
REQ POST '/api/v1/admin/mails/send' '{"template_id":999999,"to_user_id":1}' "$TOKEN_A"
assert 'POST /admin/mails/send 模板不存在 → 400' 400
REQ POST '/api/v1/admin/mails/send' '{"to_user_id":1}' "$TOKEN_A"
assert 'POST /admin/mails/send 参数缺失 → 400' 400
REQ POST '/api/v1/admin/mails/send-by-email' '{"template_id":999999,"email":"x@y.local"}' "$TOKEN_A"
assert 'POST /admin/mails/send-by-email 模板不存在 → 400' 400
REQ POST '/api/v1/admin/mails/send-bulk' '{"template_id":999999,"recipients":[{"to_user_id":1}]}' "$TOKEN_A"
assert 'POST /admin/mails/send-bulk 模板不存在 → 400' 400
REQ GET '/api/v1/admin/mails?page=1' '' "$TOKEN_A"
assert 'GET /admin/mails 记录列表 → 200' 200 0
REQ GET '/api/v1/admin/mails?from=bad-date' '' "$TOKEN_A"
assert 'GET /admin/mails 非法日期 → 400' 400
REQ DELETE '/api/v1/admin/mails/batch-delete' '{"ids":[]}' "$TOKEN_A"
assert 'DELETE /admin/mails/batch-delete 空 ids → 400' 400

# ── 4. 招新闭环（#88/#97/#99；命中缺陷 #106 时标 KNOWN）───────────
REQ GET '/api/v1/recruitment/directions' '' "$TOKEN_B"
assert_or_known 'GET /recruitment/directions → 200' 200 '#106'
REQ GET '/api/v1/admin/recruitment/rounds' '' "$TOKEN_A"
assert_or_known 'GET /admin/recruitment/rounds → 200' 200 '#106'
ROUND_ID=$(printf '%s' "$RBODY" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

REQ PATCH "/api/v1/admin/recruitment/rounds/${ROUND_ID:-1}" '{"accepting":true}' "$TOKEN_A"
assert_or_known 'PATCH /admin/recruitment/rounds/:id 开放 → 200' 200 '#106'
REQ PATCH '/api/v1/admin/recruitment/directions/front-end' '{"open":true}' "$TOKEN_A"
assert_or_known 'PATCH /admin/recruitment/directions/:code 开放 → 200' 200 '#106'

REQ POST '/api/v1/recruitment/applications' '{"direction_code":"front-end"}' "$TOKEN_B"
assert_or_known 'POST /recruitment/applications 报名 → 200' 200 '#106'
APP_ID=$(jnum id)
REQ POST '/api/v1/recruitment/applications' '{"direction_code":"front-end"}' "$TOKEN_B"
assert_or_known 'POST /recruitment/applications 重复报名 → 400（业务错误类，500 归 #106）' 400 '#106'
REQ POST '/api/v1/recruitment/applications' '{"direction_code":"not-a-dir"}' "$TOKEN_B"
assert_or_known 'POST /recruitment/applications 坏方向 → 400' 400 '#106'

REQ GET '/api/v1/recruitment/applications' '' "$TOKEN_B"
assert_or_known 'GET /recruitment/applications 本人列表 → 200' 200 '#106'
REQ GET "/api/v1/recruitment/applications/${APP_ID:-1}" '' "$TOKEN_B"
assert_or_known 'GET /recruitment/applications/:id 本人详情 → 200' 200 '#106'

REQ GET '/api/v1/admin/recruitment/applications' '' "$TOKEN_A"
assert_or_known 'GET /admin/recruitment/applications → 200' 200 '#106'
REQ GET "/api/v1/admin/recruitment/applications/${APP_ID:-1}" '' "$TOKEN_A"
assert_or_known 'GET /admin/recruitment/applications/:id → 200' 200 '#106'
REQ PATCH "/api/v1/admin/recruitment/applications/${APP_ID:-1}/review" '{"target":"reviewing"}' "$TOKEN_A"
assert_or_known 'PATCH review applied→reviewing → 200' 200 '#106'
REQ PATCH "/api/v1/admin/recruitment/applications/${APP_ID:-1}/review" '{"target":"accepted"}' "$TOKEN_A"
assert_or_known 'PATCH review reviewing→accepted → 200' 200 '#106'
REQ PATCH "/api/v1/admin/recruitment/applications/${APP_ID:-1}/review" '{"target":"reviewing"}' "$TOKEN_A"
assert_or_known 'PATCH review 非法回退 → 400' 400 '#106'

REQ GET '/api/v1/admin/invitations' '' "$TOKEN_A"
assert_or_known 'GET /admin/invitations → 200' 200 '#106'
REQ GET '/api/v1/admin/candidates' '' "$TOKEN_A"
assert_or_known 'GET /admin/candidates → 200' 200 '#106'
REQ GET '/api/v1/admin/candidates/999999' '' "$TOKEN_A"
if [[ "$CODE" == "500" && "$RBODY" == *"does not exist"* ]]; then
  known 'GET /admin/candidates/:id' '命中缺陷 #106（candidates 表名不匹配）'
else
  assert_any 'GET /admin/candidates/:id 不存在 → 400/404' '400 404'
fi
REQ POST '/api/v1/admin/invitations/send' '{"application_id":999999,"template_id":999999}' "$TOKEN_A"
assert_or_known 'POST /admin/invitations/send 坏引用 → 400（500 归 #106）' 400 '#106'
REQ POST '/api/v1/admin/candidates/999999/advance' '{"action":"hire"}' "$TOKEN_A"
if [[ "$CODE" == "500" && "$RBODY" == *"does not exist"* ]]; then
  known 'POST /admin/candidates/:id/advance' '命中缺陷 #106（candidates 表名不匹配）'
else
  assert_any 'POST /admin/candidates/:id/advance 不存在 → 400/404' '400 404'
fi
REQ GET '/api/v1/me/invitations' '' "$TOKEN_B"
assert_or_known 'GET /me/invitations → 200' 200 '#106'
REQ POST '/api/v1/me/invitations/accept' '{"token":"invalid-token-value"}' "$TOKEN_B"
assert_or_known 'POST /me/invitations/accept 坏 token → 400' 400 '#106'
REQ GET '/api/v1/me/candidates' '' "$TOKEN_B"
assert_or_known 'GET /me/candidates → 200' 200 '#106'

# 收尾删掉巡检模板
REQ DELETE "/api/v1/admin/mail-templates/${TPL_ID:-1}" '' "$TOKEN_A"
assert 'DELETE /admin/mail-templates/:id → 200' 200 0

# ── 5. 令牌生命周期（放最后：会破坏现有会话）─────────────────────
REQ POST '/api/v1/change-password' "{\"old_password\":\"$PW\",\"new_password\":\"$PW2\"}" "$TOKEN_A"
assert 'POST /change-password → 200' 200 0
REQ GET '/api/v1/me' '' "$TOKEN_A"
assert '改密后旧令牌 → 401（会话全部撤销）' 401

REQ POST '/api/v1/login' "{\"username\":\"${EMAIL_A}\",\"password\":\"$PW2\"}"
assert '新密码登录 → 200' 200 0
TOKEN_A=$(jstr access_token); REFRESH_A=$(jstr refresh_token)

REQ POST '/api/v1/refresh' "{\"refresh_token\":\"$REFRESH_A\"}"
assert 'POST /refresh → 200' 200 0
TOKEN_A2=$(jstr access_token); REFRESH_A2=$(jstr refresh_token)
REQ POST '/api/v1/refresh' "{\"refresh_token\":\"$REFRESH_A\"}"
assert 'POST /refresh 旧令牌复用 → 401（单端轮换）' 401

REQ POST '/api/v1/logout' "{\"refresh_token\":\"$REFRESH_A2\"}" "$TOKEN_A2"
assert_any 'POST /logout → 200/401' '200 401'
REQ GET '/api/v1/me' '' "$TOKEN_A2"
assert 'logout 后 access 令牌 → 401' 401

# reset-password 闭环（读 Redis 里的 reset 码，不依赖邮件送达）
REQ POST '/api/v1/send-reset-code' "{\"email\":\"${EMAIL_A}\"}"
assert_any 'POST /send-reset-code' '200 500 429'
CODE_R=$($REDIS_CMD GET "public:verify_code:${EMAIL_A}:reset" 2>/dev/null || true)
if [[ -n "$CODE_R" && "$CODE_R" != "(nil)" ]]; then
  REQ POST '/api/v1/reset-password' "{\"email\":\"${EMAIL_A}\",\"code\":\"$CODE_R\",\"new_password\":\"$PW\"}"
  assert 'POST /reset-password → 200' 200 0
  REQ POST '/api/v1/login' "{\"username\":\"${EMAIL_A}\",\"password\":\"$PW\"}"
  assert '重置后新密码登录 → 200' 200 0
else
  skip 'POST /reset-password 闭环' 'Redis 无 reset 码（IP 配额 429 或发送失败未落码）'
fi

# ── 汇总 ──────────────────────────────────────────────────────────
printf '\n══════════════════════════════\n'
printf '汇总：PASS=%d  FAIL=%d  SKIP=%d  KNOWN=%d\n' "$PASS" "$FAIL" "$SKIP_N" "$KNOWN_N"
[[ "$FAIL" -gt 0 ]] && exit 1 || exit 0
