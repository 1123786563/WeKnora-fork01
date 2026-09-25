#!/usr/bin/env python3
"""Issue #82 flow-verification helper: local WeChat Pay Native API stub.

Verbatim reuse of the Issue #81 evidence helper (docs/plans/
issue-72-flow-evidence-81/wechat_pay_stub.py) — same rationale: the real
WechatProvider adapter (internal/modules/commercial/payment/wechat.go) is
pointed at this loopback stub via WEKNORA_WECHAT_API_BASE_URL so the
browser checkout flow (CheckoutPage submits provider='wechat') can run the
full purchase chain (gated subscription -> match gate -> channel order)
end-to-end without real channel credentials.

Endpooints implemented (only what the adapter calls):
  POST /v3/pay/transactions/native          -> {"code_url": "weixin://wxpay/bizpayurl?pr=..."}
  GET  /v3/pay/transactions/out-trade-no/*  -> {"trade_state": "NOTPAY", ...}

Loopback only (binds 127.0.0.1). No real payment semantics: the stub never
marks orders paid, so product states stay in the awaiting-payment window
the browser leg of Issue #82 requires (the Alipay leg carries the paid
transition via the signed notify).

Egress note (R1-V09): pointing WEKNORA_WECHAT_API_BASE_URL at 127.0.0.1
REQUIRES the explicit server-side SSRF exemption SSRF_WHITELIST_EXTRA=127.0.0.1.
"""
import json
import secrets
from http.server import BaseHTTPRequestHandler, HTTPServer

HOST, PORT = "127.0.0.1", 8291


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
            self._send(200, {"code_url": "weixin://wxpay/bizpayurl?pr=issue82flow" + secrets.token_hex(6)})
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
