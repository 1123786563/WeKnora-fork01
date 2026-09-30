#!/usr/bin/env python3
"""Issue #83 flow-verification helper: local WeChat Pay Native APIv3 stub.

The REAL WechatProvider adapter (internal/modules/commercial/payment/wechat.go)
is configured against this stub via WEKNORA_WECHAT_API_BASE_URL so the native
channel order (Create -> code_url), Query (recovery) and Close (the #83
close-race orchestration) run the REAL adapter code — request signing
(merchant RSA-SHA256 over METHOD\\nPATH\\nts\\nnonce\\nbody\\n), trade-state
mapping and the ORDER_PAID close error — without real WeChat merchant
credentials (none available; disclosed in README as the WeChat-side
ac4-sandbox-credentials-unavailable boundary, mirroring R-4's Alipay rule:
local RSA/AES proves protocol round-trip + signature verification, NOT a
real WeChat-wallet payment).

Channel endpoints (only what the adapter calls):
  POST /v3/pay/transactions/native
      Verifies the outbound Authorization WECHATPAY2-SHA256-RSA2048 header
      with the merchant PUBLIC key (mismatch -> 401, fail-closed), registers
      out_trade_no -> {total, state=NOTPAY} and answers a code_url.
  GET  /v3/pay/transactions/out-trade-no/{id}?mchid=
      Answers from the state table (out_trade_no, trade_state, amount).
  POST /v3/pay/transactions/out-trade-no/{id}/close
      state SUCCESS -> 400 {"code":"ORDER_PAID"} (the race shape Task 1
      pinned: "wechat api status 400 code=ORDER_PAID"); else CLOSED -> 204.

Orchestration endpoints (loopback only):
  POST /stub/mark   {"out_trade_no","state","transaction_id"} — move state.
  POST /stub/notify {"out_trade_no"} — push the SIGNED callback to the
      WeKnora notify URL: TRANSACTION.SUCCESS envelope, resource encrypted
      AES-256-GCM with the APIv3 key (nonce 12B, AAD "transaction"), signed
      RSA-SHA256 (platform private key) over ts\\nnonce\\nbody\\n, delivered
      with Wechatpay-Serial/Timestamp/Nonce/Signature headers. Asserts the
      WeKnora answer is HTTP 200 {"code":"SUCCESS"} and logs both sides.
  GET  /stub/orders — state-table dump (evidence/inspection).

Credentials discipline: every key path / identity comes from the environment
(WECHAT_STUB_*); the source carries ZERO key literals. Egress note: the
adapter validates egress against the shared SSRF policy, so pointing
WEKNORA_WECHAT_API_BASE_URL at this 127.0.0.1 stub REQUIRES the explicit
server-side exemption SSRF_WHITELIST_EXTRA=127.0.0.1.

Usage: python3 wechat_native_stub.py   (binds 127.0.0.1:8296 by default)
"""
import base64
import json
import os
import secrets
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

HOST, PORT = "127.0.0.1", int(os.environ.get("WECHAT_STUB_PORT", "8296"))
APP_ID = os.environ.get("WECHAT_STUB_APP_ID", "")
MCH_ID = os.environ.get("WECHAT_STUB_MCH_ID", "")
PLATFORM_SERIAL = os.environ.get("WECHAT_STUB_PLATFORM_SERIAL", "")
NOTIFY_URL = os.environ.get("WECHAT_STUB_NOTIFY_URL", "")
KEY_DIR = os.environ.get("WECHAT_STUB_KEY_DIR", "")

REQUIRED = {
    "WECHAT_STUB_APP_ID": APP_ID,
    "WECHAT_STUB_MCH_ID": MCH_ID,
    "WECHAT_STUB_PLATFORM_SERIAL": PLATFORM_SERIAL,
    "WECHAT_STUB_NOTIFY_URL": NOTIFY_URL,
    "WECHAT_STUB_KEY_DIR": KEY_DIR,
}
missing = [k for k, v in REQUIRED.items() if not v]
if missing:
    print("missing required env: %s (no source-code fallback)" % ",".join(missing), file=sys.stderr)
    sys.exit(2)


def _load_merchant_public():
    with open(os.path.join(KEY_DIR, "mch_public.pem"), "rb") as f:
        return serialization.load_pem_public_key(f.read())


def _load_platform_private():
    with open(os.path.join(KEY_DIR, "platform_priv.pem"), "rb") as f:
        return serialization.load_pem_private_key(f.read(), password=None)


def _load_apiv3():
    with open(os.path.join(KEY_DIR, "apiv3.key"), "rb") as f:
        key = f.read().strip()
    if len(key) != 32:
        raise SystemExit("apiv3.key must hold exactly 32 bytes, got %d" % len(key))
    return key


