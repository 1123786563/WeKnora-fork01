"""Unit tests for the #104 restore-drill pure logic (no docker, no stack).

Run: python3 -m pytest deploy/lago/restore_drill_test.py (or unittest).
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import restore_drill as drill  # noqa: E402


class CompareClassesTest(unittest.TestCase):
    def test_equal_snapshots_pass(self):
        snap = {"customer": [{"lago_id": "a"}], "invoice": [{"lago_id": "b"}]}
        verdict = drill.compare_classes(snap, {"customer": [{"lago_id": "a"}], "invoice": [{"lago_id": "b"}]})
        self.assertTrue(all(v["equal"] for v in verdict.values()))
        self.assertEqual(verdict["customer"]["pre"], 1)

    def test_drifted_object_fails(self):
        pre = {"wallet": [{"lago_id": "w", "balance": "5.0"}]}
        post = {"wallet": [{"lago_id": "w", "balance": "3.0"}]}
        self.assertFalse(drill.compare_classes(pre, post)["wallet"]["equal"])

    def test_missing_class_after_restore_fails(self):
        pre = {"payment": [{"lago_id": "p"}]}
        post: dict = {}
        verdict = drill.compare_classes(pre, post)
        self.assertFalse(verdict["payment"]["equal"])
        self.assertEqual(verdict["payment"]["post"], 0)

    def test_class_coverage_is_seven(self):
        # AC1: the drill must reconcile seven classes.
        authority = {cls: [] for cls, _ in drill.CLASS_RESOURCES}
        classes = set(authority) | {"pending_work", "billing_projections"}
        self.assertEqual(len(classes), 7)


class FingerprintTest(unittest.TestCase):
    def test_picks_stable_fields_only(self):
        doc = {
            "lago_id": "inv-1",
            "external_id": "sub-ext",
            "status": "finalized",
            "fees_amount_cents": 1200,
            "updated_at": "2026-10-05T00:00:00Z",  # not fingerprinted
        }
        fp = drill.fingerprint("invoice", doc)
        self.assertEqual(fp, {"lago_id": "inv-1", "lago_customer_id": None, "status": "finalized", "currency": None, "fees_amount_cents": 1200})
        self.assertNotIn("updated_at", fp)

    def test_unknown_field_values_are_kept_verbatim(self):
        self.assertEqual(drill.fingerprint("payment", {}), {"lago_id": None, "amount_cents": None, "currency": None, "status": None})


class BackupDirTest(unittest.TestCase):
    def test_parses_reported_dir(self):
        out = "lago.sh: restore will DESTROY...\nbackup written to /tmp/x/backups/20261005T010203Z\n"
        self.assertEqual(drill.backup_dir_of(out), "/tmp/x/backups/20261005T010203Z")

    def test_missing_line_raises(self):
        with self.assertRaises(SystemExit):
            drill.backup_dir_of("nothing")


class BudgetTest(unittest.TestCase):
    def test_rto_budget_is_sixty_minutes(self):
        self.assertEqual(drill.RTO_BUDGET_SECONDS, 3600)

    def test_rpo_budget_is_five_minutes(self):
        self.assertEqual(drill.RPO_BUDGET_SECONDS, 300)


if __name__ == "__main__":
    unittest.main()
