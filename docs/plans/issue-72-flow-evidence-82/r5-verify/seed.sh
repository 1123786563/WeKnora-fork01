#!/usr/bin/env bash
# Issue #82 R5 browser re-verification seed (r5-verify round, Task 16/17).
# Consumes the SAME hardened conventions the r4-flow3 seed established
# (OCR r2 fixes applied): REQUIRED envs with pre-flight guards, jq-built
# payloads with the password on STDIN, REDACTED login captures, python3
# sqlite3 ? parameter binding on the grant insert (OCR84-R1-17 — the
# header's former "?1 parameter binding" claim described code this script
# never had), asserted registrations and tenant serials.
#
# Registers FLOW82_PAD_COUNT placeholders (fresh db: pad 0 is enough for an
# isolated db — the default keeps the r4 shape) then the two protagonists
# ({FLOW82_EMAIL_PREFIX}-a = browser face, -b = plan publisher). Grants b
# the platform-scope plan_publish capability, drafts+publishes the plan and
# asserts the publication landed in Lago (:48889).
#
# Credentials are env-injected (no literals in source): FLOW82_R5_PW.
set -euo pipefail
# (OCR84-R1-18) EV 先于 cd 解析：cd 后基于相对 $0 的二次解析在 ./seed.sh 相对
# 调用时会把 EV 静默变成仓库根，证据文件全部落错位置。
EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script
cd "$EV/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
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

reg() { # reg <email> -> "<body>\n<http_code>"
  jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}' \
    | curl -s -w "\n%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
        -H 'Content-Type: application/json' --data @-
}

# (r5 observed) THIS backend answers a duplicate registration with
# HTTP 400 + {"message":"user with this email already exists"} (the r4
# rounds' seeds recorded 409 on their integration branch) — BOTH shapes
# are the documented rerun tolerance; a 400 WITHOUT the marker is a real
# parameter failure and fails.
reg_expect() { # reg_expect <label> <email>
  local label="$1" out code body
  out=$(reg "$2") || true   # keep the 000 diagnosis reachable
  code=$(tail -n1 <<<"$out"); body=$(sed '$d' <<<"$out")
  case "$code" in
    2*) say "$label: HTTP $code (registered)" ;;
    409) say "$label: HTTP 409 (already registered — rerun tolerance, the login below reuses this account)" ;;
    400) if [[ "$body" == *"already exists"* ]]; then
        say "$label: HTTP 400 already-exists (rerun tolerance, the login below reuses this account)"
      else
        say "FAIL: $label register answered HTTP 400: ${body:0:160}"; exit 1
      fi ;;
    *) say "FAIL: $label register answered HTTP $code (body: ${body:0:120})"; exit 1 ;;
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
# (OCR84-R1-17 / safety constraint) 外部输入一律参数绑定：sqlite3 CLI 对语句值
# 没有可用的参数绑定，写入/计数改经 python3 sqlite3 的 ? 占位符——uuid_shape
# 白名单保留为纵深防御/快速失败前置，不再是唯一注入防线。
GRANT_N=$(python3 - "$DB" "$UID_B" <<'PY'
import sqlite3, sys
db, uid = sys.argv[1], sys.argv[2]
conn = sqlite3.connect(db)
with conn:
    conn.execute(
        "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version)"
        " values (0, ?, 'plan_publish', 'flow-verifier-r5', 1)",
        (uid,),
    )
print(conn.execute(
    "select count(*) from commercial_grants where capability='plan_publish' and user_id = ?",
    (uid,),
).fetchone()[0])
PY
) || { say "FAIL: grant write/read failed (python3 sqlite3, db=$DB)"; exit 1; }
say "granted: $GRANT_N row(s)"

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
