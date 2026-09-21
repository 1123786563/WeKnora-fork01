#!/usr/bin/env python3
"""R476 A1: minimal OpenAI-compatible mock LLM for WeKnora parity tests.

Daemonized successor of R475's session-scoped /tmp/r475-mock-llm.py.
No credentials, keys, or secrets in this file (verified) — it only echoes
deterministic content.

Endpoints:
  GET  /v1/models
  POST /v1/chat/completions  (stream & non-stream)
  POST /v1/embeddings

Binds 0.0.0.0:18090, reachable as 127.0.0.1 and via LAN IP 192.168.3.30
(this machine). The LAN address is required because the Go backend dials the
LLM through the SSRF whitelist (SSRF_WHITELIST_EXTRA=127.0.0.1,192.168.3.30,...
in the gitignored .env.local; R475 A2 configuration).

Run via the daemon controller (nohup + pidfile under /tmp):
  pnpm dev:mock-llm          # start
  pnpm dev:mock-llm:status   # status
  pnpm dev:mock-llm:stop     # stop
or directly: bash scripts/dev-mock-llm.sh {start|stop|restart|status}
"""
import json
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MOCK_MODEL = "mock-model"
FIXED_CONTENT = "This is a deterministic mock summary for R475 parity testing. The document describes WeKnora UI alignment verification steps and mock LLM environment setup."
# R477 A3: question-generation prompts (config/prompt_templates/generate_questions.yaml)
# are answered with one numbered question per line so the backend's
# line-splitting parser (knowledge_process.go generateQuestionsWithContext)
# materialises question_count questions per chunk.
QUESTION_LINES = (
    "1. What environment setup is required for WeKnora question generation verification?\n"
    "2. How does the WeKnora backend gate automatic question generation for a knowledge base?\n"
    "3. What assertions confirm generated questions were persisted for a document chunk?"
)
EMBED_DIM = 8


def _content_for(body):
    """Wiki extraction prompts demand {"entities":[],"concepts":[]} JSON;
    question-generation prompts get one question per line;
    everything else gets the fixed prose summary."""
    try:
        msgs = body.get("messages") or []
        text = " ".join(str(m.get("content", "")) for m in msgs)
    except Exception:
        text = ""
    if '"entities"' in text and '"concepts"' in text:
        return '{"entities": [], "concepts": []}'
    if "<main_content>" in text:
        return QUESTION_LINES
    return FIXED_CONTENT


def now():
    return int(time.time())


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):
        print("[mock-llm] %s - %s" % (self.address_string(), fmt % args), flush=True)

    def _read_body(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b"{}"
        try:
            return json.loads(raw or b"{}")
        except Exception:
            return {}

    def _send_json(self, obj, code=200):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.rstrip("/").endswith("/models"):
            self._send_json({
                "object": "list",
                "data": [{"id": MOCK_MODEL, "object": "model", "created": now(), "owned_by": "mock"}],
            })
        else:
            self._send_json({"error": {"message": "not found", "type": "invalid_request_error"}}, 404)

    def do_POST(self):
        path = self.path.rstrip("/")
        body = self._read_body()
        if path.endswith("/chat/completions"):
            if body.get("stream"):
                self._stream_chat(body)
            else:
                self._chat(body)
        elif path.endswith("/embeddings"):
            n = len(body.get("input") if isinstance(body.get("input"), list) else [body.get("input")])
            self._send_json({
                "object": "list",
                "data": [
                    {"object": "embedding", "index": i,
                     "embedding": [0.01 * ((i + j) % 7 + 1) for j in range(EMBED_DIM)]}
                    for i in range(max(n, 1))
                ],
                "model": body.get("model", MOCK_MODEL),
                "usage": {"prompt_tokens": 1, "total_tokens": 1},
            })
        else:
            self._send_json({"error": {"message": "not found", "type": "invalid_request_error"}}, 404)

    def _chat(self, body):
        self._send_json({
            "id": "chatcmpl-" + uuid.uuid4().hex[:12],
            "object": "chat.completion",
            "created": now(),
            "model": body.get("model", MOCK_MODEL),
            "choices": [{
                "index": 0,
                "message": {"role": "assistant", "content": _content_for(body)},
                "finish_reason": "stop",
            }],
            "usage": {"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
        })

    def _stream_chat(self, body):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Transfer-Encoding", "chunked")
        self.end_headers()
        cid = "chatcmpl-" + uuid.uuid4().hex[:12]

        def chunk(delta, finish=None):
            return {
                "id": cid, "object": "chat.completion.chunk", "created": now(),
                "model": body.get("model", MOCK_MODEL),
                "choices": [{"index": 0, "delta": delta, "finish_reason": finish}],
            }

        events = [chunk({"role": "assistant", "content": ""}),
                  chunk({"content": _content_for(body)})]
        # SMOKE PATCH (SP12, temporary): honor stream_options.include_usage like
        # real OpenAI-compatible providers so terminal usage lands in the turn.
        so = body.get("stream_options") or {}
        if so.get("include_usage"):
            usage_chunk = {
                "id": cid, "object": "chat.completion.chunk", "created": now(),
                "model": body.get("model", MOCK_MODEL),
                "choices": [],
                "usage": {"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
            }
            events.append(chunk({}))
            events = events + [usage_chunk]
        else:
            events.append(chunk({}, finish="stop"))
        try:
            for ev in events:
                data = ("data: " + json.dumps(ev) + "\n\n").encode()
                self.wfile.write(("%x\r\n" % len(data)).encode() + data + b"\r\n")
                self.wfile.flush()
            tail = b"data: [DONE]\n\n"
            self.wfile.write(("%x\r\n" % len(tail)).encode() + tail + b"\r\n")
            self.wfile.write(b"0\r\n\r\n")
            self.wfile.flush()
        except Exception:
            pass


if __name__ == "__main__":
    srv = ThreadingHTTPServer(("0.0.0.0", 18090), Handler)
    print("[mock-llm] listening on http://0.0.0.0:18090/v1", flush=True)
    srv.serve_forever()
