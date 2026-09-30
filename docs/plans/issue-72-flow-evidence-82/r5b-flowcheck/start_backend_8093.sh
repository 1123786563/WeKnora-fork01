#!/usr/bin/env bash
# r5b-flowcheck backend launcher (Issue #82 flow re-verification, 2026-09-28).
# Starts the integration-branch backend from THIS worktree on :8093 with a
# FRESH sqlite db (data/issue82-r5b-verify.db) against the running
# weknora-lago-82r5 Lago stack (:48889) and the local alipay stub (:8294).
#
# ALL credentials are env-injected by the caller (this script only validates
# presence and passes the environment through — no credential values are
# assigned anywhere in this file):
#   WEKNORA_COMMERCIAL_PLATFORM_API_KEY — Lago org key (read from the stack db)
#   WEKNORA_COMMERCIAL_STRIPE_API_KEY   — real Stripe TEST key (sourced from
#     ~/.zcode/issue72-stripe.env by the caller)
# Log redirection is up to the caller. Run from anywhere (paths are resolved).
set -euo pipefail
cd "$(dirname "$0")/../../../.."   # worktree root
: "${WEKNORA_COMMERCIAL_PLATFORM_API_KEY:?caller must export the Lago org key}"
: "${WEKNORA_COMMERCIAL_STRIPE_API_KEY:?caller must source ~/.zcode/issue72-stripe.env}"
KEYS="${FLOW82_KEY_DIR:?caller must export FLOW82_KEY_DIR=/tmp/issue82-r5-keys (the local RSA key pair)}"

export DB_DRIVER=sqlite
export DB_PATH=data/issue82-r5b-verify.db
export WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago
export WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48889
export WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required
export WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM_TOKEN=pm_card_visa
export WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true
export WEKNORA_ALIPAY_APP_ID=2026000000000000
export WEKNORA_ALIPAY_SELLER_ID=2088000000000000
export WEKNORA_ALIPAY_PUBLIC_KEY_PATH="$KEYS/alipay_verify_local_pub.pem"
export WEKNORA_ALIPAY_MERCHANT_KEY_PATH="$KEYS/alipay_merchant_local.pem"
export WEKNORA_ALIPAY_GATEWAY_URL=http://127.0.0.1:8294
export SSRF_WHITELIST_EXTRA=127.0.0.1

exec go run ./cmd/server
