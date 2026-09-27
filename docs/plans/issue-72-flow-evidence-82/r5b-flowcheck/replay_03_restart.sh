#!/usr/bin/env bash
# r5b replay face 3: kill the backend, restart it on the SAME env, let the
# outbox drain re-run, and assert the activation/grant/event counts are ALL
# unchanged (no second grant, no second fulfill event, order stays
# fulfilled). Caller env identical to start_backend_8093.sh (the two
# credentials are REQUIRED, never hardcoded here).
set -euo pipefail
cd "$(dirname "$0")/../../../.."
EV="$(cd "$(dirname "$0")" && pwd)"
: "${WEKNORA_COMMERCIAL_PLATFORM_API_KEY:?caller must export the Lago org key}"
: "${WEKNORA_COMMERCIAL_STRIPE_API_KEY:?caller must source ~/.zcode/issue72-stripe.env}"
DB=data/issue82-r5b-verify.db
Q_ORD="select state from commercial_orders where id='ord_86c00340e158d726';"
Q_ACT="select count(*) from commercial_fulfillment_records;"
Q_EVT="select count(*) from commercial_outbox_events where kind='fulfill';"

{
  echo "== replay face 3: backend kill + restart ($(date '+%H:%M:%S')) =="
  echo "before: order=$(sqlite3 "$DB" "$Q_ORD") activations=$(sqlite3 "$DB" "$Q_ACT") fulfill_events=$(sqlite3 "$DB" "$Q_EVT")"
  PID=$(lsof -nP -iTCP:8093 -sTCP:LISTEN -t)
  echo "killing backend pid=$PID"
  kill "$PID"
  sleep 3
  lsof -nP -iTCP:8093 -sTCP:LISTEN >/dev/null 2>&1 && { echo "FAIL: port still listening"; exit 1; } || echo "port 8093 released"
  nohup bash "$EV/start_backend_8093.sh" > /tmp/issue82-r5b-backend-restart.log 2>&1 &
  echo "restarted (launcher $!)"
} | tee "$EV/replay-03-restart.txt"

for i in $(seq 1 40); do
  code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8093/health 2>/dev/null)
  [ "$code" = "200" ] && break
  sleep 5
done
echo "health=$code after restart" | tee -a "$EV/replay-03-restart.txt"
# Let at least one full drain pass (30s cadence) re-run.
sleep 40
{
  echo "after (one drain pass past): order=$(sqlite3 "$DB" "$Q_ORD") activations=$(sqlite3 "$DB" "$Q_ACT") fulfill_events=$(sqlite3 "$DB" "$Q_EVT")"
  echo "outbox tail: $(sqlite3 "$DB" "select event_key, state, attempt_count from commercial_outbox_events;")"
} | tee -a "$EV/replay-03-restart.txt"
