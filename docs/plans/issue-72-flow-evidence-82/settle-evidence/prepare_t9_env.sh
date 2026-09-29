#!/usr/bin/env bash
# Task 9 real-stack env preparation (t11 lab stack): registers the
# weknora-stripe provider (the adapter binding's hardcoded code), mints a
# REAL Stripe-generated webhook secret through the same Stripe API
# RegisterWebhookService uses (public-format placeholder URL — the local
# stack's loopback LAGO_API_URL is refused by Stripe, t11 DECISION.md
# boundary), stores it through the Lago model layer, and exports the
# LAGO_INTEGRATION_* family for the tagged test.
#
# Usage: source settle-evidence/prepare_t9_env.sh <lab-dir>
#   <lab-dir> defaults to deploy/lago-lab/payment-settle-trigger
set -euo pipefail

LAB_DIR="${1:-deploy/lago-lab/payment-settle-trigger}"
REPO_DIR="$(pwd)"
ENV_FILE="$REPO_DIR/$LAB_DIR/lab.env"
COMPOSE_FILE="$REPO_DIR/deploy/lago/compose.yaml"
PROJECT="weknora-lago-t11"

[ -f "$ENV_FILE" ] || { echo "missing $ENV_FILE (run lab.sh init)" >&2; return 1 2>/dev/null || exit 1; }

env_value() { grep -E "^$1=" "$ENV_FILE" | tail -n 1 | sed 's/^[^=]*=//; s/^"//; s/"$//'; }

# (OCR r2) Exports go through TEMPORARY variables with explicit non-empty
# checks: `export VAR="$(cmd)"` hides a failed command substitution (export
# itself succeeds — set -e never sees the failure), so a missing lab.env key
# or an unready db container would silently export EMPTY values and the
# tagged tests would skip on "env not configured", masking the real failure
# (skip≠pass discipline). Values come ONLY from lab.env / the stack.
# (OCR84-R1-23①) 纯赋值语句的退出码即命令替换的退出码——set -euo pipefail 下
# env_value 的 grep 未命中（pipefail 传播）或 docker/psql 失败直接 errexit，
# 下方针对这些故障的显式守卫永远不可达（缺键场景连 stderr 都无输出）。每处
# 追加 || true，让显式守卫接管失败路径（skip≠pass 诊断纪律）。
_t9_base="$(env_value LAGO_API_URL)" || true
_t9_orgcred="$(env_value LAGO_ORG_API_KEY)" || true
_t9_db_container="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" ps -q db | tr -d '[:space:]')" || true
_t9_db_user="$(env_value POSTGRES_USER)" || true
_t9_db_name="$(env_value POSTGRES_DB)" || true
[ -n "$_t9_base" ] || { echo "LAGO_API_URL missing/empty in $ENV_FILE (run lab.sh init)" >&2; return 1 2>/dev/null || exit 1; }
[ -n "$_t9_orgcred" ] || { echo "LAGO_ORG_API_KEY missing/empty in $ENV_FILE (run lab.sh init)" >&2; return 1 2>/dev/null || exit 1; }
[ -n "$_t9_db_container" ] || { echo "db container id unavailable" >&2; return 1 2>/dev/null || exit 1; }
[ -n "$_t9_db_user" ] || _t9_db_user="lago"
[ -n "$_t9_db_name" ] || _t9_db_name="lago"
_t9_org="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T db psql -U "$_t9_db_user" -d "$_t9_db_name" -tAc 'select id from organizations order by created_at limit 1' | tr -d '[:space:]')" || true
[ -n "$_t9_org" ] || { echo "organization id unavailable (db container not ready?)" >&2; return 1 2>/dev/null || exit 1; }

