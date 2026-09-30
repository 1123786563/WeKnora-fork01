#!/usr/bin/env python3
"""Issue #84 flow-verification helper: the #83 WeChat Native stub COPY with
amount/currency OVERRIDE orchestration (the anomaly round's whole point: the
83 original's /stub/mark only moves state — its callback and Query amounts
always answer the pre-registered order total, so a tampered-amount collection
cannot be staged; see plan docs/plans/issue-72-plan-84.md Task 7 Step 0 /
review R2).

Deltas from the 83 original (wechat_native_stub.py — NOT modified):
  1. POST /stub/mark additionally accepts optional "total" (fen int) and
     "currency" override fields; a hit stores override_total /
     override_currency on the order row.
  2. The signed callback body AND the Query response build amount from
     order.get("override_total", order["total"]) and
     order.get("override_currency", "CNY") — both faces stay same-sourced
     (a mismatch between them would be its own artifact, never evidence).
  3. --selftest runs the copy's smoke proof in-process: no-override orders
     byte-match the 83 construction; an override lands in the decrypted
     callback amount (verifies through the same AESGCM round-trip the real
     adapter performs).

Everything else — outbound signature verification, ORDER_PAID close race,
notify push, credential discipline (WECHAT_STUB_* env only, zero key
literals) — is the 83 original verbatim.

Usage: python3 wechat_native_stub_anomaly.py          (binds 127.0.0.1:8298)
       python3 wechat_native_stub_anomaly.py --selftest
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

HOST, PORT = "127.0.0.1", int(os.environ.get("WECHAT_STUB_PORT", "8298"))
APP_ID = os.environ.get("WECHAT_STUB_APP_ID", "")
MCH_ID = os.environ.get("WECHAT_STUB_MCH_ID", "")
PLATFORM_SERIAL = os.environ.get("WECHAT_STUB_PLATFORM_SERIAL", "")
NOTIFY_URL = os.environ.get("WECHAT_STUB_NOTIFY_URL", "")
KEY_DIR = os.environ.get("WECHAT_STUB_KEY_DIR", "")

# (OCR84-R1-34) 复制 83 原版时被裁掉的模块级必需 env 校验块（原版
# wechat_native_stub.py:63-73）：任一 WECHAT_STUB_* env 缺失时快速失败——
# KEY_DIR 未设时 os.path.join("", ...) 退化为相对 cwd 的裸文件名，轻则难定位
# 的 FileNotFoundError，重则静默加载 cwd 中恰好同名的无关密钥文件；NOTIFY_URL
# 为空会推迟到 /stub/notify 才以 urllib ValueError 崩溃。
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

# out_trade_no -> {"total": int, "state": ..., "transaction_id": str,
#                  "override_total": int?, "override_currency": str?}
ORDERS = {}


def log(line):
    print("[%s] %s" % (time.strftime("%H:%M:%S"), line), flush=True)


def effective_amount(order):
    """(#84 R2) The collection face: the override when staged, the
    pre-registered total otherwise. Both the callback and Query answers use
    THIS one function — same-sourced by construction."""
    return int(order.get("override_total", order["total"])), order.get("override_currency", "CNY")


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
    total, currency = effective_amount(order)
    plain = json.dumps({
        "mchid": MCH_ID,
        "appid": APP_ID,
        "out_trade_no": out_trade_no,
        "transaction_id": order.get("transaction_id") or ("wx_txn_" + out_trade_no),
        "trade_state": order["state"],
        "amount": {"total": total, "currency": currency},
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
    """Sign + POST the callback to WeKnora; assert the SUCCESS ack.

    (#84 Task 2 contract) A tampered-amount collection is ACKED 200
    SUCCESS too (the fact is retained terminally — WeKnora answers success
    so the channel stops retrying), so the 200 assert below holds for the
    anomaly legs exactly as for the normal leg. A NON-2xx answer here means
    the callback face regressed — the stub reports it as a failed push.
    """
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
            total, currency = effective_amount(order)
            log("QUERY %s -> trade_state=%s total=%d currency=%s"
                % (order_id, order["state"], total, currency))
            self._reply(200, {
                "out_trade_no": order_id,
                "transaction_id": order.get("transaction_id") or "",
                "trade_state": order["state"],
                "amount": {"total": total, "currency": currency},
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
            # (#84 R2) Optional amount/currency overrides stage the anomaly
            # collection face (partial payment / wrong amount / wrong
            # currency) for BOTH the callback and the Query answers.
            if data.get("total") is not None:
                order["override_total"] = int(data["total"])
            if data.get("currency"):
                order["override_currency"] = data["currency"]
            log("MARK %s -> state=%s txn=%s override_total=%s override_currency=%s"
                % (data["out_trade_no"], order["state"], order.get("transaction_id", ""),
                   order.get("override_total", "-"), order.get("override_currency", "-")))
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
            code_url = "weixin://wxpay/bizpayurl?pr=issue84" + secrets.token_hex(6)
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


def selftest():
    """(#84 Step 0 pass criterion) The copy's smoke proof, in-process:
    (a) a NO-override order's decrypted callback amount equals the
        pre-registered total — byte-equivalent behaviour to the 83 original;
    (b) an override lands in the decrypted callback amount (the same
    AESGCM round-trip the real adapter performs) and in effective_amount.
    Exits 0 on success, 1 on any mismatch."""
    ORDERS.clear()
    ORDERS["mo_plain"] = {"total": 9900, "state": "SUCCESS"}
    body, err = signed_callback_body("mo_plain")
    assert body is not None, err
    env = json.loads(body.decode("utf-8"))
    ct = base64.b64decode(env["resource"]["ciphertext"])
    plain = json.loads(AESGCM(APIV3).decrypt(
        env["resource"]["nonce"].encode(), ct, b"transaction"))
    if plain["amount"]["total"] != 9900 or plain["amount"]["currency"] != "CNY":
        print("SELFTEST FAIL: no-override callback amount drifted: %s" % plain["amount"])
        return 1
    # Stage the override exactly as /stub/mark would.
    ORDERS["mo_tamper"] = {"total": 9900, "state": "SUCCESS",
                           "override_total": 5000, "override_currency": "CNY"}
    body, err = signed_callback_body("mo_tamper")
    assert body is not None, err
    env = json.loads(body.decode("utf-8"))
    ct = base64.b64decode(env["resource"]["ciphertext"])
    plain = json.loads(AESGCM(APIV3).decrypt(
        env["resource"]["nonce"].encode(), ct, b"transaction"))
    if plain["amount"]["total"] != 5000:
        print("SELFTEST FAIL: override not carried into the callback amount: %s" % plain["amount"])
        return 1
    total, currency = effective_amount(ORDERS["mo_tamper"])
    if (total, currency) != (5000, "CNY"):
        print("SELFTEST FAIL: effective_amount override face: %s %s" % (total, currency))
        return 1
    ORDERS["mo_fx"] = {"total": 9900, "state": "SUCCESS", "override_currency": "USD"}
    total, currency = effective_amount(ORDERS["mo_fx"])
    if (total, currency) != (9900, "USD"):
        print("SELFTEST FAIL: currency override face: %s %s" % (total, currency))
        return 1
    print("SELFTEST OK: no-override matches the 83 construction; "
          "overrides land in both the callback amount and the query face")
    return 0


if __name__ == "__main__":
    if "--selftest" in sys.argv:
        sys.exit(selftest())
    server = ThreadingHTTPServer((HOST, PORT), Handler)
    log("wechat native stub (anomaly copy) listening on %s:%d (app=%s mch=%s serial=%s notify=%s)"
        % (HOST, PORT, APP_ID, MCH_ID, PLATFORM_SERIAL, NOTIFY_URL))
    server.serve_forever()