MCH_PUBLIC = _load_merchant_public()
PLAT_PRIV = _load_platform_private()
APIV3 = _load_apiv3()

# out_trade_no -> {"total": int, "state": "NOTPAY"|"SUCCESS"|"CLOSED",
#                  "transaction_id": str}
ORDERS = {}


def log(line):
    print("[%s] %s" % (time.strftime("%H:%M:%S"), line), flush=True)


def parse_authorization(header):
    """Split 'WECHATPAY2-SHA256-RSA2048 k="v",...' into a dict."""
    parts = {}
    if not header.startswith("WECHATPAY2-SHA256-RSA2048"):
        return parts
    for chunk in header[len("WECHATPAY2-SHA256-RSA2048"):].split(","):
        chunk = chunk.strip()
        if "=" in chunk:
            k, v = chunk.split("=", 1)
            parts[k.strip()] = v.strip().strip('"')
    return parts


def verify_outbound_signature(method, uri, body, auth):
    """The adapter signs METHOD\\nPATH(=uri)\\nts\\nnonce\\nbody\\n."""
    fields = parse_authorization(auth)
    if not {"mchid", "nonce_str", "signature", "timestamp"} <= set(fields):
        return False, "missing authorization fields"
    if fields.get("mchid") != MCH_ID:
        return False, "mchid mismatch"
    message = "\n".join([method, uri, fields["timestamp"], fields["nonce_str"],
                         body.decode("utf-8"), ""]).encode("utf-8")
    try:
        MCH_PUBLIC.verify(
            base64.b64decode(fields["signature"]), message,
            padding.PKCS1v15(), hashes.SHA256())
    except Exception:
        return False, "signature verify failed"
    return True, ""


