#!/usr/bin/env python3
"""Offline regression tests for verify_db_watch.py (issue-72 ocr rounds 1+2).

The verifier reads an archived observer TSV (verify-db-watch-samples.tsv
next to the script) or, absent that, the newest runs/db-watch-verify-*.tsv
(git-ignored runtime artifact), plus a sibling evidence JSON. Each test
rebuilds that layout under a temp dir — keeping the script's relative path
depth — and executes a copy of the script in-process (runpy, exactly like
``python3 verify_db_watch.py``), asserting the exit-code contract:
0 PASS, 1 CHECK (assertion failed), 2 MISSING-EVIDENCE (no usable TSV).

ocr-2 regression scope: the archived copy takes priority over any runs/
TSV (a foreign replay's TSV must never silently pair with this directory's
decline boundary), the selected file is printed, and rows are filtered by
this run's weknora-t02-<run_id>- prefix so foreign-run samples cannot leak
into the sub-a/b/c assertions.
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
RUN_ID = "run"                    # t02-decline.json run_id (test sentinel)

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
# A multi-run TSV: this run's rows are clean, a foreign run's sub-a sits in
# a terminal state (3=canceled). Foreign rows must not leak into the
# assertions (ocr-2 run-prefix tightening).
FOREIGN_MIX_TSV = [
    ("2026-09-23T06:16:56Z",
     "subs[3|1;4|1]", "payments[failed|1;pending|1]",
     "rows[weknora-t02-other-run-sub-a|3;weknora-t02-run-sub-a|4]"),
    ("2026-09-23T06:22:16Z",
     "subs[3|1;1|1;4|1]", "payments[failed|1;requires_action|1;succeeded|1]",
     "rows[weknora-t02-other-run-sub-a|3;weknora-t02-run-sub-a|4;"
     "weknora-t02-run-sub-b|1;weknora-t02-run-sub-c|4]"),
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
            json.dumps({"run_id": RUN_ID, "recorded_at": BOUNDARY + "Z"}),
            encoding="utf-8")
        self.runs = root / "deploy/lago-lab/payment-activation/runs"
        self.runs.mkdir(parents=True)
        return self

    def write_tsv(self, name, text):
        (self.runs / name).write_text(text, encoding="utf-8")

    def write_archive(self, text):
        (self.evd / "verify-db-watch-samples.tsv").write_text(text, encoding="utf-8")

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
        # No archive AND no runs/ TSV: missing evidence (exit 2), never an
        # IndexError crash and never a PASS.
        with layout() as env:
            code, out = env.run()
        self.assertEqual(code, 2, out)
        self.assertIn("no observer TSV", out)
        self.assertIn("git-ignored", out)

    def test_runs_fallback_degrades_to_missing_evidence(self):
        # (R1-07) The runs/-fallback guard used to be constant-false
        # (`archived.exists() and path != str(archived)` can never hold: path
        # is only assigned inside the archived.exists() branch), so the
        # declared MISSING-EVIDENCE degradation never fired and a foreign
        # runs/ TSV was silently asserted on. Now a runs/ TSV with no
        # archive degrades loudly to exit 2: the payments aggregate cannot
        # be keyed to this run, so the exactly-once assertion is not
        # decidable — even when the TSV itself would have passed.
        with layout() as env:
            env.write_tsv("db-watch-verify-20260923T061349Z.tsv",
                          tsv_text(GOOD_TSV))
            code, out = env.run()
        self.assertEqual(code, 2, out)
        self.assertIn("WARNING", out)
        self.assertIn("not decidable", out)

    def test_missing_sub_a_samples_is_check_not_vacuous_pass(self):
        # Same assertion as before, now anchored on the ARCHIVED TSV (the
        # only decidable source): a missing sub-a sample set is a CHECK
        # (exit 1), never a vacuous pass.
        with layout() as env:
            env.write_archive(tsv_text(NO_SUB_A_TSV))
            code, out = env.run()
        self.assertEqual(code, 1, out)
        self.assertIn("DB-WATCH: CHECK", out)

    def test_good_tsv_still_passes_and_prints_the_selected_file(self):
        # (R1-07) The PASS path is the ARCHIVED TSV; the runs/ fallback now
        # degrades to exit 2 (see test_runs_fallback_degrades_to_missing_evidence).
        with layout() as env:
            env.write_archive(tsv_text(GOOD_TSV))
            code, out = env.run()
        self.assertEqual(code, 0, out)
        self.assertIn("DB-WATCH: PASS", out)
        self.assertIn("sub-a states: [4]", out)
        self.assertIn("DB-WATCH: using ", out)
        self.assertIn("verify-db-watch-samples.tsv", out)

    def test_archived_tsv_takes_priority_over_runs_and_is_printed(self):
        # ocr-2: this directory's archive must win over whatever replay TSV
        # currently sits in runs/ (a foreign run's samples against this
        # boundary would produce an untraceable CHECK/PASS).
        with layout() as env:
            env.write_tsv("db-watch-verify-20261231T235959Z.tsv",
                          tsv_text(NO_SUB_A_TSV))  # would CHECK if used
            env.write_archive(tsv_text(GOOD_TSV))
            code, out = env.run()
        self.assertEqual(code, 0, out)
        self.assertIn("DB-WATCH: PASS", out)
        selected = [ln for ln in out.splitlines() if "using" in ln][0]
        self.assertIn("verify-db-watch-samples.tsv", selected)
        self.assertNotIn("db-watch-verify-20261231", selected)

    def test_foreign_run_rows_do_not_leak_into_assertions(self):
        # ocr-2: a multi-run TSV mixes a foreign run's terminal sub-a with
        # this run's clean rows; only weknora-t02-<this run_id>- prefixed
        # rows may feed the assertions. (R1-07) anchored on the archive —
        # the runs/ fallback itself now degrades to exit 2.
        with layout() as env:
            env.write_archive(tsv_text(FOREIGN_MIX_TSV))
            code, out = env.run()
        self.assertEqual(code, 0, out)
        self.assertIn("DB-WATCH: PASS", out)
        self.assertIn("sub-a states: [4]", out)


if __name__ == "__main__":
    unittest.main()
