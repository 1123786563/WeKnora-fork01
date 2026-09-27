#!/usr/bin/env bash
# Issue #83 复验轮（v2）WeKnora 后端启动 —— :8095，worktree 集成分支代码，
# sqlite 独立新库 data/issue83-flow-v2.db，Lago 平台 = 运行中的 82flow 栈(:48889)。
# 凭据纪律：密钥只经运行时 0600 env 文件（仓库外）注入，本脚本零密钥字面量。
set -euo pipefail
cd "$(dirname "$0")/../../.."   # worktree root
# NOTE: this script may run detached under macOS bash 3.2 (non-interactive,
# no zsh profile) — pin the tool PATH so go/docker-dependent callers work.
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:$HOME/.orbstack/bin:$PATH"
KEYS="${FLOW83_V2_KEYS:?set FLOW83_V2_KEYS}"
# Secret material is env-injected ONLY: the operator pre-built a 0600 runtime
# file outside the repo that exports WEKNORA_COMMERCIAL_PLATFORM_API_KEY (a
# read-only `docker exec ... psql -tAc "select value from api_keys limit 1"`)
# and WEKNORA_COMMERCIAL_STRIPE_API_KEY (from ~/.zcode/issue72-stripe.env).
# shellcheck disable=SC1090
source "${FLOW83_V2_SECRETS_ENV:?set FLOW83_V2_SECRETS_ENV to the runtime secrets env file}"
[ -n "${WEKNORA_COMMERCIAL_PLATFORM_API_KEY:-}" ] || { echo 'no lago api key'; exit 1; }
[ -n "${WEKNORA_COMMERCIAL_STRIPE_API_KEY:-}" ] || { echo 'no stripe key'; exit 1; }

export SERVER_PORT=8095 SERVER_HOST=127.0.0.1
export DB_DRIVER=sqlite DB_PATH=data/issue83-flow-v2.db
export WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago
export WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48889
export WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required
export WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN=pm_card_visa
export WEKNORA_WECHAT_APP_ID=wx83v2flow
export WEKNORA_WECHAT_MCH_ID=1900000830
export WEKNORA_WECHAT_MCH_SERIAL=MCH-SERIAL-83-V2
export WEKNORA_WECHAT_MCH_KEY_PATH="$KEYS/mch_private.pem"
export WEKNORA_WECHAT_APIV3_KEY_PATH="$KEYS/apiv3.key"
export WEKNORA_WECHAT_PLATFORM_CERTS="PLAT-SERIAL-83-V2:$KEYS/platform_pub.pem"
export WEKNORA_WECHAT_API_BASE_URL=http://127.0.0.1:8296
export WEKNORA_WECHAT_NOTIFY_URL=http://127.0.0.1:8095/api/v1/commercial/callbacks/wechat
export WEKNORA_ALIPAY_APP_ID=alipay-app-83v2
export WEKNORA_ALIPAY_SELLER_ID=2088000000000083
export WEKNORA_ALIPAY_PUBLIC_KEY_PATH="$KEYS/alipay_verify_local_pub.pem"
export WEKNORA_ALIPAY_MERCHANT_KEY_PATH="$KEYS/alipay_merchant_local.pem"
export WEKNORA_ALIPAY_GATEWAY_URL=http://127.0.0.1:8297
export WEKNORA_ALIPAY_NOTIFY_URL=http://127.0.0.1:8095/api/v1/commercial/callbacks/alipay
export SSRF_WHITELIST_EXTRA=127.0.0.1
export WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true
exec go run ./cmd/server
