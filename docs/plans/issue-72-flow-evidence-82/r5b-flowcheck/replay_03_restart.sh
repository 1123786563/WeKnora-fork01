#!/usr/bin/env bash
# r5b replay face 3: kill the backend, restart it on the SAME env, let the
# outbox drain re-run, and assert the activation/grant/event counts are ALL
# unchanged (no second grant, no second fulfill event, order stays
# fulfilled). Caller env identical to start_backend_8093.sh (the two
# credentials are REQUIRED, never hardcoded here).
# (OCR84-R1-07) ①健康轮询的 curl 赋值补 || code=000 防护——kill 后 go run 冷启动
# 编译期端口必未监听，curl 退出码 7 在 set -e 下作为赋值退出码第一次探测即中止
# 脚本（replay-03-restart.txt 曾静默截断在 restarted 行且无 FAIL 标记，40×5s
# 等待循环根本无法度过停机窗口）；②轮询落空后强制 200 断言——否则继续输出
# after 计数读的是未被触动的旧 DB，产生「重启后无二次发放」的假阳性 PASS。
set -euo pipefail
EV="$(cd "$(dirname "$0")" && pwd)"
cd "$EV/../../../.."
: "${WEKNORA_COMMERCIAL_PLATFORM_API_KEY:?caller must export the Lago org key}"
: "${WEKNORA_COMMERCIAL_STRIPE_API_KEY:?caller must source ~/.zcode/issue72-stripe.env}"
# (OCR84-R1-07③) 与 start_backend_8093.sh 的必需集合对齐：key dir 缺失时快速
# 失败，而不是等到重启后的后端静默起来一半。
: "${FLOW82_KEY_DIR:?caller must export FLOW82_KEY_DIR (same set as start_backend_8093.sh)}"
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

code=000
for i in $(seq 1 40); do
  # (OCR84-R1-07①) 传输层失败（连接拒绝=7、超时=28…）是停机窗口的预期形态：
  # 退化 000 继续轮询，绝不触发 errexit。
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 http://127.0.0.1:8093/health 2>/dev/null) || code=000
  [ "$code" = "200" ] && break
  sleep 5
done
# (OCR84-R1-07②) 轮询落空=重启失败：显式 FAIL 并中止，不再落空后读旧 DB。
if [ "$code" != "200" ]; then
  echo "FAIL: backend did not come back healthy within 200s (last health code=$code); restart log tail:" | tee -a "$EV/replay-03-restart.txt"
  tail -20 /tmp/issue82-r5b-backend-restart.log | tee -a "$EV/replay-03-restart.txt" || true
  exit 1
fi
echo "health=$code after restart" | tee -a "$EV/replay-03-restart.txt"
# Let at least one full drain pass (30s cadence) re-run.
sleep 40
{
  echo "after (one drain pass past): order=$(sqlite3 "$DB" "$Q_ORD") activations=$(sqlite3 "$DB" "$Q_ACT") fulfill_events=$(sqlite3 "$DB" "$Q_EVT")"
  echo "outbox tail: $(sqlite3 "$DB" "select event_key, state, attempt_count from commercial_outbox_events;")"
} | tee -a "$EV/replay-03-restart.txt"
