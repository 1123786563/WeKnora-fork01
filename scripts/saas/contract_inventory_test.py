import unittest
import tempfile
from pathlib import Path
from unittest.mock import Mock, patch

from scripts.saas.contract_inventory import _probe, main, operation_paths


class InventoryTests(unittest.TestCase):
    def test_v3_prefix_is_not_dropped(self):
        schema = "paths:\n  /openmeter/customers:\n    post:\n      operationId: create-customer\n"
        self.assertEqual(
            operation_paths(schema, "/api/v3"),
            {"create-customer": "POST /api/v3/openmeter/customers"},
        )

    def test_missing_server_prefix_is_rejected(self):
        with self.assertRaises(ValueError):
            operation_paths("paths:\n  /customers:\n    get:\n      operationId: list\n", "")

    def test_duplicate_operation_id_is_rejected(self):
        schema = "paths:\n  /one:\n    get:\n      operationId: same\n  /two:\n    get:\n      operationId: same\n"
        with self.assertRaisesRegex(ValueError, "duplicate operationId"):
            operation_paths(schema, "/api/v1")

    def test_operations_after_paths_block_are_not_inventoried(self):
        schema = "paths:\n  /customers:\n    get:\n      operationId: list\ncomponents:\n  /webhooks:\n    post:\n      operationId: webhook\n"
        self.assertEqual(operation_paths(schema, "/api/v1"), {"list": "GET /api/v1/customers"})

    def test_connection_refusal_is_blocked_environment(self):
        with patch("scripts.saas.contract_inventory.urllib.request.build_opener") as build:
            build.return_value.open.side_effect = ConnectionRefusedError("refused")
            result = _probe("http://127.0.0.1:1/api/v1/customers")
        self.assertEqual(result, {"status": "blocked-env", "exception": "ConnectionRefusedError"})

    def test_probe_uses_get_only(self):
        response = Mock(status=200)
        response.__enter__ = Mock(return_value=response)
        response.__exit__ = Mock(return_value=False)
        with patch("scripts.saas.contract_inventory.urllib.request.build_opener") as build:
            build.return_value.open.return_value = response
            _probe("http://example.test/read-only")
            request = build.return_value.open.call_args.args[0]
        self.assertEqual(request.get_method(), "GET")

    def test_schema_hash_mismatch_returns_exit_two(self):
        with tempfile.TemporaryDirectory() as directory:
            schema = Path(directory) / "api" / "openapi.yaml"
            schema.parent.mkdir()
            schema.write_text("paths:\n", encoding="utf-8")
            output = Path(directory) / "result.json"
            self.assertEqual(main(["--schema", str(schema), "--prefix", "/api/v1", "--base-url", "http://127.0.0.1:1", "--output", str(output)]), 2)
            self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
