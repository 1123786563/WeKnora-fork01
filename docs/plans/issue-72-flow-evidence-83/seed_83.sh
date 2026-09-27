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
DB_PATH=data/issue83-flow.db
PW="${FLOW83_PW:?missing required env FLOW83_PW}"

: > "$EV/seed-run.txt"
say() { echo "$@" | tee -a "$EV/seed-run.txt"; }

reg_expect() { # reg_expect <label> <email>
  local label="$1" code
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$2\",\"email\":\"$2\",\"password\":\"$PW\"}")
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

LOGIN_A=$(curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"issue83-flow-a@verify.local\",\"password\":\"$PW\"}")
LOGIN_B=$(curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"issue83-flow-b@verify.local\",\"password\":\"$PW\"}")
echo "$LOGIN_A" > "$EV/seed-login-a.json"
echo "$LOGIN_B" > "$EV/seed-login-b.json"
TOKEN_A=$(jq -r .token "$EV/seed-login-a.json")
TOKEN_B=$(jq -r .token "$EV/seed-login-b.json")
UID_B=$(jq -r .user.id "$EV/seed-login-b.json")
TENANT_A=$(jq -r .active_tenant.id "$EV/seed-login-a.json")
TENANT_B=$(jq -r .active_tenant.id "$EV/seed-login-b.json")
say "tenant A (browser protagonist) = $TENANT_A"
say "tenant B (publisher) = $TENANT_B, user id = $UID_B"
[ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }

uuid_shape() { [[ "$1" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; }
uuid_shape "$UID_B" || { say "FAIL: user id not a UUID — refusing SQL interpolation"; exit 1; }

say "== grant plan_publish to B at platform scope (seed row) =="
sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-83', 1);"
say "granted: $(sqlite3 "$DB_PATH" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"

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
LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]')
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans.json"
jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json" | say "lago 9900 plans: $(cat)"
jq -e '[.plans[] | select(.amount_cents==9900)] | length >= 1' "$EV/api-03-lago-plans.json" >/dev/null \
  || { say "FAIL: no 9900 plan visible in Lago"; exit 1; }
say "SEED OK (tenants A=$TENANT_A B=$TENANT_B)"