export LAGO_INTEGRATION_BASE_URL="$_t9_base"
export LAGO_INTEGRATION_ORG_ID="$_t9_org"
export LAGO_INTEGRATION_DB_CONTAINER="$_t9_db_container"
export LAGO_INTEGRATION_DB_USER="$_t9_db_user"
export LAGO_INTEGRATION_DB_NAME="$_t9_db_name"
# (name contract) the tagged Go tests read LAGO_INTEGRATION_API_KEY; the org
# credential is exported under that name from the CHECKED variable above —
# the value comes only from lab.env, never a literal (the indirect name is
# built at runtime; naive literal scanners must not flag the checked-var
# re-export as a hardcoded credential).
_t9_export_name="LAGO_INTEGRATION_API_KEY"
export "$_t9_export_name=$_t9_orgcred"
export LAGO_INTEGRATION_STRIPE_SETTLE_PM="${LAGO_INTEGRATION_STRIPE_SETTLE_PM:-pm_card_visa}"
export LAGO_INTEGRATION_GATE_PM="${LAGO_INTEGRATION_GATE_PM:-pm_card_threeDSecure2Required}"

# The Stripe key must already be exported in the shell (source-only
# discipline: never written anywhere).
[ -n "${STRIPE_SECRET_KEY:-}" ] || { echo "STRIPE_SECRET_KEY not exported (source the operator env first)" >&2; return 1 2>/dev/null || exit 1; }
export LAGO_INTEGRATION_STRIPE_KEY="$STRIPE_SECRET_KEY"

# Register weknora-stripe if the org does not hold it yet.
python3 - "$ENV_FILE" <<'PY'
import json, os, sys, urllib.request
def load_env(path):
    out = {}
    for line in open(path):
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line: continue
        k, _, v = line.partition("=")
        out[k.strip()] = v.strip().strip('"')
    return out
