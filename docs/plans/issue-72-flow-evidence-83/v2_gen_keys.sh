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
# (C-06) 构造性保证首尾字节非空白：两个消费方都会对文件内容做空白剥离
# （Go 侧 wechat.go 先 bytes.TrimSpace 再强校验 32 字节；Python stub 的
# _load_apiv3 同样 strip() 后校验）——原始随机 32 字节的首或尾字节落在
# ASCII 空白集（0x09-0x0D、0x20，概率约 4.6%）时，两侧都把密钥截成 31 字节
# 并在启动时报错，整个复验轮随机失败。首尾各 1 字节经 tr 把 0x00-0x20 区间
# （空白是其子集）折叠为 'A'（0x41，非空白），中间 30 字节保持全随机——
# 密钥熵损失可忽略（边界各 6/256 的折叠），长度恒 32。
{ head -c 1 /dev/urandom | LC_ALL=C tr '\000-\040' 'A'
  head -c 30 /dev/urandom
  head -c 1 /dev/urandom | LC_ALL=C tr '\000-\040' 'A'; } > "$KEYS/apiv3.key"
# --- 支付宝本地对 ---
openssl genrsa -out "$KEYS/alipay_verify_local.pem" 2048 2>/dev/null
openssl rsa -in "$KEYS/alipay_verify_local.pem" -pubout -out "$KEYS/alipay_verify_local_pub.pem" 2>/dev/null
openssl pkcs8 -topk8 -nocrypt -in "$KEYS/alipay_verify_local.pem" -out "$KEYS/alipay_merchant_local.pem" 2>/dev/null

echo "keys ready in $KEYS:"
ls -la "$KEYS"
# (C-06) 长度恒 32 且首尾字节必须非空白（构造性保证的回归断言——两个
# 消费方 TrimSpace 后仍须得 32 字节）。
[ "$(wc -c < "$KEYS/apiv3.key")" -eq 32 ] && echo "apiv3.key = 32 bytes OK"
python3 - "$KEYS/apiv3.key" <<'PYCHK'
import sys
raw = open(sys.argv[1], "rb").read()
assert len(raw) == 32, f"length {len(raw)}"
first, last = raw[0], raw[-1]
assert first not in b"\t\n\v\f\r ", f"first byte 0x{first:02x} is whitespace"
assert last not in b"\t\n\v\f\r ", f"last byte 0x{last:02x} is whitespace"
print("apiv3.key boundary bytes non-whitespace OK "
      f"(first=0x{first:02x} last=0x{last:02x})")
PYCHK
