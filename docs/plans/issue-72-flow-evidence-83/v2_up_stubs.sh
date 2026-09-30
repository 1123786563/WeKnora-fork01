#!/usr/bin/env bash
# Issue #83 复验轮（v2）渠道 stub 启动：微信 Native stub :8296 + 支付宝 stub :8297。
# 日志固定到 api_recovery_83.mjs 读取的路径（${TMPDIR}issue83-{wechat,alipay}-stub.log）。
# 全部身份经 env 注入（WECHAT_STUB_* / FLOW82_*），源码与脚本零密钥字面量。
set -euo pipefail
EV="$(cd "$(dirname "$0")" && pwd)"
KEYS="${FLOW83_V2_KEYS:?set FLOW83_V2_KEYS}"
MCH_ID=1900000830
APP_ID=wx83v2flow
SERIAL=PLAT-SERIAL-83-V2
NOTIFY=http://127.0.0.1:8095/api/v1/commercial/callbacks/wechat

pkill -f 'wechat_native_stub.py' 2>/dev/null || true
pkill -f 'alipay_gateway_stub.py' 2>/dev/null || true
sleep 1

WECHAT_STUB_PORT=8296 \
WECHAT_STUB_APP_ID=$APP_ID \
WECHAT_STUB_MCH_ID=$MCH_ID \
WECHAT_STUB_PLATFORM_SERIAL=$SERIAL \
WECHAT_STUB_NOTIFY_URL=$NOTIFY \
WECHAT_STUB_KEY_DIR="$KEYS" \
  nohup python3 "$EV/wechat_native_stub.py" >> "${TMPDIR}issue83-wechat-stub.log" 2>&1 &
echo "wechat stub pid $!"

FLOW82_ALIPAY_PORT=8297 \
FLOW82_KEY_DIR="$KEYS" \
  nohup python3 "$EV/../issue-72-flow-evidence-82/alipay_gateway_stub.py" >> "${TMPDIR}issue83-alipay-stub.log" 2>&1 &
echo "alipay stub pid $!"

sleep 2
echo '--- smoke: wechat stub rejects unsigned native create (expect 401) ---'
curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8296/v3/pay/transactions/native -d '{}'
echo '--- smoke: alipay stub answers precreate-unauth ---'
curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8297/gateway.do -d 'service=alipay.trade.precreate'