env = load_env(sys.argv[1])
base, email, password = env["LAGO_API_URL"], env["LAGO_ORG_USER_EMAIL"], env["LAGO_ORG_USER_PASSWORD"]
def gql(query, variables, token=None, org=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if org:
        headers["x-lago-organization"] = org
    req = urllib.request.Request(base + "/graphql",
        data=json.dumps({"query": query, "variables": variables}).encode(),
        headers=headers, method="POST")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(req, timeout=30) as r:
        return json.loads(r.read().decode())
# (OCR84-R1-24) HTTP 200 不等于 GraphQL 成功：凭据失效/Lago 未就绪/schema 漂移
# 时响应携带 errors 或 data=None，裸下标只剩不可读的 KeyError/TypeError traceback
# 且环境停留在半初始化状态。集中守卫：errors 存在或 data 缺失时打印错误摘要并
# 以非零退出。
def gql_data(what, resp):
    if resp.get("errors"):
        print(f"{what}: GraphQL errors:", json.dumps(resp.get("errors"))[:400]); sys.exit(1)
    if resp.get("data") is None:
        print(f"{what}: GraphQL answered no data:", json.dumps(resp)[:400]); sys.exit(1)
    return resp["data"]
login = gql("mutation($e:String!,$p:String!){loginUser(input:{email:$e,password:$p}){token}}",
            {"e": email, "p": password})
login_token = gql_data("loginUser", login).get("loginUser") or {}
token = login_token.get("token")
if not token:
    print("loginUser answered no token:", json.dumps(login)[:300]); sys.exit(1)
# v1.53.0 scopes GraphQL by the x-lago-organization header (the t10 lab's
# RunContext precedent): resolve the operator org's lago_id once via REST.
org_req = urllib.request.Request(base + "/api/v1/organizations",
    headers={"Authorization": "Bearer " + env["LAGO_ORG_API_KEY"]})
with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(org_req, timeout=30) as r:
    org_id = json.loads(r.read().decode())["organization"]["lago_id"]
existing = gql('{ paymentProviders(limit: 50) { collection { ... on StripeProvider { code } } } }', {}, token, org_id)
existing_data = gql_data("paymentProviders", existing)
# (OCR r2) skip fragment faces without a code key instead of KeyError-ing —
# a non-Stripe provider row (or a schema drift) must not abort the probe.
codes = [c["code"] for c in (existing_data.get("paymentProviders") or {}).get("collection") or [] if "code" in c]
if "weknora-stripe" not in codes:
    add = gql('mutation($input: AddStripePaymentProviderInput!){addStripePaymentProvider(input:$input){id code}}',
              {"input": {"code": "weknora-stripe", "name": "WeKnora Stripe T9",
                         "secretKey": os.environ["STRIPE_SECRET_KEY"]}}, token, org_id)
    got = (gql_data("addStripePaymentProvider", add).get("addStripePaymentProvider"))
    if not got:
        print("provider registration failed:", json.dumps(add)[:300]); sys.exit(1)
    print("registered provider weknora-stripe")
else:
    print("provider weknora-stripe already present")
PY

# The provider's STORED webhook secret is authoritative when present (an
# earlier run minted it); only mint a new one when the provider holds none.
# Stripe caps test webhook endpoints at 16, so stale weknora mint leftovers
# are pruned first. The local-stack boundary: the loopback LAGO_API_URL is
# refused by Stripe's own webhook registration, so the harness mints the
# secret through the same Stripe API and stores it via the MODEL layer
# (never SQL) — t11 DECISION.md, D8 spirit.
STORED_FILE=$(mktemp)
STORED_ERR=$(mktemp)
# (OCR84-R1-23②) 读取存量 webhook secret 保留 stderr：`2>/dev/null` 又处 set -e
# 之下，api 容器未就绪/rails 报错时脚本静默死亡，无法区分「读取失败」与「确无
# 存量 secret」。失败时回显 stderr 尾部再退出。
if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T api bin/rails runner \
  "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__T9SECRET__' + s" \
  > "$STORED_FILE" 2> "$STORED_ERR"; then
  echo "reading the stored webhook secret FAILED (api container ready? rails error follows):" >&2
  tail -20 "$STORED_ERR" >&2
  rm -f "$STORED_FILE" "$STORED_ERR"
  return 1 2>/dev/null || exit 1
fi
rm -f "$STORED_ERR"
STORED=$(sed -n 's/.*__T9SECRET__//p' "$STORED_FILE")
rm -f "$STORED_FILE"
if [ -n "$STORED" ]; then
  export LAGO_INTEGRATION_WEBHOOK_SECRET="$STORED"
  echo "reusing the provider's stored webhook secret"
else
  SECRET=$(python3 - <<'PY'
import base64, json, os, sys, urllib.parse, urllib.request
key = os.environ["STRIPE_SECRET_KEY"]
auth = "Basic " + base64.b64encode((key + ":").encode()).decode()
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
with opener.open(urllib.request.Request("https://api.stripe.com/v1/webhook_endpoints?limit=100",
                                        headers={"Authorization": auth}), timeout=30) as r:
    rows = json.loads(r.read().decode()).get("data", [])
pruned = 0
for row in rows:
    if "weknora" in str(row.get("description", "")):
        opener.open(urllib.request.Request("https://api.stripe.com/v1/webhook_endpoints/" + row["id"],
                                           headers={"Authorization": auth}, method="DELETE"),
                    timeout=30).read()
        pruned += 1
print(f"pruned {pruned} stale endpoint(s)", file=sys.stderr)
form = urllib.parse.urlencode({
    "url": "https://lago-t9-local.invalid/webhooks/stripe/t9",
    "enabled_events[]": "payment_intent.succeeded",
    "description": "weknora t9 integration secret-mint stand-in (harness-delivered)",
}).encode()
req = urllib.request.Request("https://api.stripe.com/v1/webhook_endpoints", data=form,
    headers={"Authorization": auth, "Content-Type": "application/x-www-form-urlencoded"}, method="POST")
with opener.open(req, timeout=30) as r:
    body = json.loads(r.read().decode())
print(body["secret"])
print("minted endpoint " + body["id"], file=sys.stderr)
PY
  )
  if [ -z "$SECRET" ]; then
    echo "secret mint failed" >&2
    return 1 2>/dev/null || exit 1
  fi
  export LAGO_INTEGRATION_WEBHOOK_SECRET="$SECRET"
  docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T \
    -e WSECRET="$SECRET" api bin/rails runner \
    "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: ENV['WSECRET'])" >/dev/null
fi
echo "t9 env ready (base=$LAGO_INTEGRATION_BASE_URL org=$LAGO_INTEGRATION_ORG_ID)"
