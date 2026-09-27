#!/usr/bin/env bash
# r5b replay face 2: deliver the SAME signed webhook again (fresh event id,
# same succeeded PI body) and re-read the four authority objects — every
# projection must be byte-identical (no second payment, no wallet change).
# Caller env (both REQUIRED, never hardcoded):
#   STRIPE_SECRET_KEY          — source ~/.zcode/issue72-stripe.env (set -a)
#   LAGO_INTEGRATION_WEBHOOK_SECRET — minted/stored via the R-11 channel
set -euo pipefail
cd "$(dirname "$0")/../../../.."
EV="$(cd "$(dirname "$0")" && pwd)"
: "${STRIPE_SECRET_KEY:?source the operator env}"
: "${LAGO_INTEGRATION_WEBHOOK_SECRET:?export the stored provider secret}"
LAGO_KEY=$(docker exec weknora-lago-82r5-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')

{
  echo "== replay face 2: duplicate webhook ($(date '+%H:%M:%S')) =="
  python3 docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py \
    --customer cus_VL2rl1L1V8yBYd --base http://127.0.0.1:48889 \
    --org c5a2dffd-ad75-415a-998a-22cf880148d1 --code weknora-stripe 2>&1 || \
    echo "(delivery leg answered non-2xx — recorded verbatim above)"
  sleep 3
  echo "--- four objects re-read after duplicate webhook ---"
  curl -s "http://127.0.0.1:48889/api/v1/subscriptions?external_id=weknora-tenant-7-purchase" \
    -H "Authorization: Bearer $LAGO_KEY" \
    | jq -c '{status: .subscriptions[0].status, plan: .subscriptions[0].plan_code}'
  curl -s "http://127.0.0.1:48889/api/v1/invoices/8e813325-93d2-4058-9232-327a1c34aab8" \
    -H "Authorization: Bearer $LAGO_KEY" \
    | jq -c '{status: .invoice.status, payment_status: .invoice.payment_status, number: .invoice.number, total: .invoice.total_amount_cents}'
  docker exec weknora-lago-82r5-db-1 psql -U lago -tAc \
    "select count(*) filter (where status='succeeded') as succeeded, count(*) as total from payments where payable_id='8e813325-93d2-4058-9232-327a1c34aab8';"
  docker exec weknora-lago-82r5-db-1 psql -U lago -tAc \
    "select name, balance_cents from wallets where customer_id=(select id from customers where external_id='weknora-tenant-7') order by name;"
  echo "--- local fulfillment count ---"
  sqlite3 data/issue82-r5b-verify.db "select count(*) from commercial_fulfillment_records;"
} | tee "$EV/replay-02-webhook.txt"
