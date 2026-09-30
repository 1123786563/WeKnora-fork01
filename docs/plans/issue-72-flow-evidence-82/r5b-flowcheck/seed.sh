#!/usr/bin/env bash
# r5b-flowcheck seed (Issue #82 flow re-verification round r5b, 2026-09-28).
# Same hardened shape as ../r5-verify/seed.sh (Task 16 fixed template) with
# ONE adaptation: the Lago arrival assertion. This round reuses the
# weknora-lago-82r5 stack that ALREADY carries weknora-pro-v1 (9900/CNY/
# monthly/pay_in_advance) from the r5 round, and the adapter's createPlan
# treats a 422 already-exists with EQUAL priced content as an idempotent
# replay (lago.go createPlan -> verifyPlanReplay). "This round's publish
# arrived" therefore has TWO acceptable faces here:
#   (1) the 9900fen plan count increased (fresh-stack shape), or
#   (2) the count is unchanged AND GET /plans/weknora-pro-v1 reads back the
#       same priced identity (idempotent-replay shape — the authoritative
#       plan exists and matches what THIS round published).
# Everything else (env-required credentials, REDACTED login captures,
# asserted tenant serials, sqlite UUID-gated grant insert) is unchanged.
set -euo pipefail
# (OCR84-R1-18) EV 先于 cd 解析：cd 后基于相对 $0 的二次解析在 ./seed.sh 相对
# 调用时会把 EV 静默变成仓库根，证据文件全部落错位置。
EV="$(cd "$(dirname "$0")" && pwd)"
cd "$EV/../../../.."   # worktree root
BACKEND=http://127.0.0.1:8093
PW="${FLOW82_R5_PW:?missing required env FLOW82_R5_PW}"
DB="${DB_PATH:?missing required env DB_PATH (this round: data/issue82-r5b-verify.db)}"
[ -f "$DB" ] || { echo "FAIL: DB_PATH=$DB not found (start the backend first)" >&2; exit 1; }
PREFIX="${FLOW82_EMAIL_PREFIX:?missing required env FLOW82_EMAIL_PREFIX (this round: settle-r5b)}"
PLAN_KEY="${PLAN_KEY:-pro}"
PLAN_FEN="${PLAN_FEN:-9900}"

: > "$EV/seed-run.txt"
say() { echo "$@" | tee -a "$EV/seed-run.txt"; }

reg() { # reg <email> -> "<body>\n<http_code>"
  jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}' \
    | curl -s -w "\n%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
        -H 'Content-Type: application/json' --data @-
}
reg_expect() { # reg_expect <label> <email>
  local label="$1" out code body
  out=$(reg "$2") || true
  code=$(tail -n1 <<<"$out"); body=$(sed '$d' <<<"$out")
  case "$code" in
    2*) say "$label: HTTP $code (registered)" ;;
    409) say "$label: HTTP 409 (already registered — rerun tolerance)" ;;
    400) if [[ "$body" == *"already exists"* ]]; then
        say "$label: HTTP 400 already-exists (rerun tolerance)"
      else
        say "FAIL: $label register answered HTTP 400: ${body:0:160}"; exit 1
      fi ;;
    *) say "FAIL: $label register answered HTTP $code (body: ${body:0:120})"; exit 1 ;;
  esac
}

PAD="${FLOW82_PAD_COUNT:-2}"
case "$PAD" in ''|*[!0-9]*) say "FAIL: FLOW82_PAD_COUNT must be a positive integer"; exit 1 ;; esac
[ "$PAD" -ge 1 ] || { say "FAIL: FLOW82_PAD_COUNT must be >= 1"; exit 1; }
say "== register $PAD placeholders =="
for i in $(seq 1 "$PAD"); do
  reg_expect "placeholder-$i" "$PREFIX-pad$i@verify.local"
done

say "== register protagonists =="
reg_expect "a (browser)" "$PREFIX-a@verify.local"
reg_expect "b (publisher)" "$PREFIX-b@verify.local"

login() { # login <email> -> token json
  jq -nc --arg e "$1" --arg p "$PW" '{email:$e,password:$p}' \
    | curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' --data @-
}
LOGIN_A=$(login "$PREFIX-a@verify.local") || true
LOGIN_B=$(login "$PREFIX-b@verify.local") || true
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
  || { say "FAIL: protagonists on unexpected tenants A=$TENANT_A B=$TENANT_B (expected $((PAD+1))/$((PAD+2)))"; exit 1; }

uuid_shape() { [[ "$1" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; }
uuid_shape "$UID_B" || { say "FAIL: user id not a UUID ('$UID_B')"; exit 1; }

say "== grant plan_publish to B at platform scope (seed row) =="
# (OCR84-R1-17 / safety constraint) 外部输入一律参数绑定：写入/计数改经 python3
# sqlite3 的 ? 占位符——uuid_shape 白名单保留为纵深防御/快速失败前置。
GRANT_N=$(python3 - "$DB" "$UID_B" <<'PY'
import sqlite3, sys
db, uid = sys.argv[1], sys.argv[2]
conn = sqlite3.connect(db)
with conn:
    conn.execute(
        "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version)"
        " values (0, ?, 'plan_publish', 'flow-verifier-r5b', 1)",
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
  say "lago ${PLAN_FEN}fen plan count increased ($BASE_N -> $N): THIS round's publish arrived (fresh-stack shape)"
else
  # (r5b adaptation) idempotent-replay shape: equal priced identity read-back
  curl -s "http://127.0.0.1:48889/api/v1/plans/weknora-$PLAN_KEY-v$DRAFT_V" \
    -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-04-lago-plan-readback.json"
  jq -e --argjson fen "$PLAN_FEN" '.plan.amount_cents==$fen and .plan.amount_currency=="CNY" and .plan.interval=="monthly" and .plan.pay_in_advance==true' \
    "$EV/api-04-lago-plan-readback.json" >/dev/null \
    && say "lago plan count unchanged ($BASE_N) but weknora-$PLAN_KEY-v$DRAFT_V reads back the SAME priced identity (idempotent-replay shape — authoritative plan present, this round's publish accepted)" \
    || { say "FAIL: neither arrival shape holds (count $BASE_N unchanged AND read-back mismatch — see api-04-lago-plan-readback.json)"; exit 1; }
fi
say "SEED OK (tenants A=$TENANT_A B=$TENANT_B)"
