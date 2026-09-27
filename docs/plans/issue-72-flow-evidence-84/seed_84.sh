#!/usr/bin/env bash
# Issue #84 flow-verification seed（backend :8096, sqlite issue84-flow.db）。
# 照 seed_83.sh 形态：注册主角（a=浏览器/购买人，b=平台发布人）、发布 pro 9900、
# 断言发布落地 Lago（:48889, 82r5 栈）。凭据经 env 注入（FLOW84_PW）。
set -euo pipefail
cd "$(dirname "$0")/../../.."   # worktree root
EV="$(cd "$(dirname "$0")" && pwd)"
BACKEND=http://127.0.0.1:8096
DB_PATH="${FLOW84_DB:-data/issue84-flow.db}"
PW="${FLOW84_PW:?missing required env FLOW84_PW}"
# (r2 复验) plan key env 化：复验轮用新 key，规避共享 Lago 栈上历史轮次的 code 残留。
PLAN_KEY="${FLOW84_PLAN_KEY:-pro}"

: > "$EV/seed-run.txt"
say() { echo "$@" | tee -a "$EV/seed-run.txt"; }

reg_expect() { # reg_expect <label> <email>
  local label="$1" code body
  body=$(curl -s -w '\n%{http_code}' -X POST "$BACKEND/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$2\",\"email\":\"$2\",\"password\":\"$PW\"}") \
    || { say "FAIL: $label register TRANSPORT failure (curl exit $? — backend at $BACKEND reachable?)"; exit 1; }
  code=$(tail -n1 <<<"$body")
  case "$code" in
    2*) say "$label: HTTP $code (registered)" ;;
    # 重注册面：本仓答 400 + "already exists"（email 唯一冲突的产品形态）。
    400) grep -q "already exists" <<<"$body" \
      && say "$label: HTTP 400 already-exists (rerun tolerance)" \
      || { say "FAIL: $label register 400: $(head -n1 <<<"$body")"; exit 1; } ;;
    *) say "FAIL: $label register answered HTTP $code: $(head -n1 <<<"$body")"; exit 1 ;;
  esac
}

say "== register placeholders (absorb Lago tenant ids earlier rounds occupy) =="
# 82/83 轮已用掉 Lago weknora-customer-1..~12+；本轮 FRESH sqlite 从 tenant 1 起，
# 主角必须落在已占范围之后，否则与历史轮次的 purchase 订阅身份碰撞。
PAD="${FLOW84_PAD_COUNT:-16}"
for i in $(seq 1 "$PAD"); do
  reg_expect "placeholder-$i" "issue84-pad$i@verify.local"
done

say "== register protagonists =="
reg_expect "a (browser buyer)" "issue84-flow-a@verify.local"
reg_expect "b (publisher)" "issue84-flow-b@verify.local"

LOGIN_A=$(curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"issue84-flow-a@verify.local\",\"password\":\"$PW\"}") \
  || { say "FAIL: login A TRANSPORT failure (curl exit $? — backend at $BACKEND reachable?)"; exit 1; }
LOGIN_B=$(curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"issue84-flow-b@verify.local\",\"password\":\"$PW\"}") \
  || { say "FAIL: login B TRANSPORT failure (curl exit $?)"; exit 1; }
# 登录落盘一律 REDACTED——JWT 只存活于本轮 shell（凭据纪律）。
redact_login() { jq '.token = "REDACTED" | .refresh_token = "REDACTED"'; }
redact_login <<<"$LOGIN_A" > "$EV/seed-login-a.json"
redact_login <<<"$LOGIN_B" > "$EV/seed-login-b.json"
TOKEN_A=$(jq -r .token <<<"$LOGIN_A")
TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
UID_B=$(jq -r .user.id "$EV/seed-login-b.json")
TENANT_A=$(jq -r .active_tenant.id "$EV/seed-login-a.json")
TENANT_B=$(jq -r .active_tenant.id "$EV/seed-login-b.json")
say "tenant A (browser buyer) = $TENANT_A"
say "tenant B (publisher) = $TENANT_B, user id = $UID_B"
[ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }

uuid_shape() { [[ "$1" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; }
uuid_shape "$UID_B" || { say "FAIL: user id not a UUID — refusing SQL interpolation"; exit 1; }

say "== grants: plan_publish (B) + refund_review (B) at platform scope =="
sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-84', 1);"
sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'refund_review', 'flow-verifier-84', 1);"
say "granted: $(sqlite3 "$DB_PATH" "select count(*) from commercial_grants where user_id='$UID_B';") row(s) for B"

say "== draft + publish plan pro v1 (9900 CNY monthly, advanced_models) =="
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts" \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d "{
    \"plan_key\":\"$PLAN_KEY\",\"name\":\"Pro\",\"amount_fen\":9900,\"currency\":\"CNY\",
    \"included_credits_micro\":9900000,\"features\":{\"advanced_models\":true},
    \"limits\":{},\"charges\":[]}" > "$EV/api-01-draft.json"
DRAFT_V=$(jq -r '.data.version // 0' "$EV/api-01-draft.json")
jq -e --arg pk "$PLAN_KEY" '.success == true and .data.plan_key == $pk and .data.version >= 1' "$EV/api-01-draft.json" >/dev/null \
  || { say "FAIL: plan draft rejected (see api-01-draft.json)"; exit 1; }
say "draft: $(jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' "$EV/api-01-draft.json")"
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts/$PLAN_KEY/$DRAFT_V/publish" \
  -H "Authorization: Bearer $TOKEN_B" > "$EV/api-02-publish.json"
jq -e ".data.receipt.command_key == \"publish_plan_version:$PLAN_KEY:$DRAFT_V\"" "$EV/api-02-publish.json" >/dev/null \
  || { say "FAIL: publish receipt mismatch"; exit 1; }
say "publish: $(jq -c '{success, receipt:.data.receipt.command_key}' "$EV/api-02-publish.json")"

say "== assert publication landed in Lago (:48889, 82r5 stack) =="
LAGO_KEY=$(docker exec weknora-lago-82r5-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]') \
  || { say "FAIL: LAGO_KEY read TRANSPORT failure (docker exec psql exit $? — 82r5 db up?)"; exit 1; }
[ -n "$LAGO_KEY" ] || { say "FAIL: LAGO_KEY read answered empty"; exit 1; }
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans.json"
jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json" | say "lago 9900 plans: $(cat)"
jq -e --arg code "weknora-$PLAN_KEY-v1" '[.plans[] | select(.code==$code and .amount_cents==9900 and .interval=="monthly")] | length == 1' \
  "$EV/api-03-lago-plans.json" >/dev/null \
  || { say "FAIL: plan weknora-$PLAN_KEY-v1 (9900/monthly) not found in Lago"; exit 1; }
say "plan weknora-$PLAN_KEY-v1 present at the exact 9900/monthly face"
say "SEED OK (tenants A=$TENANT_A B=$TENANT_B)"
