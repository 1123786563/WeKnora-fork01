#!/usr/bin/env python3
"""Synthetic adapter/direct listeners with source-owned recorder streams."""
from __future__ import annotations

import argparse
import hashlib
import http.server
import json
import os
import socket
import socketserver
import threading
import time
import uuid
from pathlib import Path
from urllib.parse import urlsplit

class RecorderError(RuntimeError):
    """A source stream can no longer produce complete trustworthy evidence."""


class _ControlHandler(socketserver.StreamRequestHandler):
    def handle(self):
        raw = self.rfile.readline(64 * 1024)
        try:
            request = json.loads(raw)
            response = self.server.recorder.command(request)
        except Exception as exc:
            response = {"ok": False, "error": str(exc)}
        try:
            self.wfile.write(json.dumps(response, sort_keys=True).encode() + b"\n")
            self.wfile.flush()
        except OSError:
            pass


class _UnixControlServer(socketserver.ThreadingUnixStreamServer):
    daemon_threads = True
    allow_reuse_address = False


class SourceRecorder:
    """Single-writer event stream controlled through a private host Unix socket."""

    def __init__(self, run_id, source, ports, evidence_dir, control_socket):
        if source not in {"direct", "adapter"}:
            raise ValueError("source must be direct or adapter")
        self.run_id = run_id
        self.source = source
        self.ports = sorted(int(port) for port in ports)
        self.evidence_dir = Path(evidence_dir)
        self.events_path = self.evidence_dir / "events.jsonl"
        self.control_socket = Path(control_socket)
        self.boot_id = str(uuid.uuid4())
        self.lock = threading.RLock()
        self.seq = 0
        self.ordinal = 0
        self.used_phase_ids = set()
        self.phase_id = None
        self.phase_class = None
        self.mode = "normal"
        self.redirect_location = ""
        self.connections = {}
        self.connection_ordinal = 0
        self.active_requests = set()
        self.fault_used = False
        self.fatal_error = None
        self.control_server = None
        self.control_thread = None
        self.sealed = False
        self.on_seal = None

    def _healthy(self):
        if self.fatal_error is not None:
            raise RecorderError(f"recorder poisoned: {self.fatal_error}")
        if self.sealed:
            raise RecorderError("recorder already sealed")

    def append(self, event):
        with self.lock:
            self._healthy()
            row = dict(event)
            row.update(run_id=self.run_id, source=self.source, boot_id=self.boot_id,
                       seq=self.seq + 1, time_ns=time.time_ns(), monotonic_ns=time.monotonic_ns(),
                       phase_id=row.get("phase_id", self.phase_id))
            try:
                encoded = json.dumps(row, sort_keys=True, separators=(",", ":")) + "\n"
                fd = os.open(self.events_path, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)
                try:
                    os.fchmod(fd, 0o600)
                    with os.fdopen(fd, "a", encoding="utf-8", closefd=True) as out:
                        fd = -1
                        out.write(encoded)
                        out.flush()
                        os.fsync(out.fileno())
                finally:
                    if fd >= 0:
                        os.close(fd)
                self.seq += 1
                return row
            except Exception as exc:
                self.fatal_error = f"append failed at seq {self.seq + 1}: {exc}"
                raise RecorderError(self.fatal_error) from exc

    def start(self):
        self.evidence_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.control_socket.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        os.chmod(self.evidence_dir, 0o700)
        os.chmod(self.control_socket.parent, 0o700)
        if self.events_path.exists() or self.control_socket.exists():
            raise RecorderError("source evidence path already exists")
        self.append({"event": "stream_start", "source_identity": self.source, "ports": self.ports, "initial_mode": self.mode})
        server = _UnixControlServer(str(self.control_socket), _ControlHandler)
        server.recorder = self
        os.chmod(self.control_socket, 0o600)
        self.control_server = server
        self.control_thread = threading.Thread(target=server.serve_forever, name=f"{self.source}-control", daemon=True)
        self.control_thread.start()

    def stop_control(self):
        server = self.control_server
        if server is not None:
            server.shutdown()
            server.server_close()
            self.control_server = None
        if self.control_socket.exists():
            self.control_socket.unlink()

    def command(self, request):
        with self.lock:
            self._healthy()
            expected_keys = {"ordinal", "op", "phase_id"}
            if isinstance(request, dict) and request.get("op") == "begin_phase":
                expected_keys.add("mode")
            if not isinstance(request, dict) or set(request) != expected_keys:
                raise RecorderError("invalid control command schema")
            ordinal = request.get("ordinal")
            if type(ordinal) is not int or ordinal != self.ordinal + 1:
                raise RecorderError("control ordinal must increase by exactly one")
            op = request.get("op")
            phase_id = request.get("phase_id")
            if op == "begin_phase":
                mode = request.get("mode")
                if self.phase_id is not None or self.connections or self.active_requests:
                    raise RecorderError("cannot begin phase while another phase or connection is active")
                base_mode, separator, option = mode.partition("|") if isinstance(mode, str) else (None, "", "")
                allowed_modes = {"normal", "control", "negative", "redirect307", "redirect308", "fault503", "disconnect"}
                if not isinstance(phase_id, str) or not phase_id or phase_id in self.used_phase_ids or base_mode not in allowed_modes:
                    raise RecorderError("invalid or reused phase identity/mode")
                redirect_location = ""
                if base_mode in {"redirect307", "redirect308"}:
                    parsed = urlsplit(option) if separator else None
                    if parsed is None or parsed.scheme != "http" or not parsed.hostname or parsed.username or parsed.password or parsed.fragment:
                        raise RecorderError("redirect mode requires a host-selected HTTP Location")
                    redirect_location = option
                elif separator:
                    raise RecorderError("phase options are allowed only for redirect modes")
                phase_class = "control" if base_mode == "control" else "negative"
                self.phase_id, self.phase_class, self.mode = phase_id, phase_class, base_mode
                self.used_phase_ids.add(phase_id)
                self.redirect_location = redirect_location
                self.fault_used = False
                row = self.append({"event": "phase_begin", "ordinal": ordinal, "phase_id": phase_id, "phase_class": phase_class, "mode": mode, "active_connections": 0, "active_requests": 0})
            elif op == "barrier":
                if self.phase_id != phase_id:
                    raise RecorderError("barrier phase does not match current phase")
                if self.connections or self.active_requests:
                    raise RecorderError(f"barrier refused with active connections={len(self.connections)} requests={len(self.active_requests)}")
                row = self.append({"event": "phase_barrier", "ordinal": ordinal, "phase_id": phase_id, "phase_class": self.phase_class, "active_connections": 0, "active_requests": 0})
                self.phase_id, self.phase_class, self.mode, self.redirect_location = None, None, "normal", ""
            elif op == "seal":
                if phase_id is not None or self.phase_id is not None:
                    raise RecorderError("seal requires no active phase")
                if self.connections or self.active_requests:
                    raise RecorderError("seal refused with active connection or request")
                row = self.append({"event": "stream_end", "ordinal": ordinal, "final_seq": self.seq + 1, "event_count": self.seq + 1, "active_connections": 0, "active_requests": 0})
                self.ordinal = ordinal
                self.sealed = True
                if self.on_seal is not None:
                    self.on_seal()
                return {"ok": True, "source": self.source, "phase_id": None, "stream_cursor": row["seq"], "sealed": True}
            else:
                raise RecorderError("unknown control operation")
            self.ordinal = ordinal
            return {"ok": True, "source": self.source, "phase_id": phase_id, "stream_cursor": row["seq"]}

    def accept(self, peer, local):
        with self.lock:
            self.connection_ordinal += 1
            row = self.append({"event": "tcp_accept", "connection_id": f"{self.source}-c{self.connection_ordinal}", "peer": peer, "local": local, "local_port": int(local.rsplit(":", 1)[-1].rstrip("]")), "phase_id": self.phase_id})
            self.connections[row["connection_id"]] = {"phase_id": self.phase_id, "request_ordinal": 0}
            return row["connection_id"]

    def request_start(self, connection_id, method, path, untrusted_case_header=None):
        with self.lock:
            connection = self.connections[connection_id]
            connection["request_ordinal"] += 1
            request_id = f"{connection_id}-r{connection['request_ordinal']}"
            row = self.append({"event": "request_start", "connection_id": connection_id, "request_id": request_id, "request_ordinal": connection["request_ordinal"], "phase_id": connection["phase_id"], "method": method, "path": path, "untrusted_case_header": untrusted_case_header})
            self.active_requests.add(request_id)
            return row

    def request_body(self, request_id, body, complete, observations=None):
        with self.lock:
            connection_id = request_id.rsplit("-r", 1)[0] if request_id else None
            phase_id = self.connections.get(connection_id, {}).get("phase_id")
            return self.append({"event": "request_body", "request_id": request_id, "phase_id": phase_id, "body_length": len(body), "body_sha256": hashlib.sha256(body).hexdigest(), "body_complete": bool(complete), "observations": observations or {}})

    def consume_fault(self, mode):
        with self.lock:
            if self.mode != mode or self.fault_used:
                return False
            self.fault_used = True
            return True

    def response_sent(self, request_id, status):
        with self.lock:
            row = self.append({"event": "response_sent", "request_id": request_id, "status": int(status)})
            self.active_requests.discard(request_id)
            return row

    def request_error(self, request_id, kind, detail):
        with self.lock:
            row = self.append({"event": "request_error", "request_id": request_id, "kind": kind, "detail": str(detail)})
            self.active_requests.discard(request_id)
            return row

    def parse_error(self, connection_id, detail):
        with self.lock:
            return self.append({"event": "parse_error", "connection_id": connection_id, "detail": str(detail), "phase_id": self.connections.get(connection_id, {}).get("phase_id")})

    def is_request_active(self, request_id):
        with self.lock:
            return request_id in self.active_requests

    def close_connection(self, connection_id, reason):
        with self.lock:
            if connection_id not in self.connections:
                return
            for request_id in [r for r in self.active_requests if r.startswith(connection_id + "-")]:
                self.request_error(request_id, "connection_close", reason)
            self.append({"event": "connection_close", "connection_id": connection_id, "reason": str(reason), "phase_id": self.connections[connection_id]["phase_id"]})
            del self.connections[connection_id]


