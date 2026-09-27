#!/usr/bin/env bash
# Issue #82 R5 browser re-verification seed (r5-verify round, Task 16/17).
# Consumes the SAME hardened conventions the r4-flow3 seed established
# (OCR r2 fixes applied): REQUIRED envs with pre-flight guards, jq-built
# payloads with the password on STDIN, REDACTED login captures, sqlite3
# ?1 parameter binding, asserted registrations and tenant serials.
#
# Registers FLOW82_PAD_COUNT placeholders (fresh db: pad 0 is enough for an
# isolated db — the default keeps the r4 shape) then the two protagonists
# ({FLOW82_EMAIL_PREFIX}-a = browser face, -b = plan publisher). Grants b
# the platform-scope plan_publish capability, drafts+publishes the plan and
# asserts the publication landed in Lago (:48889).
#
# Credentials are env-injected (no literals in source): FLOW82_R5_PW.
set -euo pipefail
cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script
BACKEND=http://127.0.0.1:8093
PW="${FLOW82_R5_PW:?missing required env FLOW82_R5_PW}"
# (OCR r2 discipline) DB_PATH pre-flight BEFORE any registration side
# effect; a nonexistent path would let sqlite3 mint an empty db silently.
DB="${DB_PATH:?missing required env DB_PATH (this round: data/issue82-r5verify.db)}"
[ -f "$DB" ] || { echo "FAIL: DB_PATH=$DB not found (start the backend first)" >&2; exit 1; }
PREFIX="${FLOW82_EMAIL_PREFIX:?missing required env FLOW82_EMAIL_PREFIX (e.g. settle-r5)}"
# The plan face this round publishes (parameterized; the browser legs'
# FLOW82_EXPECT_CNY must agree: ¥${PLAN_FEN}/100).
PLAN_KEY="${PLAN_KEY:-pro}"
PLAN_FEN="${PLAN_FEN:-9900}"

: > "$EV/seed-run.txt"
say() { echo "$@" | tee -a "$EV/seed-run.txt"; }

reg() { # reg <email>
  jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}' \
    | curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
        -H 'Content-Type: application/json' --data @-
}

reg_expect() { # reg_expect <label> <email>
  local label="$1" code
  code=$(reg "$2") || true   # keep the 000 diagnosis reachable
  case "$code" in
    2*) say "$label: HTTP $code (registered)" ;;
    409) say "$label: HTTP 409 (already registered — documented rerun tolerance, the login below reuses this account)" ;;
    *) say "FAIL: $label register answered HTTP $code (expected 2xx; only the documented 409 rerun shape is tolerated)"; exit 1 ;;
  esac
}

PAD="${FLOW82_PAD_COUNT:-2}"
case "$PAD" in ''|*[!0-9]*) say "FAIL: FLOW82_PAD_COUNT must be a positive integer, got '$PAD'"; exit 1 ;; esac
[ "$PAD" -ge 1 ] || { say "FAIL: FLOW82_PAD_COUNT must be >= 1, got '$PAD'"; exit 1; }
say "== register $PAD placeholders =="
for i in $(seq 1 "$PAD"); do
  reg_expect "placeholder-$i" "$PREFIX-pad$i@verify.local"
done

say "== register protagonists =="
reg_expect "a (browser)" "$PREFIX-a@verify.local"
reg_expect "b (publisher)" "$PREFIX-b@verify.local"

login() { # login <email> -> token
  jq -nc --arg e "$1" --arg p "$PW" '{email:$e,password:$p}' \
    | curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' --data @-
}
LOGIN_A=$(login "$PREFIX-a@verify.local") || true
LOGIN_B=$(login "$PREFIX-b@verify.local") || true
# Login captures land on disk REDACTED — the JWTs live only in the shell
# variables for this run's own API calls (branch redline).
jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"
TOKEN_A=$(jq -r .token <<<"$LOGIN_A")
TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
UID_B=$(jq -r .user.id <<<"$LOGIN_B")
TENANT_A=$(jq -r .active_tenant.id <<<"$LOGIN_A")
TENANT_B=$(jq -r .active_tenant.id <<<"$LOGIN_B")
say "tenant A (browser protagonist) = $TENANT_A"
say "tenant B (publisher) = $TENANT_B, user id = $UID_B"
for v in "$TOKEN_A" "$TOKEN_B" "$TENANT_A" "$TENANT_B"; do
  [ -n "$v" ] && [ "$v" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }
