#!/usr/bin/env python3
"""Issue #81 flow-verification helper: local WeChat Pay Native API stub.

The real WechatProvider adapter (internal/modules/commercial/payment/wechat.go)
is configured against this stub via WEKNORA_WECHAT_API_BASE_URL so the
channel-availability PRE-check (purchase.go review F1) passes and the full
purchase chain (gated subscription -> match gate -> channel order) can run
end-to-end without real channel credentials.

Endpoints implemented (only what the adapter calls):
  POST /v3/pay/transactions/native          -> {"code_url": "weixin://wxpay/bizpayurl?pr=..."}
  GET  /v3/pay/transactions/out-trade-no/*  -> {"trade_state": "NOTPAY", ...}

Loopback only (binds 127.0.0.1). No real payment semantics: the stub never
marks orders paid, so product states stay in the awaiting-payment window
that Issue #81's user flow requires.

Egress note (R1-V09): since the channel egress is validated by the shared
SSRF policy (secutils.ValidateURLForSSRF in WechatProvider.do), pointing
WEKNORA_WECHAT_API_BASE_URL at this 127.0.0.1 stub now REQUIRES an explicit,
auditable server-side exemption, e.g.

    SSRF_WHITELIST_EXTRA=127.0.0.1

Replaying the #81 evidence flow against this stub must export that variable
before starting the WeKnora backend; without it the purchase answers 202
with a channel-failed (checkout_error) order (the R3-27 fixed chain): the
SSRF gate refuses the loopback base.
"""
import json
import os
import secrets
from http.server import BaseHTTPRequestHandler, HTTPServer

# (OCR r2, shared with the #82 copy) env-overridable port — parallel
# verification rounds must not collide on the shared default.
HOST, PORT = "127.0.0.1", int(os.environ.get("FLOW82_WECHAT_PORT", "8291"))


class Handler(BaseHTTPRequestHandler):
    def _send(self, code: int, payload: dict) -> None:
        body = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):  # noqa: N802 (http.server API)
        length = int(self.headers.get("Content-Length") or 0)
        self.rfile.read(length)
        if self.path == "/v3/pay/transactions/native":
            self._send(200, {"code_url": "weixin://wxpay/bizpayurl?pr=issue81flow" + secrets.token_hex(6)})
            return
        self._send(404, {"code": "NOT_FOUND", "message": "stub: unknown path"})

    def do_GET(self):  # noqa: N802
        if self.path.startswith("/v3/pay/transactions/out-trade-no/"):
            self._send(200, {
                "out_trade_no": self.path.rstrip("/").rsplit("/", 1)[-1].split("?")[0],
                "trade_state": "NOTPAY",
                "amount": {"total": 0, "currency": "CNY"},
            })
            return
        self._send(404, {"code": "NOT_FOUND", "message": "stub: unknown path"})

    def log_message(self, fmt, *args):  # keep run logs readable
        print("[wechat-stub]", self.address_string(), fmt % args)


if __name__ == "__main__":
    print(f"[wechat-stub] listening on http://{HOST}:{PORT}")
    HTTPServer((HOST, PORT), Handler).serve_forever()