def signed_callback_body(out_trade_no):
    order = ORDERS.get(out_trade_no)
    if order is None:
        return None, "unknown out_trade_no"
    plain = json.dumps({
        "mchid": MCH_ID,
        "appid": APP_ID,
        "out_trade_no": out_trade_no,
        "transaction_id": order.get("transaction_id") or ("wx_txn_" + out_trade_no),
        "trade_state": order["state"],
        "amount": {"total": order["total"], "currency": "CNY"},
    }, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    nonce = secrets.token_hex(6)  # 12 bytes
    ct = AESGCM(APIV3).encrypt(nonce.encode(), plain, b"transaction")
    return json.dumps({
        "id": "evt-stub-" + secrets.token_hex(8),
        "event_type": "TRANSACTION.SUCCESS",
        "create_time": time.strftime("%Y-%m-%dT%H:%M:%S+08:00"),
        "resource": {
            "algorithm": "AEAD_AES_256_GCM",
            "ciphertext": base64.b64encode(ct).decode(),
            "associated_data": "transaction",
            "nonce": nonce,
        },
    }, separators=(",", ":")).encode("utf-8"), None


def push_notification(out_trade_no):
    """Sign + POST the callback to WeKnora; assert the SUCCESS ack."""
    import urllib.request
    body, err = signed_callback_body(out_trade_no)
    if body is None:
        return False, err
    ts = str(int(time.time()))
    nonce = secrets.token_hex(8)
    message = "\n".join([ts, nonce, body.decode("utf-8"), ""]).encode("utf-8")
    sig = PLAT_PRIV.sign(message, padding.PKCS1v15(), hashes.SHA256())
    req = urllib.request.Request(NOTIFY_URL, data=body, method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("Wechatpay-Serial", PLATFORM_SERIAL)
    req.add_header("Wechatpay-Timestamp", ts)
    req.add_header("Wechatpay-Nonce", nonce)
    req.add_header("Wechatpay-Signature", base64.b64encode(sig).decode())
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            payload = resp.read().decode("utf-8")
            log("NOTIFY PUSH out_trade_no=%s -> HTTP %d %s" % (out_trade_no, resp.status, payload))
            return resp.status == 200 and '"SUCCESS"' in payload, payload
    except Exception as exc:  # noqa: BLE001 - evidence log wants the raw error
        log("NOTIFY PUSH out_trade_no=%s FAILED: %s" % (out_trade_no, exc))
        return False, str(exc)


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _reply(self, status, payload):
        body = payload if isinstance(payload, bytes) else json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self):
        length = int(self.headers.get("Content-Length") or 0)
        return self.rfile.read(length) if length else b""

    def do_GET(self):  # noqa: N802 - http.server naming
        path, _, query = self.path.partition("?")
        if path == "/stub/orders":
            self._reply(200, json.dumps(ORDERS, ensure_ascii=False).encode("utf-8"))
            return
        if path.startswith("/v3/pay/transactions/out-trade-no/"):
            order_id = path[len("/v3/pay/transactions/out-trade-no/"):]
            auth = self.headers.get("Authorization", "")
            ok, why = verify_outbound_signature("GET", self.path, b"", auth)
            if not ok:
                log("QUERY %s REJECTED (%s)" % (order_id, why))
                self._reply(401, {"code": "SIGN_ERROR", "message": why})
                return
            order = ORDERS.get(order_id)
            if order is None:
                self._reply(404, {"code": "ORDER_NOT_EXIST", "message": "order not found"})
                return
            log("QUERY %s -> trade_state=%s total=%d" % (order_id, order["state"], order["total"]))
            self._reply(200, {
                "out_trade_no": order_id,
                "transaction_id": order.get("transaction_id") or "",
                "trade_state": order["state"],
                "amount": {"total": order["total"], "currency": "CNY"},
            })
            return
        self._reply(404, {"code": "NOT_FOUND", "message": path})

    def do_POST(self):  # noqa: N802 - http.server naming
        body = self._read_body()
        path = self.path
        if path == "/stub/mark":
            data = json.loads(body.decode("utf-8"))
            order = ORDERS.get(data["out_trade_no"])
            if order is None:
                self._reply(404, {"error": "unknown out_trade_no"})
                return
            order["state"] = data["state"]
            if data.get("transaction_id"):
                order["transaction_id"] = data["transaction_id"]
            log("MARK %s -> state=%s txn=%s" % (data["out_trade_no"], order["state"],
                                                order.get("transaction_id", "")))
            self._reply(200, {"ok": True, "order": order})
            return
        if path == "/stub/notify":
            data = json.loads(body.decode("utf-8"))
            ok, detail = push_notification(data["out_trade_no"])
            self._reply(200 if ok else 502, {"ok": ok, "detail": detail})
            return
        if path == "/v3/pay/transactions/native":
            auth = self.headers.get("Authorization", "")
            ok, why = verify_outbound_signature("POST", "/v3/pay/transactions/native", body, auth)
            if not ok:
                log("NATIVE REJECTED (%s)" % why)
                self._reply(401, {"code": "SIGN_ERROR", "message": why})
                return
            req = json.loads(body.decode("utf-8"))
            out_trade_no = req.get("out_trade_no", "")
            total = int((req.get("amount") or {}).get("total", 0))
            if not out_trade_no or total <= 0:
                self._reply(400, {"code": "INVALID_REQUEST", "message": "out_trade_no/amount required"})
                return
            ORDERS[out_trade_no] = {"total": total, "state": "NOTPAY"}
            code_url = "weixin://wxpay/bizpayurl?pr=issue83" + secrets.token_hex(6)
            log("NATIVE out_trade_no=%s total=%d appid=%s mchid=%s -> code_url=%s"
                % (out_trade_no, total, req.get("appid"), req.get("mchid"), code_url))
            self._reply(200, {"code_url": code_url})
            return
        if path.startswith("/v3/pay/transactions/out-trade-no/") and path.endswith("/close"):
            order_id = path[len("/v3/pay/transactions/out-trade-no/"):-len("/close")]
            auth = self.headers.get("Authorization", "")
            ok, why = verify_outbound_signature("POST", path, body, auth)
            if not ok:
                log("CLOSE %s REJECTED (%s)" % (order_id, why))
                self._reply(401, {"code": "SIGN_ERROR", "message": why})
                return
            order = ORDERS.get(order_id)
            if order is None:
                self._reply(404, {"code": "ORDER_NOT_EXIST", "message": "order not found"})
                return
            if order["state"] == "SUCCESS":
                log("CLOSE %s -> 400 ORDER_PAID (race shape)" % order_id)
                self._reply(400, {"code": "ORDER_PAID", "message": "订单已支付，禁止关单"})
                return
            order["state"] = "CLOSED"
            log("CLOSE %s -> 204 (closed)" % order_id)
            self.send_response(204)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self._reply(404, {"code": "NOT_FOUND", "message": path})

    def log_message(self, fmt, *args):  # route access logs through log()
        log("http " + (fmt % args))


if __name__ == "__main__":
    server = ThreadingHTTPServer((HOST, PORT), Handler)
    log("wechat native stub listening on %s:%d (app=%s mch=%s serial=%s notify=%s)"
        % (HOST, PORT, APP_ID, MCH_ID, PLATFORM_SERIAL, NOTIFY_URL))
    server.serve_forever()
