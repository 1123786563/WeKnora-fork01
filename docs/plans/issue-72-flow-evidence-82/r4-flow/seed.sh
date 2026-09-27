#!/usr/bin/env bash
# Issue #82 R-4 re-verification seed (integration worktree backend :8093).
# Registers 6 placeholder tenants (their Lago identities weknora-tenant-1..6
# are already occupied on the 82flow stack by earlier rounds — placeholders
# absorb those ids so the protagonists land on fresh Lago identities), then
# the two protagonists: settle-r4-a@verify.local (browser face) and
# settle-r4-b@verify.local (plan publisher). Grants b the platform-scope
# plan_publish capability, drafts+publishes plan `pro` v1 and asserts the
# publication landed in Lago (:48889).
#
# Credentials are env-injected (no literals in source): FLOW82_R4_PW.
set -euo pipefail
cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
BACKEND=http://127.0.0.1:8093
PW="${FLOW82_R4_PW:?missing required env FLOW82_R4_PW}"
# (OCR r2) DB_PATH pre-flight, BEFORE any registration side effect: unset
# would abort late with "unbound variable" after the pads registered; a
# nonexistent path would let sqlite3 silently mint an EMPTY db ("no such
# table") and the grant would write nowhere meaningful.
DB="${DB_PATH:?missing required env DB_PATH (this round's sqlite db per its README)}"
[ -f "$DB" ] || { echo "FAIL: DB_PATH=$DB not found" >&2; exit 1; }
# (OCR r2) The protagonists' email prefix is REQUIRED injection: the
# historical default collided head-on with round 1's accounts (409
# tolerance would silently reuse them — cross-round data bleed).
PREFIX="${FLOW82_EMAIL_PREFIX:?missing required env FLOW82_EMAIL_PREFIX (this round's unique prefix)}"

: > "$EV/seed-run.txt"
say() { echo "$@" | tee -a "$EV/seed-run.txt"; }

reg() { # reg <email>
  # (OCR r2) The payload is jq-built and the password rides STDIN (--data
  # @-): manual JSON escaping breaks on quotes/backslashes in PW, and a
  # curl argv password is observable in local ps.
  jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}' \
    | curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
        -H 'Content-Type: application/json' --data @-
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
  # (OCR r2) || true keeps the 000 diagnosis reachable: a connection-refused
  # curl exits non-zero and set -e would otherwise abort BEFORE the case
  # below can report the diagnostic code.
  code=$(reg "$2") || true
  case "$code" in
    2*) say "$label: HTTP $code (registered)" ;;
    409) say "$label: HTTP 409 (already registered — documented rerun tolerance, the login below reuses this account)" ;;
    *) say "FAIL: $label register answered HTTP $code (expected 2xx; only the documented 409 rerun shape is tolerated)"; exit 1 ;;
  esac
}

# FLOW82_PAD_COUNT placeholders absorb the Lago tenant ids earlier rounds
# already occupy on the 82flow stack (round 1: 1..6, round 2: 1..8);
# FLOW82_EMAIL_PREFIX namespaces the protagonists per round.
# (A-10) The default pad count records THIS round's requirement (this round IS round 1 of the r4 series: pads 1..6 (its own header and r4-flow2's ledger record round 1 as 1..6));
# a stale default (6 left over from an earlier round) would under-pad and
# land a protagonist on an occupied Lago tenant id — cross-round data bleed.
PAD="${FLOW82_PAD_COUNT:-6}"
# (OCR r2) PAD must be a positive integer — a stray non-numeric value would
# either expand weirdly in seq or silently under/over-pad the round.
case "$PAD" in ''|*[!0-9]*) say "FAIL: FLOW82_PAD_COUNT must be a positive integer, got '$PAD'"; exit 1 ;; esac
[ "$PAD" -ge 1 ] || { say "FAIL: FLOW82_PAD_COUNT must be >= 1, got '$PAD'"; exit 1; }
say "== register $PAD placeholders (absorb Lago tenant ids) =="
for i in $(seq 1 "$PAD"); do
  reg_expect "placeholder-$i" "$PREFIX-pad$i@verify.local"
done

say "== register protagonists =="
reg_expect "a (browser)" "$PREFIX-a@verify.local"
reg_expect "b (publisher)" "$PREFIX-b@verify.local"

login() { # login <email> -> token
  # (OCR r2) jq-built payload, password via STDIN — same posture as reg().
  jq -nc --arg e "$1" --arg p "$PW" '{email:$e,password:$p}' \
    | curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' --data @-
}
# (C-83/C-01) Login captures land on disk REDACTED — the access/refresh
# JWTs live only in the shell variables for this run's own API calls; no
# usable credential literal ever reaches a file (branch redline).
# (OCR r2) || true: a transport-failed login (empty body, curl non-zero)
# must fall through to the explicit assertions below, not abort set -e
# style with no diagnosis.
LOGIN_A=$(login "$PREFIX-a@verify.local") || true
LOGIN_B=$(login "$PREFIX-b@verify.local") || true
jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"
TOKEN_A=$(jq -r .token <<<"$LOGIN_A")
TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
UID_B=$(jq -r .user.id "$EV/seed-login-b.json")
TENANT_A=$(jq -r .active_tenant.id "$EV/seed-login-a.json")
TENANT_B=$(jq -r .active_tenant.id "$EV/seed-login-b.json")
say "tenant A (browser protagonist) = $TENANT_A"
say "tenant B (publisher) = $TENANT_B, user id = $UID_B"
for v in "$TENANT_A" "$TENANT_B"; do
  # (OCR r2) -n covers the empty-string shape jq emits for a present-but-
  # blank field (the old != "null" check alone let it through).
  [ -n "$v" ] && [ "$v" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }
done
# (OCR r2) Round-isolation proof: on this round's OWN fresh db the
# protagonists land exactly on pad+1 / pad+2 — any other serial means a
# concurrent registration broke the isolation this evidence depends on.
[ "$TENANT_A" -eq $((PAD + 1)) ] && [ "$TENANT_B" -eq $((PAD + 2)) ] \
  || { say "FAIL: protagonists on unexpected tenants A=$TENANT_A B=$TENANT_B (expected $((PAD+1))/$((PAD+2))) — concurrent registration may have broken round isolation"; exit 1; }

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
# (OCR r2 / safety constraint) The SQL binds ?1 parameters — no string
# interpolation of external input into the statement (the UUID whitelist
# above stays as defense in depth).
sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, ?1, 'plan_publish', 'flow-verifier-r4', 1);" "$UID_B"
say "granted: $(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id=?1;" "$UID_B") row(s)"

say "== read Lago 9900-plan baseline (BEFORE this round's publish) =="
# (A-09) The 82flow Lago stack is shared across rounds — "a 9900 plan
# exists" is satisfiable by EARLIER rounds' residue. This round's publish
# arrival is proven by a STRICT count increase over the pre-publish
# baseline read here.
# (OCR r2) The newest key wins (ORDER BY created_at) and an empty read
# fails explicitly — limit-1-without-order picked an arbitrary key and a
# failed read only surfaced as obscure downstream jq assertion noise.
LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')
[ -n "$LAGO_KEY" ] || { say "FAIL: no lago api key readable from weknora-lago-82flow-db-1"; exit 1; }
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
