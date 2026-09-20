"""Contract tests for the isolated native GraphRAG experiment adapter."""

from __future__ import annotations

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from tempfile import TemporaryDirectory

from native_env import NativeEnvClient, NativeEnvConfig, OwnershipError


class NativeEnvConfigTest(unittest.TestCase):
    def test_config_rejects_non_loopback_runtime(self) -> None:
        with self.assertRaises(ValueError):
            NativeEnvConfig.from_mapping({"app_url": "http://10.0.0.8:18081"})

    def test_owned_compose_resources_are_unique_and_loopback_only(self) -> None:
        config = NativeEnvConfig.from_mapping({"run_id": "case-a", "app_url": "http://127.0.0.1:18081"})
        compose = config.compose_document()
        self.assertEqual(compose["name"], "semantica-v03-native-case-a")
        self.assertEqual(compose["services"]["postgres"]["ports"], ["127.0.0.1:15432:5432"])
        self.assertEqual(compose["services"]["neo4j"]["ports"], ["127.0.0.1:17687:7687", "127.0.0.1:17474:7474"])
        self.assertIn("apoc", compose["services"]["neo4j"]["environment"]["NEO4J_PLUGINS"])
        self.assertTrue(all(name.startswith("semantica-v03-native-case-a-") for name in compose["volumes"]))

    def test_cleanup_refuses_resources_outside_its_run_prefix(self) -> None:
        config = NativeEnvConfig.from_mapping({"run_id": "case-a"})
        with self.assertRaises(OwnershipError):
            config.assert_owned("WeKnora-postgres-dev")


class _Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.0"
    calls: list[tuple[str, dict]] = []

    def do_POST(self) -> None:  # noqa: N802
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        self.calls.append((self.path, body))
        if self.path == "/api/v1/knowledge-chat/s-1":
            payload = 'data: {"type":"reference","data":{"match_type":"graph","knowledge_id":"k-1","chunk_id":"c-2"}}\n\n'
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Content-Length", str(len(payload.encode())))
            self.end_headers()
            self.wfile.write(payload.encode())
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"success":true,"data":{"id":"s-1"}}')

    def log_message(self, *_: object) -> None:
        pass


class NativeEnvClientTest(unittest.TestCase):
    def test_initialization_payload_preserves_enabled_graph_extract(self) -> None:
        payload = NativeEnvClient.initialization_payload("model-1")
        self.assertEqual(payload["llmModelId"], "model-1")
        self.assertTrue(payload["nodeExtract"]["enabled"])
        self.assertEqual(payload["nodeExtract"]["relations"][0]["type"], "depends_on")

    def test_query_preserves_graph_reference_and_evaluator_fields(self) -> None:
        server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            client = NativeEnvClient(f"http://127.0.0.1:{server.server_port}", "test-token", timeout_seconds=2)
            result = client.query(
                {"case_id": "q-1", "document_revision": "fixture-v1", "question": "甲依赖什么？"},
                ["d-1", "d-2"],
                session_id="s-1",
            )
        finally:
            server.shutdown()
            thread.join()
            server.server_close()
        self.assertEqual(result["requested_mode"], "native")
        self.assertEqual(result["actual_mode"], "native")
        self.assertEqual(result["case_id"], "q-1")
        self.assertEqual(result["document_revision"], "fixture-v1")
        self.assertEqual(result["evidence_ids"], ["d-1", "d-2"])
        self.assertEqual(result["references"][0]["match_type"], "graph")
        self.assertGreaterEqual(result["latency_ms"], 0)
        self.assertEqual(_Handler.calls[-1][1]["knowledge_ids"], ["d-1", "d-2"])


if __name__ == "__main__":
    unittest.main()