done
[ "$TENANT_A" -eq $((PAD + 1)) ] && [ "$TENANT_B" -eq $((PAD + 2)) ] \
  || { say "FAIL: protagonists on unexpected tenants A=$TENANT_A B=$TENANT_B (expected $((PAD+1))/$((PAD+2))) — concurrent registration may have broken round isolation"; exit 1; }

uuid_shape() { [[ "$1" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; }
uuid_shape "$UID_B" || { say "FAIL: login response user id is not a UUID ('$UID_B') — refusing further processing"; exit 1; }

say "== grant plan_publish to B at platform scope (seed row) =="
# (safety constraint) ?1 parameter binding — no interpolation of external
# input into the SQL statement (UUID whitelist stays as defense in depth).
sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, ?1, 'plan_publish', 'flow-verifier-r5', 1);" "$UID_B"
say "granted: $(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id=?1;" "$UID_B") row(s)"

say "== read Lago plan baseline (BEFORE this round's publish) =="
LAGO_KEY=$(docker exec weknora-lago-82r5-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')
[ -n "$LAGO_KEY" ] || { say "FAIL: no lago api key readable from weknora-lago-82r5-db-1"; exit 1; }
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
BASE_N=$(jq --argjson fen "$PLAN_FEN" '[.plans[] | select(.amount_cents==$fen)] | length' "$EV/api-03-lago-plans-baseline.json")
say "baseline plans at ${PLAN_FEN}fen already in Lago: $BASE_N"

say "== draft + publish the plan =="
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts" \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' \
  --data "$(jq -nc --arg k "$PLAN_KEY" --argjson fen "$PLAN_FEN" \
    '{plan_key:$k,name:($k+" Plan"),amount_fen:$fen,currency:"CNY",included_credits_micro:($fen*1000),features:{advanced_models:true},limits:{},charges:[]}')" \
  | tee "$EV/api-01-draft.json" | jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' | say "draft: $(cat)"
DRAFT_V=$(jq -r '.data.version // 0' "$EV/api-01-draft.json")
jq -e --arg k "$PLAN_KEY" '.success == true and .data.plan_key == $k and .data.version >= 1' "$EV/api-01-draft.json" >/dev/null \
  || { say "FAIL: plan draft rejected (see api-01-draft.json)"; exit 1; }
curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts/$PLAN_KEY/$DRAFT_V/publish" \
  -H "Authorization: Bearer $TOKEN_B" | tee "$EV/api-02-publish.json" | jq -c '{success, receipt:.data.receipt.command_key, state:.data.state}' | say "publish: $(cat)"
jq -e ".data.receipt.command_key == \"publish_plan_version:$PLAN_KEY:$DRAFT_V\"" "$EV/api-02-publish.json" >/dev/null \
  || { say "FAIL: publish receipt mismatch"; exit 1; }

say "== assert publication landed in Lago (:48889) =="
curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans.json"
N=$(jq --argjson fen "$PLAN_FEN" '[.plans[] | select(.amount_cents==$fen)] | length' "$EV/api-03-lago-plans.json")
if [ "$N" -gt "$BASE_N" ]; then
  say "lago ${PLAN_FEN}fen plan count increased ($BASE_N -> $N): THIS round's publish arrived"
elif [ "$N" -ge 1 ]; then
  say "FAIL: lago plan count unchanged at $BASE_N — residue satisfies existence but not THIS round's arrival"; exit 1
else
  say "FAIL: no plan visible in Lago"; exit 1
fi
say "SEED OK (tenants A=$TENANT_A B=$TENANT_B)"
