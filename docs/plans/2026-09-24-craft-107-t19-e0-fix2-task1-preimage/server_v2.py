#!/usr/bin/env python3
"""Synthetic OpenAI-compatible adapter/direct listener with append-only ledgers."""
from __future__ import annotations

import argparse
import hashlib
import http.server
import json
import os
import socketserver
import threading
import time
import uuid
from pathlib import Path

RUN_ID = ""
ROLE = ""
STATE = Path("/state")
LEDGER = Path("/ledger")
BOOT_ID = str(uuid.uuid4())
LOCK = threading.Lock()
SEQ = 0


def append(event: dict) -> None:
    global SEQ
    with LOCK:
        SEQ += 1
        port = event.get("port")
        role = ({8080: "gateway", 8081: "provider", 8082: "proxy", 8083: "host"}.get(port, "direct") if ROLE == "direct" else "adapter")
        event.update(run_id=RUN_ID, boot_id=BOOT_ID, seq=SEQ, time_ns=time.time_ns(), role=role)
        path = LEDGER / f"{role}.jsonl"
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("a", encoding="utf-8") as out:
            out.write(json.dumps(event, sort_keys=True, separators=(",", ":")) + "\n")
            out.flush()
            os.fsync(out.fileno())


class LedgerHTTPServer(http.server.ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = False

    def get_request(self):
        sock, peer = super().get_request()
        append({"event": "tcp_accept", "peer": f"{peer[0]}:{peer[1]}", "port": self.server_port})
        return sock, peer


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server_version = "E0Synthetic/2"
    sys_version = ""

    def log_message(self, *_args):
        return

    def _body(self):
        try:
            size = min(int(self.headers.get("Content-Length", "0")), 2_000_000)
        except ValueError:
            size = 0
        return self.rfile.read(size) if size else b""

    @staticmethod
    def _purpose(payload):
        try:
            messages = payload.get("messages", [])
            system = " ".join(str(m.get("content", "")) for m in messages if m.get("role") == "system")
            if "title generator" in system.lower():
                return "title"
            if messages:
                return "task"
        except Exception:
            pass
        return "unknown"

    def _record(self, body=b"", status=None, complete=False, fault="normal", purpose=None, payload=None):
        try:
            active_mode = json.loads((STATE / "adapter-mode.json").read_text())
            # Both protected adapter requests and direct-sink observations use
            # the runner's current case identity. Explicit request headers win.
            # The state directory is host-owned and not mounted into the client.
            active_case = active_mode.get("case")
        except Exception:
            active_case = None
        append({
            "event": "http", "port": self.server.server_port, "method": self.command,
            "path": self.path, "peer": f"{self.client_address[0]}:{self.client_address[1]}",
            "case": self.headers.get("X-E0-Case") or active_case,
            "session_id": self.headers.get("x-session-id"),
            "session_affinity": self.headers.get("x-session-affinity"),
            "body_sha256": hashlib.sha256(body).hexdigest(), "purpose": purpose,
            "model": payload.get("model") if isinstance(payload, dict) else None,
            "stream": payload.get("stream") if isinstance(payload, dict) else None,
            "fault": fault, "status": status, "complete": complete,
        })

    def _json(self, status, obj, extra_headers=None):
        raw = json.dumps(obj).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        for key, value in (extra_headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(raw)
        self.wfile.flush()

    def _sse(self, content, completion_id=None):
        cid = completion_id or "chatcmpl-e0-" + uuid.uuid4().hex
        now = int(time.time())
        frames = [
            {"id": cid, "object": "chat.completion.chunk", "created": now, "model": "mock-model", "choices": [{"index": 0, "delta": {"role": "assistant"}, "finish_reason": None}]},
            {"id": cid, "object": "chat.completion.chunk", "created": now, "model": "mock-model", "choices": [{"index": 0, "delta": {"content": content}, "finish_reason": None}]},
            {"id": cid, "object": "chat.completion.chunk", "created": now, "model": "mock-model", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]},
            {"id": cid, "object": "chat.completion.chunk", "created": now, "model": "mock-model", "choices": [], "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}},
        ]
        raw = b"".join(b"data: " + json.dumps(frame, separators=(",", ":")).encode() + b"\n\n" for frame in frames) + b"data: [DONE]\n\n"
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)
        self.wfile.flush()

    def _handle(self):
        body = self._body()
        payload = {}
        try:
            payload = json.loads(body or b"{}")
        except Exception:
            pass
        purpose = self._purpose(payload)
        if ROLE == "adapter":
            mode_file = STATE / "adapter-mode.json"
            try:
                mode = json.loads(mode_file.read_text())
            except Exception:
                mode = {"mode": "normal"}
            fault = mode.get("mode", "normal")
            if self.command == "CONNECT" or not self.path.startswith("/v1/"):
                self._record(body, 403, False, fault, purpose, payload)
                self._json(403, {"error": "adapter endpoint only"})
                return
            if self.command != "POST":
                self._record(body, 405, False, fault, purpose, payload)
                self._json(405, {"error": "POST required"})
                return
            if fault in ("redirect307", "redirect308") and purpose == "task":
                status = 307 if fault == "redirect307" else 308
                self._record(body, status, False, fault, purpose, payload)
                self._json(status, {"error": "redirect control"}, {"Location": mode["location"]})
                return
            if fault == "fault503" and purpose == "task" and not mode.get("task_failed"):
                mode["task_failed"] = True
                mode_file.write_text(json.dumps(mode))
                self._record(body, 503, False, fault, purpose, payload)
                self._json(503, {"error": {"message": "synthetic 503", "type": "server_error"}})
                return
            if fault == "disconnect" and purpose == "task" and not mode.get("task_failed"):
                mode["task_failed"] = True
                mode_file.write_text(json.dumps(mode))
                self._record(body, None, False, fault, purpose, payload)
                self.close_connection = True
                self.connection.shutdown(1)
                self.connection.close()
                return
            self._record(body, 200, True, fault, purpose, payload)
            self._sse("Probe title" if purpose == "title" else "probe-ok")
            return

        port = self.server.server_port
        if self.command == "CONNECT":
            self._record(body, 200, True, "sink", None, payload)
            self.send_response(200, "Connection Established")
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self._record(body, 200, True, "sink", purpose, payload)
        self._json(200, {"ok": True, "role": {8080: "gateway", 8081: "provider", 8082: "proxy", 8083: "host"}.get(port, "direct"), "method": self.command, "path": self.path})

    do_GET = do_POST = do_PUT = do_DELETE = do_OPTIONS = do_PATCH = do_HEAD = _handle
    do_CONNECT = _handle

    def handle_expect_100(self):
        return True

    def send_error(self, code, message=None, explain=None):
        body = b""
        try:
            append({"event": "http_parse_error", "port": self.server.server_port, "method": getattr(self, "command", "MALFORMED"), "path": getattr(self, "path", ""), "peer": f"{self.client_address[0]}:{self.client_address[1]}", "status": code, "message": str(message or "")})
        except Exception:
            pass
        self.close_connection = True


def main():
    global RUN_ID, ROLE, STATE, LEDGER
    p = argparse.ArgumentParser()
    p.add_argument("--role", choices=("adapter", "direct"), required=True)
    p.add_argument("--run-id", required=True)
    p.add_argument("--state", default="/state")
    p.add_argument("--ledger", default="/ledger")
    a = p.parse_args()
    RUN_ID, ROLE, STATE, LEDGER = a.run_id, a.role, Path(a.state), Path(a.ledger)
    STATE.mkdir(parents=True, exist_ok=True)
    ports = [8080, 8081, 8082, 8083] if ROLE == "direct" else [8080]
    servers = []
    for port in ports:
        server = LedgerHTTPServer(("0.0.0.0", port), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        servers.append((server, thread))
    ready = Path("/state") / f"{ROLE}-ready.json"
    ready.write_text(json.dumps({"run_id": RUN_ID, "boot_id": BOOT_ID, "role": ROLE, "ports": ports}))
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        pass
    finally:
        for server, _ in servers:
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    main()
