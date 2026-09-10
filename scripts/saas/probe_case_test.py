import unittest

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

    def test_unknown_capture_rejected(self):
        case = {"id": "x", "operation_id": "get", "request": {"body": {"id": "${capture:nope}"}}, "expected": {}}
        with self.assertRaises(ValueError):
            validate_case(case, "saas-x", True)

    def test_json_pointer(self):
        self.assertEqual(json_pointer({"items": [{"id": "g1"}]}, "/items/0/id"), "g1")


if __name__ == "__main__":
    unittest.main()
