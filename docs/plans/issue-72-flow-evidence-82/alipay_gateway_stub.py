#!/usr/bin/env python3
"""Issue #82 flow-verification helper: local Alipay open-platform gateway stub.

The real AlipayProvider adapter (internal/modules/commercial/payment/alipay.go)
is configured against this stub via WEKNORA_ALIPAY_GATEWAY_URL so the
alipay.trade.precreate channel order (CreateOrder -> QR code) runs the REAL
adapter code — request signing (merchant key), envelope parsing and response
signature verification (alipay public key) — without real sandbox credentials
(none available in this session; AC4 residual disclosed in README).

Signing uses the openssl CLI (no third-party python deps): RSA-SHA256
PKCS#1 v1.5 per the Alipay gateway protocol (ALI-01), identical to what the
Go adapter verifies.

Key material (generated once, local-only):
  docs/plans/issue-72-flow-evidence-82/alipay_verify_local.pem       RSA-2048 "alipay side" key
  docs/plans/issue-72-flow-evidence-82/alipay_verify_local_pub.pem   PKIX public key -> WEKNORA_ALIPAY_PUBLIC_KEY_PATH
  docs/plans/issue-72-flow-evidence-82/alipay_merchant_local.pem     PKCS#8 -> WEKNORA_ALIPAY_MERCHANT_KEY_PATH
  (same key pair plays both roles locally: we hold no real Alipay key;
   this validates the product code's signing/verification chain, NOT the
   real sandbox wallet — README discloses ac4-sandbox-credentials-unavailable)

Endpoints: a single gateway.do receiving form-urlencoded requests with
method=alipay.trade.precreate | alipay.trade.query | alipay.trade.close.

  precreate -> signed envelope {"alipay_trade_precreate_response":{
                 code 10000, out_trade_no, qr_code "https://qr.alipay.com/..."}}
  query     -> signed envelope trade_status=WAIT_BUYER_PAY (never paid: the
               trusted payment fact only ever arrives via the signed notify)
  close     -> signed envelope code 10000

Response signing mirrors ALI-01: sign covers the ORIGINAL inner JSON bytes
with the "alipay" private key; the adapter verifies those exact bytes
(alipay.go call()). Incoming request signatures (alipayRequestSignContent:
all params except sign, keys sorted, k=v& joined) are verified with the
shared public key and logged — a rejected request signature answers 200 with
an isv.invalid-signature envelope like the real gateway.

Egress note (R1-V09): the adapter validates egress against the shared SSRF
policy, so pointing WEKNORA_ALIPAY_GATEWAY_URL at this 127.0.0.1 stub
REQUIRES an explicit server-side exemption: SSRF_WHITELIST_EXTRA=127.0.0.1.

Every received precreate is logged (timestamp, out_trade_no, total_amount,
subject) to stdout so the flow verifier can lift the out_trade_no for the
signed notify without touching product databases.
"""
import base64
import json
import os
import secrets
import subprocess
import tempfile
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs

# (OCR r4 / plan Task 10) PORT overridable via FLOW82_ALIPAY_PORT: parallel
# verification rounds drift the loopback ports (5272/5273 reserved; 8292 was
# the first round). Default keeps the historical 8292.
HOST, PORT = "127.0.0.1", int(os.environ.get("FLOW82_ALIPAY_PORT", "8292"))
# (OCR r4) KEY_DIR overridable: the local-only key pair may live outside
# the repo (never committed); default stays beside this script.
KEY_DIR = os.environ.get("FLOW82_KEY_DIR") or os.path.dirname(os.path.abspath(__file__))
ALIPAY_KEY = f"{KEY_DIR}/alipay_verify_local.pem"
SHARED_PUB = f"{KEY_DIR}/alipay_verify_local_pub.pem"


def _openssl(args, data: bytes) -> bytes:
    proc = subprocess.run(["openssl", *args], input=data,
                          capture_output=True, check=True)
    return proc.stdout


def rsa_sign_sha256(priv_pem: str, content: bytes) -> bytes:
    return _openssl(["dgst", "-sha256", "-sign", priv_pem], content)


