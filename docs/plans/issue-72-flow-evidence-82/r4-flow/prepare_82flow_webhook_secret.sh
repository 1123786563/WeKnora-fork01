#!/usr/bin/env bash
# Issue #82 R-4 re-verification: mint a REAL Stripe-issued webhook secret and
# store it on the 82flow stack's weknora-stripe provider through the Lago
# model layer (same shape as settle-evidence/prepare_t9_env.sh, which targets
# the t11 lab stack — this variant targets the weknora-lago-82flow project
# whose lab .env no longer exists; the org api key comes from the live DB).
# Requires STRIPE_SECRET_KEY already exported in the shell (source-only).
set -euo pipefail

PROJECT="weknora-lago-82flow"
COMPOSE_FILE="$(pwd)/deploy/lago/compose.yaml"

[ -n "${STRIPE_SECRET_KEY:-}" ] || { echo "STRIPE_SECRET_KEY not exported" >&2; exit 1; }

# Stored secret is authoritative when present.
STORED=$(docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
  "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s" 2>/dev/null | sed -n 's/.*__R4SECRET__//p')
if [ -n "$STORED" ]; then
  echo "provider already holds a stored webhook secret (len ${#STORED})"
  exit 0
fi

# Mint through the same Stripe API RegisterWebhookService uses; prune stale
# endpoints minted by THIS script first (Stripe caps test endpoints at 16).
# (OCR84-R1-31) prune 条件收窄到本轮自己的标记：原条件「description 含 weknora
# 即删」会波及共用同一 Stripe 测试账号的 t11 实验（phases.py 铸造的
# "weknora t11 local settle probe…"）与 t9（"weknora t9 integration secret-mint
# stand-in…"）端点——破坏它们记录在 state 里的 endpoint_id 清理链与 harness
# 投递。本脚本自己铸造的端点带唯一标记：description 含 'weknora r4' 或 URL 以
# /webhooks/stripe/r4 结尾。
SECRET=$(python3 - <<'PY'
import base64, json, os, sys, urllib.parse, urllib.request
key = os.environ["STRIPE_SECRET_KEY"]
auth = "Basic " + base64.b64encode((key + ":").encode()).decode()
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
with opener.open(urllib.request.Request("https://api.stripe.com/v1/webhook_endpoints?limit=100",
                                        headers={"Authorization": auth}), timeout=30) as r:
    rows = json.loads(r.read().decode()).get("data", [])
def is_ours(row):
    desc = str(row.get("description", ""))
    url = str(row.get("url", ""))
    return "weknora r4" in desc or url.endswith("/webhooks/stripe/r4")
pruned = 0
for row in rows:
    if is_ours(row):
        opener.open(urllib.request.Request("https://api.stripe.com/v1/webhook_endpoints/" + row["id"],
                                           headers={"Authorization": auth}, method="DELETE"),
                    timeout=30).read()
        pruned += 1
print(f"pruned {pruned} stale endpoint(s)", file=sys.stderr)
form = urllib.parse.urlencode({
    "url": "https://lago-r4-local.invalid/webhooks/stripe/r4",
    "enabled_events[]": "payment_intent.succeeded",
    "description": "weknora r4 flow secret-mint stand-in (harness-delivered)",
}).encode()
req = urllib.request.Request("https://api.stripe.com/v1/webhook_endpoints", data=form,
    headers={"Authorization": auth, "Content-Type": "application/x-www-form-urlencoded"}, method="POST")
with opener.open(req, timeout=30) as r:
    body = json.loads(r.read().decode())
print(body["secret"])
print("minted endpoint " + body["id"], file=sys.stderr)
PY
)
[ -n "$SECRET" ] || { echo "secret mint failed" >&2; exit 1; }
docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T -e WSECRET="$SECRET" api bin/rails runner \
  "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: ENV['WSECRET'])" >/dev/null
echo "minted + stored a new webhook secret (len ${#SECRET})"
