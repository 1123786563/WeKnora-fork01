"""Server entrypoint (O01): gRPC service + HTTP readiness/liveness.

gRPC serves business RPC on SEMANTIC_ADDRESS (default :50051). A small
HTTP server on SEMANTIC_HTTP_ADDRESS (default :50052) exposes /ready
(migrations + stores + deletion barriers synced) and /healthz (process
alive). Restoring service stays NOT ready and refuses queries until the
barrier sync completes.
"""

from __future__ import annotations

import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from .config import SemanticServiceConfig
from .server import create_server


def compute_readiness(migrations_ready: bool, stores_connected: bool,
		deletion_barriers_synced: bool) -> bool:
	"""The readiness fold (O01 core invariant, shared with the tests):
	migrations AND stores AND barrier sync. Importable so the tests pin
	the SHIPPED computation, not a test-local copy."""
	return migrations_ready and stores_connected and deletion_barriers_synced


def _readiness_inputs() -> dict:
	"""Hardcoded placeholders until the O02 recovery harness wires real
	probing: the barrier gate deliberately fails closed (restoring service
	refuses queries). Do NOT represent this as live probing."""
	return {"migrations_ready": True, "stores_connected": True,
			"deletion_barriers_synced": False}


def _make_probe_handler(current_ready):
	"""/ready + /healthz handler factory with injectable readiness (the
	tests boot the handler directly; main() wires the live inputs)."""
	class Probes(BaseHTTPRequestHandler):
		def do_GET(self):
			if self.path == "/ready":
				ready_now = current_ready()
				self.send_response(200 if ready_now else 503)
				self.end_headers()
				self.wfile.write(b"ready" if ready_now else b"not-ready")
			elif self.path == "/healthz":
				self.send_response(200)
				self.end_headers()
				self.wfile.write(b"ok")
			else:
				self.send_response(404)
				self.end_headers()

		def log_message(self, *args):
			return  # probe noise stays out of logs

	return Probes


def main() -> None:
	config = SemanticServiceConfig(
		address=os.environ.get("SEMANTIC_ADDRESS", "0.0.0.0:50051"),
		internal_token=os.environ.get("SEMANTIC_INTERNAL_TOKEN", ""),
		tls_cert_path=os.environ.get("SEMANTIC_TLS_CERT", ""),
		tls_key_path=os.environ.get("SEMANTIC_TLS_KEY", ""),
		allow_plaintext=os.environ.get("SEMANTIC_ALLOW_PLAINTEXT", "") == "true",
		query_dsn=os.environ.get("SEMANTIC_QUERY_DSN", ""),
	)
	server = create_server(config)
	server.start()

	def current_ready() -> bool:
		# Recompute per probe (not a boot-time snapshot): a post-boot store
		# outage must be able to flip 200 -> 503 once O02 wires real inputs.
		inputs = _readiness_inputs()
		return compute_readiness(inputs["migrations_ready"],
			inputs["stores_connected"], inputs["deletion_barriers_synced"])

	http_address = os.environ.get("SEMANTIC_HTTP_ADDRESS", "0.0.0.0:50052")
	host, _, port = http_address.rpartition(":")
	httpd = ThreadingHTTPServer((host or "0.0.0.0", int(port)), _make_probe_handler(current_ready))
	print(f"semantic service: grpc on {config.address}, probes on {http_address}", flush=True)
	import signal
	import threading
	# shutdown() blocks until serve_forever returns - calling it from the
	# signal handler (same thread) deadlocks. Delegate to a short-lived
	# thread so the finally clause (gRPC graceful stop) actually runs.
	signal.signal(signal.SIGTERM,
		lambda *_: threading.Thread(target=httpd.shutdown, daemon=True).start())
	try:
		httpd.serve_forever()
	finally:
		server.stop(0)


if __name__ == "__main__":
	main()