#!/usr/bin/env bash
# Issue #83 复验轮（v2）一次性本地密钥生成 —— 运行时目录，不进仓库，无真实凭据价值。
# 产出（$1 = 目标目录，默认 ${TMPDIR}issue83-v2-keys）：
#   微信： mch_private.pem(PKCS#1) mch_public.pem(PKIX) platform_priv.pem platform_pub.pem apiv3.key(raw 32B)
#   支付宝： alipay_verify_local.pem(PKCS#1) alipay_verify_local_pub.pem(PKIX) alipay_merchant_local.pem(PKCS#8)
# 注意：本地 lab 惯例（82 轮同源）——支付宝 stub 用 SHARED_PUB 验 WeKnora 出站签名，
# 因此商户私钥与 alipay_verify_local 同钥（PKCS#8 导出），一对密钥两用。
set -euo pipefail
KEYS="${1:-${TMPDIR}issue83-v2-keys}"
mkdir -p "$KEYS"
rm -f "$KEYS"/*.pem "$KEYS"/apiv3.key

# --- 微信商户 RSA ---
openssl genrsa -out "$KEYS/mch_private.pem" 2048 2>/dev/null
openssl rsa -in "$KEYS/mch_private.pem" -pubout -out "$KEYS/mch_public.pem" 2>/dev/null
# --- 微信平台 RSA（stub 回调签名侧）---
openssl genrsa -out "$KEYS/platform_priv.pem" 2048 2>/dev/null
openssl rsa -in "$KEYS/platform_priv.pem" -pubout -out "$KEYS/platform_pub.pem" 2>/dev/null
# --- APIv3 对称密钥：raw 32 字节 ---
head -c 32 /dev/urandom > "$KEYS/apiv3.key"
# --- 支付宝本地对 ---
openssl genrsa -out "$KEYS/alipay_verify_local.pem" 2048 2>/dev/null
openssl rsa -in "$KEYS/alipay_verify_local.pem" -pubout -out "$KEYS/alipay_verify_local_pub.pem" 2>/dev/null
openssl pkcs8 -topk8 -nocrypt -in "$KEYS/alipay_verify_local.pem" -out "$KEYS/alipay_merchant_local.pem" 2>/dev/null

echo "keys ready in $KEYS:"
ls -la "$KEYS"
[ "$(wc -c < "$KEYS/apiv3.key")" -eq 32 ] && echo "apiv3.key = 32 bytes OK"
