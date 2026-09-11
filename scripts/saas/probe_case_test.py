import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from scripts.saas.probe_case import assert_subset, json_pointer, validate_case


class ProbeTests(unittest.TestCase):
    def test_http_success_cannot_hide_wrong_balance(self):
        with self.assertRaises(AssertionError):
            assert_subset({"balance": "99"}, {"balance": "100"})
        assert_subset({"balance": "100", "id": "g1"}, {"balance": "100"})

    def test_nested_balance_mismatch_fails(self):
        with self.assertRaises(AssertionError):
            assert_subset({"balances": {"credit": {"available": 9}}}, {"balances": {"credit": {"available": 10}}})

    def test_writes_require_flag_and_namespace(self):
        case = {"id": "x", "operation_id": "create", "request": {"method": "POST"}, "expected": {}}
        with self.assertRaises(PermissionError):
            validate_case(case, "saas-x", False)
        case["request"]["namespace"] = "other"
        with self.assertRaises(ValueError):
            validate_case(case, "saas-x", True)
        case["request"]["path"] = "/namespaces/saas-x/customers"
        case["request"]["replay"] = True
        case["request"]["idempotency_key"] = "k"
        case["request"]["replay_identity"] = "id"
        with self.assertRaises(ValueError):
            validate_case(case, "saas-x", True)

    def test_same_namespace_body_and_cleanup_guards(self):
        case = {"id": "x", "operation_id": "create", "captures": {"id": "/id"},
                "request": {"method": "POST", "body": {"namespace": "saas-x"}},
                "cleanup": [{"capture": "id", "path": "/customers/${capture:id}"}], "expected": {}}
        validate_case(case, "saas-x", True)
        with self.assertRaises(PermissionError):
            validate_case(case, "saas-x", False)

    def test_repeatable_write_requires_idempotency_key(self):
        case = {"id": "x", "operation_id": "create", "request": {"method": "POST", "repeatable": True}, "expected": {}}
        with self.assertRaises(ValueError):
            validate_case(case, "saas-x", True)

    def test_replay_requires_identity_pointer(self):
        case = {"id": "x", "operation_id": "create", "request": {"method": "POST", "replay": True, "idempotency_key": "k"}, "expected": {}}
        with self.assertRaises(ValueError):
            validate_case(case, "saas-x", True)

    def test_path_namespace_and_identity_pointer_are_validated(self):
        case = {"id": "x", "operation_id": "get", "request": {"path": "/namespaces/other/customers"}, "expected": {}}
        with self.assertRaises(ValueError):
            validate_case(case, "saas-x", True)

    def test_captured_path_namespace_is_checked_after_binding(self):
        from scripts.saas.probe_case import _bind, _validate_path_namespace
        path = _bind("/namespaces/${capture:ns}/customers", {"ns": "other"})
        with self.assertRaises(ValueError):
            _validate_path_namespace(path, "saas-x")

    def test_cleanup_requires_capture_created_by_write_step(self):
        from scripts.saas.probe_case import _validate_cleanup_capture
        with self.assertRaises(ValueError):
            _validate_cleanup_capture("customer_id", set())
        _validate_cleanup_capture("customer_id", {"customer_id"})

    def test_get_capture_cannot_be_cleaned_up(self):
        from scripts.saas.probe_case import run_case
        case = {
            "id": "read-then-clean",
            "operation_id": "read",
            "request": {"method": "GET", "path": "/customers/c1", "captures": {"customer_id": "/id"}},
            "cleanup": [{"capture": "customer_id", "path": "/customers/${capture:customer_id}"}],
            "expected": {"id": "c1"},
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            schema = root / "schema.json"
            schema.write_text(json.dumps({"operations": [{"operation_id": "read", "method": "GET", "local_path": "/customers/{id}"}]}))
            with patch("scripts.saas.probe_case._request_json", return_value=(200, {"id": "c1"})) as request:
                with self.assertRaises(ValueError):
                    run_case(case, "http://openmeter", "saas-x", True, schema, root / "artifacts")
            request.assert_called_once()

    def test_unknown_capture_rejected(self):
        case = {"id": "x", "operation_id": "get", "request": {"body": {"id": "${capture:nope}"}}, "expected": {}}
        with self.assertRaises(ValueError):
            validate_case(case, "saas-x", True)

    def test_json_pointer(self):
        self.assertEqual(json_pointer({"items": [{"id": "g1"}]}, "/items/0/id"), "g1")


if __name__ == "__main__":
    unittest.main()
