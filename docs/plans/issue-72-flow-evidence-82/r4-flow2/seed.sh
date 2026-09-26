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

# (A-13) Registration responses are ASSERTED, never merely printed: a failed
# placeholder registration (backend down -> 000, 4xx/5xx) would otherwise let
# the protagonists land on Lago tenant ids earlier rounds already occupy —
# silently breaking the round isolation this script's evidence depends on.
# (The say pipeline's exit status rides its LAST command, so set -e never
# sees a curl failure inside a command substitution — the assertion must be
# explicit.) 409 is the one explicitly tolerated shape: a rerun
# re-registering an existing email keeps the EXISTING account (the login
# below then reuses it — same placeholder semantics, same tenants).
reg_expect() { # reg_expect <label> <email>
  local label="$1" code
  code=$(reg "$2")
  case "$code" in
    2*) say "$label: HTTP $code (registered)" ;;
    409) say "$label: HTTP 409 (already registered — documented rerun tolerance, the login below reuses this account)" ;;
    *) say "FAIL: $label register answered HTTP $code (expected 2xx; only the documented 409 rerun shape is tolerated)"; exit 1 ;;
  esac
}

# FLOW82_PAD_COUNT placeholders absorb the Lago tenant ids earlier rounds
# already occupy on the 82flow stack (round 1: 1..6, round 2: 1..8);
# FLOW82_EMAIL_PREFIX namespaces the protagonists per round.
# (A-10) The default pad count records THIS round's requirement (this round IS round 2: round 1 occupied 1..6, this round pads 1..8 (its own header records 'round 2 uses 1..8'));
# a stale default (6 left over from an earlier round) would under-pad and
# land a protagonist on an occupied Lago tenant id — cross-round data bleed.
PAD="${FLOW82_PAD_COUNT:-8}"
say "== register $PAD placeholders (absorb Lago tenant ids) =="
for i in $(seq 1 "$PAD"); do
  reg_expect "placeholder-$i" "${FLOW82_EMAIL_PREFIX:-settle-r4}-pad$i@verify.local"
done

say "== register protagonists =="
reg_expect "a (browser)" "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"
reg_expect "b (publisher)" "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local"

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

# (A-08) UID_B comes from the backend's login response — EXTERNAL input that
# is interpolated into the sqlite3 CLI calls below (the CLI has no parameter
# binding; a strict UUID shape whitelist is the guard). null/empty or a
# tampered shape (e.g. a quote payload in a forged login response) fails
# fast instead of injecting arbitrary SQL.
uuid_shape() { [[ "$1" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; }
for v in "$TOKEN_A" "$TOKEN_B"; do
  [ -n "$v" ] && [ "$v" != "null" ] || { say "FAIL: login response token missing/null"; exit 1; }
done
uuid_shape "$UID_B" || { say "FAIL: login response user id is not a UUID ('$UID_B') — refusing SQL interpolation"; exit 1; }

say "== grant plan_publish to B at platform scope (seed row) =="
sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
say "granted: $(sqlite3 "$DB_PATH" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"

say "== read Lago 9900-plan baseline (BEFORE this round's publish) =="
# (A-09) The 82flow Lago stack is shared across rounds — "a 9900 plan
# exists" is satisfiable by EARLIER rounds' residue. This round's publish
# arrival is proven by a STRICT count increase over the pre-publish
# baseline read here.
LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]')
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
BASE_N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans-baseline.json")
say "baseline 9900 plans already in Lago: $BASE_N"

say "== draft + publish plan pro v1 (9900 CNY monthly, advanced_models) =="
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts" \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d '{
    "plan_key":"pro","name":"Pro","amount_fen":9900,"currency":"CNY",
    "included_credits_micro":9900000,"features":{"advanced_models":true},
    "limits":{},"charges":[]}' | tee "$EV/api-01-draft.json" | jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' | say "draft: $(cat)"
# (A-14) The draft response is ASSERTED and the publish URL is DRIVEN by the
# draft's answered version: the say pipeline's exit status rides its last
# command, so an unasserted draft failure (401/422/non-JSON) would surface
# only as a misleading publish-receipt mismatch — or worse, publish would
# act on a LEFTOVER v1 draft from an earlier run and its receipt check
# would still pass, minting evidence unrelated to this run's draft.
DRAFT_V=$(jq -r '.data.version // 0' "$EV/api-01-draft.json")
jq -e '.success == true and .data.plan_key == "pro" and .data.version >= 1' "$EV/api-01-draft.json" >/dev/null \
  || { say "FAIL: plan draft rejected (see api-01-draft.json)"; exit 1; }
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts/pro/$DRAFT_V/publish" \
  -H "Authorization: Bearer $TOKEN_B" | tee "$EV/api-02-publish.json" | jq -c '{success, receipt:.data.receipt.command_key, state:.data.state}' | say "publish: $(cat)"
jq -e ".data.receipt.command_key == \"publish_plan_version:pro:$DRAFT_V\"" "$EV/api-02-publish.json" >/dev/null \
  || { say "FAIL: publish receipt mismatch"; exit 1; }

say "== assert publication landed in Lago (:48889) =="
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans.json"
# integration publishes under an external plan code; assert by amount+currency+interval face
# (amount_cents is a JSON number on the wire — compare numerically)
jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json" | say "lago 9900 plans: $(cat)"
N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans.json")
# (A-09) Round-specific proof: a STRICT increase over the pre-publish
# baseline. An unchanged nonzero count means this round's Lago projection
# cannot be proven (earlier-round residue would satisfy the old >=1 check
# with this round's publish never having arrived) — fail honestly instead
# of minting a cross-round false positive.
if [ "$N" -gt "$BASE_N" ]; then
  say "lago 9900 plan count increased ($BASE_N -> $N): THIS round's publish arrived"
elif [ "$N" -ge 1 ]; then
  say "FAIL: lago 9900 plan count unchanged at $BASE_N — earlier-round residue satisfies existence but not THIS round's arrival"; exit 1
else
  say "FAIL: no 9900 plan visible in Lago"; exit 1
fi
say "SEED OK (tenants A=$TENANT_A B=$TENANT_B)"
