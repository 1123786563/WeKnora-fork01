#!/usr/bin/env bash
# Issue #84 WeKnora 后端启动 —— :8096，worktree 分支代码，sqlite 独立新库
# data/issue84-flow.db，Lago 平台 = 运行中的 82r5 栈(:48889)。
# 凭据纪律：密钥只经运行时 0600 env 文件（仓库外）注入，本脚本零密钥字面量。
set -euo pipefail
cd "$(dirname "$0")/../../.."   # worktree root
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:$HOME/.orbstack/bin:$PATH"
KEYS="${FLOW84_KEYS:?set FLOW84_KEYS}"
# 密钥材料只经 env 注入：操作者预先在仓库外备好 0600 运行时文件，导出
# WEKNORA_COMMERCIAL_PLATFORM_API_KEY（只读 docker exec psql 从 82r5 栈读出）
# 与 WEKNORA_COMMERCIAL_STRIPE_API_KEY（~/.zcode/issue72-stripe.env）。
# shellcheck disable=SC1090
source "${FLOW84_SECRETS_ENV:?set FLOW84_SECRETS_ENV to the runtime secrets env file}"
[ -n "${WEKNORA_COMMERCIAL_PLATFORM_API_KEY:-}" ] || { echo 'no lago api key'; exit 1; }
[ -n "${WEKNORA_COMMERCIAL_STRIPE_API_KEY:-}" ] || { echo 'no stripe key'; exit 1; }
echo "env self-check: lago key length=${#WEKNORA_COMMERCIAL_PLATFORM_API_KEY} stripe key length=${#WEKNORA_COMMERCIAL_STRIPE_API_KEY}"

export SERVER_PORT=8096 SERVER_HOST=127.0.0.1
export DB_DRIVER=sqlite DB_PATH=data/issue84-flow.db
export WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago
export WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48889
export WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required
export WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN=pm_card_visa
export WEKNORA_WECHAT_APP_ID=wx84flow
export WEKNORA_WECHAT_MCH_ID=1900000084
export WEKNORA_WECHAT_MCH_SERIAL=MCH-SERIAL-84
export WEKNORA_WECHAT_MCH_KEY_PATH="$KEYS/mch_private.pem"
export WEKNORA_WECHAT_APIV3_KEY_PATH="$KEYS/apiv3.key"
export WEKNORA_WECHAT_PLATFORM_CERTS="PLAT-SERIAL-84:$KEYS/platform_pub.pem"
export WEKNORA_WECHAT_API_BASE_URL=http://127.0.0.1:8298
export WEKNORA_WECHAT_NOTIFY_URL=http://127.0.0.1:8096/api/v1/commercial/callbacks/wechat
export WEKNORA_ALIPAY_APP_ID=alipay-app-84
export WEKNORA_ALIPAY_SELLER_ID=2088000000000084
export WEKNORA_ALIPAY_PUBLIC_KEY_PATH="$KEYS/alipay_verify_local_pub.pem"
export WEKNORA_ALIPAY_MERCHANT_KEY_PATH="$KEYS/alipay_merchant_local.pem"
export WEKNORA_ALIPAY_GATEWAY_URL=http://127.0.0.1:8299
export WEKNORA_ALIPAY_NOTIFY_URL=http://127.0.0.1:8096/api/v1/commercial/callbacks/alipay
# 环回 stub 豁免（既有惯例：SSRF 策略默认拒绝环回出站）
export SSRF_WHITELIST_EXTRA=127.0.0.1
export WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true
exec go run ./cmd/server
