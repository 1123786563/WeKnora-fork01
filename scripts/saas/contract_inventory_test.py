import unittest

from scripts.saas.contract_inventory import operation_paths


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


if __name__ == "__main__":
    unittest.main()
