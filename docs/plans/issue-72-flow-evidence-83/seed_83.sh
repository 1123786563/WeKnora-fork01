#!/usr/bin/env bash
# Issue #83 flow-verification seed (worktree backend :8095, sqlite issue83-flow.db).
# Registers the two protagonists (a = browser face, b = plan publisher with the
# platform-scope plan_publish grant), drafts+publishes plan `pro` v1 and asserts
# the publication landed in Lago (:48889, 82flow stack). The 9900-plan code
# pro:1 already exists from earlier rounds — republishing the same code is the
# documented idempotent path (r4-flow3 verified), so THIS round asserts the
# publish receipt (not a count increase).
#
# Credentials are env-injected (no literals in source): FLOW83_PW.
set -euo pipefail
cd "$(dirname "$0")/../../.."   # worktree root
EV="$(cd "$(dirname "$0")" && pwd)"
BACKEND=http://127.0.0.1:8095
DB_PATH="${FLOW83_DB:-data/issue83-flow.db}"
PW="${FLOW83_PW:?missing required env FLOW83_PW}"

: > "$EV/seed-run.txt"
say() { echo "$@" | tee -a "$EV/seed-run.txt"; }

reg_expect() { # reg_expect <label> <email>
  local label="$1" code
  # (C-12) A transport-layer failure (backend down -> curl exit 7) would
  # otherwise kill the script through set -e with ZERO output — the
  # promised HTTP-status FAIL path is unreachable for it. Fail loudly here.
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$2\",\"email\":\"$2\",\"password\":\"$PW\"}") \
    || { say "FAIL: $label register TRANSPORT failure (curl exit $? — backend at $BACKEND reachable?)"; exit 1; }
  case "$code" in
    2*) say "$label: HTTP $code (registered)" ;;
    409) say "$label: HTTP 409 (already registered — rerun tolerance)" ;;
    *) say "FAIL: $label register answered HTTP $code"; exit 1 ;;
  esac
}

say "== register placeholders (absorb Lago tenant ids earlier 82-rounds occupy) =="
# The 82flow Lago stack already holds weknora-customer-1..12 (82 rounds used
# up to tenant 11/12); this round's FRESH WeKnora sqlite starts at tenant 1,
# so the protagonists MUST land past the occupied range or their Lago
# identity (weknora-customer-<t>/weknora-tenant-<t>-purchase) would collide
# with an earlier round's possibly-active purchase subscription.
PAD="${FLOW83_PAD_COUNT:-14}"
for i in $(seq 1 "$PAD"); do
  reg_expect "placeholder-$i" "issue83-pad$i@verify.local"
done

say "== register protagonists =="
reg_expect "a (browser)" "issue83-flow-a@verify.local"
reg_expect "b (publisher)" "issue83-flow-b@verify.local"

# (C-12) Bare curl in a command substitution + set -e = silent death on a
# transport failure; fail loudly instead.
LOGIN_A=$(curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"issue83-flow-a@verify.local\",\"password\":\"$PW\"}") \
  || { say "FAIL: login A TRANSPORT failure (curl exit $? — backend at $BACKEND reachable?)"; exit 1; }
LOGIN_B=$(curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"issue83-flow-b@verify.local\",\"password\":\"$PW\"}") \
  || { say "FAIL: login B TRANSPORT failure (curl exit $?)"; exit 1; }
# (C-01) The login captures land on disk REDACTED — the access/refresh JWTs
# live ONLY in the shell variables below for this run's own API calls; no
# usable credential literal ever reaches a file (the branch redline). The
# evidence files keep the success/tenant/user shape only.
redact_login() { jq '.token = "REDACTED" | .refresh_token = "REDACTED"'; }
redact_login <<<"$LOGIN_A" > "$EV/seed-login-a.json"
redact_login <<<"$LOGIN_B" > "$EV/seed-login-b.json"
# The main-chain protagonist's login, ALSO landed under the -f name the
# downstream api_recovery_83.mjs reads (act 3/5 — tenant identity only; the
# script takes its API token from the FLOW83_TOKEN env, never this file).
redact_login <<<"$LOGIN_A" > "$EV/seed-login-f.json"
TOKEN_A=$(jq -r .token <<<"$LOGIN_A")
TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
UID_B=$(jq -r .user.id "$EV/seed-login-b.json")
TENANT_A=$(jq -r .active_tenant.id "$EV/seed-login-a.json")
TENANT_B=$(jq -r .active_tenant.id "$EV/seed-login-b.json")
say "tenant A (browser protagonist) = $TENANT_A"
say "tenant B (publisher) = $TENANT_B, user id = $UID_B"
[ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }
# (OCR84-R1-29 / C-11) 断言轮次特定事实：PAD 占位的前提是主角租户号越过共享
# Lago 栈上外部轮已占用的上限——按默认 PAD 重跑时主角落在 tenant 15/16，与
# 外部轮的 weknora-customer-15/16 在 Lago 侧身份碰撞，act3/act5 会对错误租户
# 的 Lago 对象做断言，产生误导性 PASS/FAIL。上限经 env 可调（复验轮实测被占
# 到 34）。
OCCUPIED_MAX="${FLOW83_LAGO_OCCUPIED_MAX:-12}"
case "$OCCUPIED_MAX" in ''|*[!0-9]*) say "FAIL: FLOW83_LAGO_OCCUPIED_MAX must be an integer, got '$OCCUPIED_MAX'"; exit 1 ;; esac
[ "$TENANT_A" -gt "$OCCUPIED_MAX" ] && [ "$TENANT_B" -gt "$OCCUPIED_MAX" ] \
  || { say "FAIL: protagonists landed on tenants A=$TENANT_A B=$TENANT_B — within the externally occupied range (1..$OCCUPIED_MAX); raise FLOW83_PAD_COUNT or set FLOW83_LAGO_OCCUPIED_MAX to the verified bound"; exit 1; }
