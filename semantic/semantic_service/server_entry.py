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

	class Probes(BaseHTTPRequestHandler):
		def do_GET(self):
			if self.path == "/ready":
				code = 200 if ready else 503
				self.send_response(code)
				self.end_headers()
				self.wfile.write(b"ready" if current_ready() else b"not-ready")
			elif self.path == "/healthz":
				self.send_response(200)
				self.end_headers()
				self.wfile.write(b"ok")
			else:
				self.send_response(404)
				self.end_headers()

		def log_message(self, *args):
			return  # probe noise stays out of logs

	http_address = os.environ.get("SEMANTIC_HTTP_ADDRESS", "0.0.0.0:50052")
	host, _, port = http_address.rpartition(":")
	httpd = ThreadingHTTPServer((host or "0.0.0.0", int(port)), Probes)
	print(f"semantic service: grpc on {config.address}, probes on {http_address}", flush=True)
	import signal
	signal.signal(signal.SIGTERM, lambda *_: httpd.shutdown())
	try:
		httpd.serve_forever()
	finally:
		server.stop(0)


if __name__ == "__main__":
	main()