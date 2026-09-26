#!/usr/bin/env bash
# Issue #82 R-4 re-verification seed (integration worktree backend :8093).
# Round 2 (post-fix re-verification): FLOW82_PAD_COUNT placeholders absorb
# the Lago tenant ids earlier rounds already occupy on the 82flow stack
# (round 1 used 1..6, round 2 uses 1..8), then the two protagonists
# ({FLOW82_EMAIL_PREFIX}-a = browser face, -b = plan publisher). Grants b
# the platform-scope plan_publish capability, drafts+publishes plan `pro` v1
# and asserts the publication landed in Lago (:48889).
#
# Credentials are env-injected (no literals in source): FLOW82_R4_PW.
set -euo pipefail
cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
BACKEND=http://127.0.0.1:8093
PW="${FLOW82_R4_PW:?missing required env FLOW82_R4_PW}"

: > "$EV/seed-run.txt"
say() { echo "$@" | tee -a "$EV/seed-run.txt"; }

reg() { # reg <email>
  curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$1\",\"email\":\"$1\",\"password\":\"$PW\"}"
}

# FLOW82_PAD_COUNT placeholders absorb the Lago tenant ids earlier rounds
# already occupy on the 82flow stack (round 1: 1..6, round 2: 1..8);
# FLOW82_EMAIL_PREFIX namespaces the protagonists per round.
say "== register ${FLOW82_PAD_COUNT:-6} placeholders (absorb Lago tenant ids) =="
for i in $(seq 1 "${FLOW82_PAD_COUNT:-6}"); do
  say "placeholder-$i: HTTP $(reg "${FLOW82_EMAIL_PREFIX:-settle-r4}-pad$i@verify.local")"
done

say "== register protagonists =="
say "a (browser): HTTP $(reg "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local")"
say "b (publisher): HTTP $(reg "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local")"

login() { # login <email> -> token
  curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' \
    -d "{\"email\":\"$1\",\"password\":\"$PW\"}"
}
LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"); echo "$LOGIN_A" > "$EV/seed-login-a.json"
LOGIN_B=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local"); echo "$LOGIN_B" > "$EV/seed-login-b.json"
TOKEN_A=$(jq -r .token "$EV/seed-login-a.json")
TOKEN_B=$(jq -r .token "$EV/seed-login-b.json")
UID_B=$(jq -r .user.id "$EV/seed-login-b.json")
TENANT_A=$(jq -r .active_tenant.id "$EV/seed-login-a.json")
TENANT_B=$(jq -r .active_tenant.id "$EV/seed-login-b.json")
say "tenant A (browser protagonist) = $TENANT_A"
say "tenant B (publisher) = $TENANT_B, user id = $UID_B"
[ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }

say "== grant plan_publish to B at platform scope (seed row) =="
sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
say "granted: $(sqlite3 "$DB_PATH" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"

say "== draft + publish plan pro v1 (9900 CNY monthly, advanced_models) =="
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts" \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d '{
    "plan_key":"pro","name":"Pro","amount_fen":9900,"currency":"CNY",
    "included_credits_micro":9900000,"features":{"advanced_models":true},
    "limits":{},"charges":[]}' | tee "$EV/api-01-draft.json" | jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' | say "draft: $(cat)"
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts/pro/1/publish" \
  -H "Authorization: Bearer $TOKEN_B" | tee "$EV/api-02-publish.json" | jq -c '{success, receipt:.data.receipt.command_key, state:.data.state}' | say "publish: $(cat)"
jq -e '.data.receipt.command_key == "publish_plan_version:pro:1"' "$EV/api-02-publish.json" >/dev/null \
  || { say "FAIL: publish receipt mismatch"; exit 1; }

say "== assert publication landed in Lago (:48889) =="
LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]')
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans.json"
# integration publishes under an external plan code; assert by amount+currency+interval face
# (amount_cents is a JSON number on the wire — compare numerically)
jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json" | say "lago 9900 plans: $(cat)"
N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans.json")
[ "$N" -ge 1 ] || { say "FAIL: no 9900 plan visible in Lago"; exit 1; }
say "SEED OK (tenants A=$TENANT_A B=$TENANT_B)"