say "protagonist tenants ($TENANT_A/$TENANT_B) are clear of the occupied range (1..$OCCUPIED_MAX)"

uuid_shape() { [[ "$1" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; }
# (OCR84-R1-17 同型) uuid_shape 保留为纵深防御前置；注入防线是下方 python3
# sqlite3 的 ? 参数绑定。
uuid_shape "$UID_B" || { say "FAIL: user id not a UUID — refusing further processing"; exit 1; }

say "== grant plan_publish to B at platform scope (seed row) =="
GRANT_N=$(python3 - "$DB_PATH" "$UID_B" <<'PY'
import sqlite3, sys
db, uid = sys.argv[1], sys.argv[2]
conn = sqlite3.connect(db)
with conn:
    conn.execute(
        "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version)"
        " values (0, ?, 'plan_publish', 'flow-verifier-83', 1)",
        (uid,),
    )
print(conn.execute(
    "select count(*) from commercial_grants where capability='plan_publish' and user_id = ?",
    (uid,),
).fetchone()[0])
PY
) || { say "FAIL: grant write/read failed (python3 sqlite3, db=$DB_PATH)"; exit 1; }
say "granted: $GRANT_N row(s)"

say "== draft + publish plan pro v1 (9900 CNY monthly, advanced_models) =="
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts" \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d '{
    "plan_key":"pro","name":"Pro","amount_fen":9900,"currency":"CNY",
    "included_credits_micro":9900000,"features":{"advanced_models":true},
    "limits":{},"charges":[]}' > "$EV/api-01-draft.json"
DRAFT_V=$(jq -r '.data.version // 0' "$EV/api-01-draft.json")
jq -e '.success == true and .data.plan_key == "pro" and .data.version >= 1' "$EV/api-01-draft.json" >/dev/null \
  || { say "FAIL: plan draft rejected (see api-01-draft.json)"; exit 1; }
say "draft: $(jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' "$EV/api-01-draft.json")"
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts/pro/$DRAFT_V/publish" \
  -H "Authorization: Bearer $TOKEN_B" > "$EV/api-02-publish.json"
jq -e ".data.receipt.command_key == \"publish_plan_version:pro:$DRAFT_V\"" "$EV/api-02-publish.json" >/dev/null \
  || { say "FAIL: publish receipt mismatch"; exit 1; }
say "publish: $(jq -c '{success, receipt:.data.receipt.command_key}' "$EV/api-02-publish.json")"

say "== assert publication landed in Lago (:48889) =="
# (C-12) Transport failure of the docker exec | tr pipeline (pipefail
# propagates it) would silently kill the script under set -e — fail loudly.
LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]') \
  || { say "FAIL: LAGO_KEY read TRANSPORT failure (docker exec psql exit $? — 82flow db container up?)"; exit 1; }
[ -n "$LAGO_KEY" ] || { say "FAIL: LAGO_KEY read answered empty"; exit 1; }
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans.json"
jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json" | say "lago 9900 plans: $(cat)"
# (C-11) The old `count >= 1` over ANY 9900 plan was satisfiable by earlier
# rounds' residue (pro:1 predates this round on the shared 82flow stack) —
# it never proved THIS round's publish arrived. Republish of the same code
# is idempotent (no count delta available), so the round-specific proof is
# the SHAPE: this round's exact plan code, at this round's frozen
# amount/interval, present in the authority's index.
jq -e '[.plans[] | select(.code=="weknora-pro-v1" and .amount_cents==9900 and .interval=="monthly")] | length == 1' \
  "$EV/api-03-lago-plans.json" >/dev/null \
  || { say "FAIL: plan weknora-pro-v1 (9900/monthly, this round's draft face) not found in Lago"; exit 1; }
say "plan weknora-pro-v1 present at the exact 9900/monthly face (round-specific shape assertion)"
say "SEED OK (tenants A=$TENANT_A B=$TENANT_B)"