RUN_ID = ""
ROLE = ""
RECORDER = None
CONNECTION_IDS = {}
CONNECTION_LOCK = threading.Lock()


class LedgerHTTPServer(http.server.ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = False

    def get_request(self):
        sock, peer = super().get_request()
        peer_text = f"[{peer[0]}]:{peer[1]}" if ":" in peer[0] else f"{peer[0]}:{peer[1]}"
        local = sock.getsockname()
        local_text = f"[{local[0]}]:{local[1]}" if isinstance(local, tuple) and ":" in local[0] else f"{local[0]}:{local[1]}"
        connection_id = RECORDER.accept(peer_text, local_text)
        with CONNECTION_LOCK:
            CONNECTION_IDS[sock.fileno()] = connection_id
        return sock, peer

    def close_request(self, request):
        with CONNECTION_LOCK:
            connection_id = CONNECTION_IDS.pop(request.fileno(), None)
        try:
            if connection_id:
                RECORDER.close_connection(connection_id, "server_close")
        finally:
            return super().close_request(request)


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server_version = "E0Synthetic/2"
    sys_version = ""

    def setup(self):
        super().setup()
        with CONNECTION_LOCK:
            self.connection_id = CONNECTION_IDS.get(self.connection.fileno())
        self.request_id = None
        self.connection.settimeout(10)

    def log_message(self, *_args):
        return

    def handle(self):
        try:
            return super().handle()
        except (socket.timeout, OSError, RecorderError) as exc:
            request_id = getattr(self, "request_id", None)
            if request_id and RECORDER.is_request_active(request_id):
                RECORDER.request_error(request_id, "socket_error", str(exc))
            elif self.connection_id:
                RECORDER.parse_error(self.connection_id, f"socket_error: {exc}")
            self.close_connection = True

    def parse_request(self):
        ok = super().parse_request()
        if not ok:
            return False
        if self.connection_id:
            self.request_id = RECORDER.request_start(
                self.connection_id, self.command, self.path, self.headers.get("X-E0-Case"))['request_id']
        return True

    def _body(self):
        if self.headers.get("Transfer-Encoding"):
            RECORDER.request_error(self.request_id, "unsupported_transfer_encoding", self.headers.get("Transfer-Encoding"))
            raise RecorderError("chunked request bodies are not accepted by the evidence recorder")
        try:
            raw_size = self.headers.get("Content-Length", "0")
            size = int(raw_size)
        except (TypeError, ValueError):
            RECORDER.request_error(self.request_id, "invalid_content_length", raw_size)
            raise RecorderError("invalid Content-Length")
        if size < 0 or size > 2_000_000:
            RECORDER.request_body(self.request_id, b"", False, {"content_length": size, "rejected": True})
            RECORDER.request_error(self.request_id, "body_limit", f"Content-Length {size} exceeds 2000000")
            raise RecorderError("request body is invalid or exceeds 2000000 bytes")
        body = self.rfile.read(size) if size else b""
        complete = len(body) == size
        if not complete:
            RECORDER.request_body(self.request_id, body, False, {"content_length": size})
            RECORDER.request_error(self.request_id, "incomplete_body", f"read {len(body)} of {size} bytes")
            raise RecorderError("request body ended before Content-Length")
        return body

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
        observations = {
            "purpose": purpose,
            "model": payload.get("model") if isinstance(payload, dict) else None,
            "stream": payload.get("stream") if isinstance(payload, dict) else None,
            "session_id": self.headers.get("x-session-id"),
            "session_affinity": self.headers.get("x-session-affinity"),
            "fault_mode": fault,
        }
        return RECORDER.request_body(self.request_id, body, True, observations)

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
        RECORDER.response_sent(self.request_id, status)

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
        RECORDER.response_sent(self.request_id, 200)

    def _handle(self):
        body = self._body()
        payload = {}
        try:
            payload = json.loads(body or b"{}")
        except Exception:
            pass
        purpose = self._purpose(payload)
        if ROLE == "adapter":
            fault = RECORDER.mode
            mode = {"mode": fault, "location": RECORDER.redirect_location}
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
            if fault == "fault503" and purpose == "task" and RECORDER.consume_fault(fault):
                self._record(body, 503, False, fault, purpose, payload)
                self._json(503, {"error": {"message": "synthetic 503", "type": "server_error"}})
                return
            if fault == "disconnect" and purpose == "task" and RECORDER.consume_fault(fault):
                self._record(body, None, False, fault, purpose, payload)
                RECORDER.request_error(self.request_id, "injected_disconnect", "connection closed before response")
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
            RECORDER.response_sent(self.request_id, 200)
            return
        self._record(body, 200, True, "sink", purpose, payload)
        self._json(200, {"ok": True, "role": {8080: "gateway", 8081: "provider", 8082: "proxy", 8083: "host"}.get(port, "direct"), "method": self.command, "path": self.path})

    do_GET = do_POST = do_PUT = do_DELETE = do_OPTIONS = do_PATCH = do_HEAD = _handle
    do_CONNECT = _handle

    def handle_expect_100(self):
        self.send_error(417, "Expect: 100-continue is unsupported")
        return False

    def send_error(self, code, message=None, explain=None):
        if self.connection_id:
            RECORDER.parse_error(self.connection_id, f"{code}: {message or explain or ''}")
        self.close_connection = True


def main():
    global RUN_ID, ROLE, RECORDER
    p = argparse.ArgumentParser()
    p.add_argument("--role", choices=("adapter", "direct"), required=True)
    p.add_argument("--run-id", required=True)
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--control-socket", required=True)
    a = p.parse_args()
    RUN_ID, ROLE = a.run_id, a.role
    ports = [8080, 8081, 8082, 8083] if ROLE == "direct" else [8080]
    RECORDER = SourceRecorder(RUN_ID, ROLE, ports, a.evidence_dir, a.control_socket)
    RECORDER.start()
    servers = []
    try:
        for port in ports:
            server = LedgerHTTPServer(("0.0.0.0", port), Handler)
            thread = threading.Thread(target=server.serve_forever, name=f"{ROLE}-{port}", daemon=True)
            thread.start()
            servers.append((server, thread))
        RECORDER.on_seal = lambda: [server.shutdown() for server, _ in servers]
        stopped = threading.Event()
        import signal
        signal.signal(signal.SIGTERM, lambda *_: stopped.set())
        signal.signal(signal.SIGINT, lambda *_: stopped.set())
        while not stopped.wait(0.25):
            if RECORDER.fatal_error is not None:
                raise RecorderError(RECORDER.fatal_error)
    finally:
        for server, _ in servers:
            server.shutdown()
            server.server_close()
        RECORDER.stop_control()


if __name__ == "__main__":
    main()
