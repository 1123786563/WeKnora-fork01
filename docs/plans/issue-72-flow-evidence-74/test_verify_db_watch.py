#!/usr/bin/env python3
"""Offline regression tests for verify_db_watch.py (issue-72 ocr round 1).

The verifier reads a git-ignored runtime artifact (runs/db-watch-verify-*.tsv
under deploy/lago-lab/payment-activation) plus a sibling evidence JSON. Each
test rebuilds that layout under a temp dir — keeping the script's relative
path depth — and executes a copy of the script in-process (runpy, exactly
like ``python3 verify_db_watch.py``), asserting the exit-code contract:
0 PASS, 1 CHECK (assertion failed), 2 MISSING-EVIDENCE (no observer TSV:
fresh clone / cleaned runs/).
"""
import contextlib
import io
import json
import runpy
import shutil
import tempfile
import unittest
from pathlib import Path

EVID = Path(__file__).resolve().parent
SCRIPT = EVID / "verify_db_watch.py"
BOUNDARY = "2026-09-23T06:27:38"  # t02-decline.json recorded_at (truncated)

# Observer TSV rows (ts \t subs[] \t payments[] \t rows[]), all strictly
# before the mid-run boundary so they count as commercial-window samples.
GOOD_TSV = [
    ("2026-09-23T06:16:56Z",
     "subs[4|1]", "payments[failed|1;pending|1]",
     "rows[weknora-t02-run-sub-a|4]"),
    ("2026-09-23T06:22:16Z",
     "subs[4|1]", "payments[failed|1;requires_action|1;succeeded|1]",
     "rows[weknora-t02-run-sub-a|4;weknora-t02-run-sub-b|1;weknora-t02-run-sub-c|4]"),
]
# Tag drift / unsampled sub-a: only -b and -c rows exist. The A assertion
# ("seen staying incomplete") must not pass vacuously on the empty set.
NO_SUB_A_TSV = [
    ("2026-09-23T06:22:16Z",
     "subs[1|1]", "payments[succeeded|1]",
     "rows[weknora-t02-run-sub-b|1;weknora-t02-run-sub-c|4]"),
]


def tsv_text(rows):
    return "".join("\t".join(row) + "\n" for row in rows)


class layout:
    """Temp dir mirroring the script's relative-path layout."""

    def __enter__(self):
        self._tmp = tempfile.TemporaryDirectory()
        root = Path(self._tmp.name)
        self.evd = root / "docs/plans/issue-72-flow-evidence-74"
        self.evd.mkdir(parents=True)
        shutil.copy(SCRIPT, self.evd / "verify_db_watch.py")
        (self.evd / "t02-decline.json").write_text(
            json.dumps({"recorded_at": BOUNDARY + "Z"}), encoding="utf-8")
        self.runs = root / "deploy/lago-lab/payment-activation/runs"
        self.runs.mkdir(parents=True)
        return self

    def write_tsv(self, name, text):
        (self.runs / name).write_text(text, encoding="utf-8")

    def run(self):
        """Execute the copied script like a CLI call; returns (exit_code, stdout)."""
        stdout = io.StringIO()
        code = 0
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stdout):
            try:
                runpy.run_path(str(self.evd / "verify_db_watch.py"),
                               run_name="__main__")
            except SystemExit as exit_request:
                code = exit_request.code if isinstance(exit_request.code, int) else 0
        return code, stdout.getvalue()

    def __exit__(self, *exc):
        self._tmp.cleanup()
        return False


class TestVerifyDbWatch(unittest.TestCase):
    def test_missing_tsv_exits_2_with_actionable_message(self):
        # runs/ is git-ignored: a fresh clone or a cleaned checkout has no
        # observer TSV. That is missing evidence (exit 2), never an
        # IndexError crash and never a PASS.
        with layout() as env:
            code, out = env.run()
        self.assertEqual(code, 2, out)
        self.assertIn("no observer TSV", out)
        self.assertIn("git-ignored", out)

    def test_missing_sub_a_samples_is_check_not_vacuous_pass(self):
        with layout() as env:
            env.write_tsv("db-watch-verify-20260923T061349Z.tsv",
                          tsv_text(NO_SUB_A_TSV))
            code, out = env.run()
        self.assertEqual(code, 1, out)
        self.assertIn("DB-WATCH: CHECK", out)

    def test_good_tsv_still_passes(self):
        with layout() as env:
            env.write_tsv("db-watch-verify-20260923T061349Z.tsv",
                          tsv_text(GOOD_TSV))
            code, out = env.run()
        self.assertEqual(code, 0, out)
        self.assertIn("DB-WATCH: PASS", out)
        self.assertIn("sub-a states: [4]", out)


if __name__ == "__main__":
    unittest.main()