def rsa_verify_sha256(pub_pem: str, content: bytes, sig: bytes) -> bool:
    with tempfile.NamedTemporaryFile(suffix=".sig", delete=False) as f:
        f.write(sig)
        sig_path = f.name
    try:
        proc = subprocess.run(
            ["openssl", "dgst", "-sha256", "-verify", pub_pem,
             "-signature", sig_path],
            input=content, capture_output=True)
        return proc.returncode == 0
    finally:
        # (OCR r2) the module-level os import serves here — no shadowing
        # local import.
        os.unlink(sig_path)


def request_sign_content(params: dict) -> str:
    """ALI-01 outbound rule: all params except sign, keys sorted, k=v& joined."""
    return "&".join(f"{k}={params[k]}" for k in sorted(params) if k != "sign")


def verify_request(params: dict, sign: str) -> bool:
    # (OCR r2) only the DECODE face converts to "signature invalid": a
    # missing/unreadable key file or a failing openssl call is a
    # CONFIGURATION error and must raise (misreading it as a signature
    # mismatch sent verifiers chasing the wrong bug).
    try:
        sig = base64.b64decode(sign, validate=True)
    except (ValueError, TypeError):
        return False
    return rsa_verify_sha256(SHARED_PUB,
                             request_sign_content(params).encode(), sig)


def signed_envelope(method: str, inner: dict) -> bytes:
    inner_json = json.dumps(inner, separators=(",", ":"), ensure_ascii=False)
    key = f"alipay_{method.replace('.', '_')}_response"
    sig = rsa_sign_sha256(ALIPAY_KEY, inner_json.encode())
    envelope = {key: json.loads(inner_json),
                "sign": base64.b64encode(sig).decode(),
                "sign_type": "RSA2"}
    return json.dumps(envelope, separators=(",", ":"), ensure_ascii=False).encode()


class Handler(BaseHTTPRequestHandler):
    def _send_bytes(self, code: int, body: bytes) -> None:
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):  # noqa: N802 (http.server API)
        length = int(self.headers.get("Content-Length") or 0)
        form = parse_qs(self.rfile.read(length).decode())
        params = {k: v[0] for k, v in form.items()}
        method = params.get("method", "")
        sign = params.get("sign", "")
        if not method or not verify_request(params, sign):
            inner = {"code": "40002", "msg": "Invalid Arguments",
                     "sub_code": "isv.invalid-signature",
                     "sub_msg": "stub: request signature rejected"}
            body = json.dumps(inner, separators=(",", ":")).encode()
            self._send_bytes(200, body)  # real gateway answers 200 with error envelope
            print(f"[alipay-stub] REJECTED signature method={method} sign_ok=False",
                  flush=True)
            return
        biz = json.loads(params.get("biz_content", "{}"))
        if method == "alipay.trade.precreate":
            out_trade_no = biz.get("out_trade_no", "")
            print(f"[alipay-stub] PRECREATE out_trade_no={out_trade_no} "
                  f"total_amount={biz.get('total_amount')} subject={biz.get('subject')}",
                  flush=True)
            inner = {"code": "10000", "msg": "Success",
                     "out_trade_no": out_trade_no,
                     "qr_code": f"https://qr.alipay.com/issue82flow{secrets.token_hex(8)}"}
            self._send_bytes(200, signed_envelope("trade.precreate", inner))
            return
        if method == "alipay.trade.query":
            out = biz.get("out_trade_no", "")
            print(f"[alipay-stub] QUERY out_trade_no={out}", flush=True)
            inner = {"code": "10000", "msg": "Success", "out_trade_no": out,
                     "trade_status": "WAIT_BUYER_PAY",
                     "total_amount": biz.get("total_amount", "")}
            self._send_bytes(200, signed_envelope("trade.query", inner))
            return
        if method == "alipay.trade.close":
            out = biz.get("out_trade_no", "")
            print(f"[alipay-stub] CLOSE out_trade_no={out}", flush=True)
            inner = {"code": "10000", "msg": "Success", "out_trade_no": out}
            self._send_bytes(200, signed_envelope("trade.close", inner))
            return
        self._send_bytes(200, json.dumps(
            {"code": "40004", "msg": "Business Failed",
             "sub_code": "aop.UNKNOWN_METHOD",
             "sub_msg": f"stub: method {method} not implemented"}).encode())

    def log_message(self, fmt, *args):  # keep run logs readable
        pass


if __name__ == "__main__":
    print(f"[alipay-stub] listening on http://{HOST}:{PORT} (gateway.do)", flush=True)
    HTTPServer((HOST, PORT), Handler).serve_forever()
