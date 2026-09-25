#!/usr/bin/env python3
"""Issue #82 flow-verification helper: build a SIGNED Alipay async notify.

Mirrors the trusted-channel leg the product code expects (ALI-02): a
form-urlencoded notification whose values are parsed exactly once, signed
with RSA2 (RSA-SHA256 PKCS#1 v1.5, base64) over the sorted k=v& content of
every parameter except sign/sign_type, verified by the REAL
AlipayProvider.Verify (internal/modules/commercial/payment/alipay.go).

Signing uses the openssl CLI (no third-party python deps). The signing key
is the local "alipay side" key pair because no real sandbox credentials
exist in this session (README discloses ac4-sandbox-credentials-unavailable
— this proves the signed-notify -> ConfirmPayment chain, NOT a real
sandbox-wallet payment).

Usage:
  ./alipay_sandbox_notify.py --out-trade-no <id> --trade-no <id> \
      --total-amount 99.00 [--trade-status TRADE_SUCCESS] [--app-id X --seller-id Y]
      -> prints the urlencoded body on stdout

  POST it with:
    curl -s -X POST http://127.0.0.1:8092/api/v1/commercial/callbacks/alipay \
      -H 'Content-Type: application/x-www-form-urlencoded' --data-binary @body.txt
"""
import argparse
import base64
import subprocess
from urllib.parse import urlencode

KEY_DIR = __file__.rsplit("/", 1)[0]
ALIPAY_KEY = f"{KEY_DIR}/alipay_verify_local.pem"


def sign_content(params: dict) -> str:
    """ALI-02 async-notify rule: exclude sign AND sign_type, keys sorted."""
    return "&".join(f"{k}={params[k]}" for k in sorted(params)
                    if k not in ("sign", "sign_type"))


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out-trade-no", required=True)
    ap.add_argument("--trade-no", required=True)
    ap.add_argument("--total-amount", required=True, help="e.g. 99.00")
    ap.add_argument("--trade-status", default="TRADE_SUCCESS")
    ap.add_argument("--app-id", default="2026000000000000")
    ap.add_argument("--seller-id", default="2088000000000000")
    ap.add_argument("--notify-id", default="N82flow000000000000000000001")
    args = ap.parse_args()

    params = {
        "app_id": args.app_id,
        "seller_id": args.seller_id,
        "notify_type": "trade_status_sync",
        "notify_id": args.notify_id,
        "notify_time": "2026-09-25 11:00:00",
        "trade_status": args.trade_status,
        "out_trade_no": args.out_trade_no,
        "trade_no": args.trade_no,
        "total_amount": args.total_amount,
        "receipt_amount": args.total_amount,
        "buyer_pay_amount": args.total_amount,
        "point_amount": "0.00",
        "currency": "CNY",
        "gmt_create": "2026-09-25 10:55:00",
        "gmt_payment": "2026-09-25 10:56:00",
        "version": "1.0",
        "charset": "utf-8",
        "sign_type": "RSA2",
    }
    content = sign_content(params)
    sig = subprocess.run(
        ["openssl", "dgst", "-sha256", "-sign", ALIPAY_KEY],
        input=content.encode(), capture_output=True, check=True).stdout
    params["sign"] = base64.b64encode(sig).decode()
    print(urlencode(params), end="")


if __name__ == "__main__":
    main()
