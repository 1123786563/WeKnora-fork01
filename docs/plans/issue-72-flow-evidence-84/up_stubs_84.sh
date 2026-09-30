#!/usr/bin/env bash
# Issue #84 渠道 stub 启动：微信 anomaly 副本 :8298 + 支付宝 stub :8299（82 复用）。
# 身份全部经 env 注入（WECHAT_STUB_* / FLOW82_*），源码与脚本零密钥字面量。
# 端口纪律：避开既往占用口径（:8291-8297 已被 82/83 轮使用）。
set -euo pipefail
EV="$(cd "$(dirname "$0")" && pwd)"
KEYS="${FLOW84_KEYS:?set FLOW84_KEYS}"
MCH_ID=1900000084
APP_ID=wx84flow
SERIAL=PLAT-SERIAL-84
NOTIFY=http://127.0.0.1:8096/api/v1/commercial/callbacks/wechat

pkill -f 'wechat_native_stub_anomaly.py' 2>/dev/null || true
pkill -f 'alipay_gateway_stub.py' 2>/dev/null || true
sleep 1

WECHAT_STUB_PORT=8298 \
WECHAT_STUB_APP_ID=$APP_ID \
WECHAT_STUB_MCH_ID=$MCH_ID \
WECHAT_STUB_PLATFORM_SERIAL=$SERIAL \
WECHAT_STUB_NOTIFY_URL=$NOTIFY \
WECHAT_STUB_KEY_DIR="$KEYS" \
  nohup python3 "$EV/wechat_native_stub_anomaly.py" >> "${TMPDIR}issue84-wechat-stub.log" 2>&1 &
echo "wechat anomaly stub pid $!"

FLOW82_ALIPAY_PORT=8299 \
FLOW82_KEY_DIR="$KEYS" \
  nohup python3 "$EV/../issue-72-flow-evidence-82/alipay_gateway_stub.py" >> "${TMPDIR}issue84-alipay-stub.log" 2>&1 &
echo "alipay stub pid $!"

sleep 2
echo '--- smoke: wechat anomaly stub rejects unsigned native create (expect 401) ---'
curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8298/v3/pay/transactions/native -d '{}'
echo '--- smoke: alipay stub answers precreate-unauth ---'
curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8299/gateway.do -d 'service=alipay.trade.precreate'
