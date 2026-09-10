import csv
import json
import subprocess
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
import generate_react_migration_baseline as generator  # noqa: E402


class ReactMigrationBaselineGeneratorTest(unittest.TestCase):
    def read_csv(self, name: str) -> list[dict[str, str]]:
        with (ROOT / "docs/migrations/react" / name).open(newline="", encoding="utf-8") as handle:
            return list(csv.DictReader(handle))

    def test_swagger_inventory_and_registered_routes_are_classified(self) -> None:
        swagger = json.loads((ROOT / "docs/swagger.json").read_text(encoding="utf-8"))
        operations = [
            (method.upper(), generator.canonical_path(path))
            for path, item in swagger["paths"].items()
            for method in item
            if method.upper() in generator.HTTP_METHODS
        ]
        self.assertEqual(len(swagger["paths"]), 282)
        self.assertEqual(len(operations), 361)

        rows = self.read_csv("api-contract-matrix.csv")
        matrix_keys = {(row["method"], generator.canonical_path(row["path"])) for row in rows}
        self.assertTrue(all(key in matrix_keys for key in operations))
        registered = generator.registered_routes()
        self.assertTrue(registered)
        self.assertTrue(
            all(
                (route["method"], generator.canonical_path(route["path"])) in matrix_keys
                for route in registered
            )
        )
        self.assertEqual(
            sum(row["document_source"] == "swagger-2.0" for row in rows),
            361,
        )
        self.assertTrue(any(row["comparison_status"] == "swagger-only" for row in rows))
        self.assertTrue(any(row["comparison_status"] == "implementation-only" for row in rows))

    def test_source_status_and_special_routes_are_explicit(self) -> None:
        rows = self.read_csv("api-contract-matrix.csv")
        self.assertTrue(rows)
        self.assertTrue(
            {row["source_status"] for row in rows}
            <= generator.SOURCE_STATUSES
        )
        special_paths = {
            "/files",
            "/r/:token",
            "/files/presigned",
            "/sessions/:id/sandbox/terminal",
            "/embed/:channel_id/files",
        }
        actual = {
            row["path"]
            for row in rows
            if row["source_status"] == "special-route"
        }
        self.assertTrue(special_paths <= actual)
        self.assertTrue(
            all(
                row["source_status"] != "swagger"
                for row in rows
                if row["comparison_status"] != "swagger-only"
            )
        )

    def test_route_parity_has_complete_scope_and_special_entries(self) -> None:
        rows = self.read_csv("route-parity.csv")
        self.assertGreaterEqual(len(rows), 46)
        for row in rows:
            self.assertTrue(row["target_task"].strip(), row)
            self.assertTrue(row["identity"].strip(), row)
            self.assertTrue(row["capability"].strip(), row)
            self.assertIn(row["source_status"], generator.SOURCE_STATUSES)
        entries = {row["route_or_entry"] for row in rows}
        self.assertIn("/files", entries)
        self.assertIn("/api/v1/sessions/:id/sandbox/terminal", entries)

    def test_generation_is_byte_for_byte_deterministic(self) -> None:
        outputs = [
            ROOT / "docs/migrations/react/api-contract-matrix.csv",
            ROOT / "docs/migrations/react/route-parity.csv",
            ROOT / "docs/migrations/react/reuse-manifest.csv",
            ROOT / "docs/migrations/react/version-matrix.md",
            ROOT / "docs/migrations/react/runtime-baseline.md",
        ]
        snapshots = []
        for _ in range(2):
            subprocess.run(
                ["python3", "scripts/generate_react_migration_baseline.py"],
                cwd=ROOT,
                check=True,
            )
            snapshots.append([path.read_bytes() for path in outputs])
        self.assertEqual(snapshots[0], snapshots[1])


if __name__ == "__main__":
    unittest.main(verbosity=2)
