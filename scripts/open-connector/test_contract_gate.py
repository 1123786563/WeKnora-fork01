import json
import os
import subprocess
import sys
import tempfile
import unittest

from contract_gate import PINNED_SHA, REQUIRED, validate

HERE = os.path.dirname(os.path.abspath(__file__))
GATE = os.path.join(HERE, "contract_gate.py")


def full_case(case_id, artifact=None):
    if artifact is None:
        artifact = "fixtures/%s.json" % case_id
    return {"id": case_id, "kind": "runtime", "passed": True, "artifact": artifact}


def full_report():
    return {
        "sha": PINNED_SHA,
        "image_digest": "sha256:" + "a" * 64,
        "cases": [full_case(name) for name in sorted(REQUIRED)],
    }


class GateTest(unittest.TestCase):
    def test_documentation_is_not_runtime_evidence(self):
        errors = validate({"sha": PINNED_SHA,
                           "cases": [{"id": "cross_connection", "kind": "doc", "passed": True}]})
        self.assertTrue(errors)
        self.assertIn("cross_connection: missing runtime proof", errors)

    def test_missing_cases_fail(self):
        self.assertTrue(validate({"cases": []}))

    def test_unreviewed_sha_fails(self):
        report = full_report()
        report["sha"] = "0" * 40
        errors = validate(report)
        self.assertIn("unreviewed upstream SHA", errors)

    def test_missing_digest_fails(self):
        report = full_report()
        del report["image_digest"]
        errors = validate(report)
        self.assertIn("missing immutable image", errors)

    def test_digest_without_sha256_prefix_fails(self):
        report = full_report()
        report["image_digest"] = "latest"
        errors = validate(report)
        self.assertIn("missing immutable image", errors)

    def test_partial_case_set_fails(self):
        report = full_report()
        report["cases"] = report["cases"][:3]
        errors = validate(report)
        self.assertEqual(len(errors), len(REQUIRED) - 3)
        for error in errors:
            self.assertTrue(error.endswith(": missing runtime proof"), error)

    def test_runtime_case_not_passed_fails(self):
        report = full_report()
        report["cases"][0]["passed"] = False
        errors = validate(report)
        self.assertEqual(errors, [report["cases"][0]["id"] + ": missing runtime proof"])

    def test_runtime_case_without_artifact_fails(self):
        report = full_report()
        report["cases"][1]["artifact"] = ""
        errors = validate(report)
        self.assertEqual(errors, [report["cases"][1]["id"] + ": missing runtime proof"])

    def test_complete_runtime_report_passes(self):
        self.assertEqual(validate(full_report()), [])

    def test_non_dict_report_rejected(self):
        self.assertTrue(validate(["not", "a", "report"]))


class GateCliTest(unittest.TestCase):
    def run_gate(self, report):
        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as handle:
            if isinstance(report, str):
                handle.write(report)
            else:
                json.dump(report, handle)
            path = handle.name
        try:
            proc = subprocess.run(
                [sys.executable, GATE, path],
                capture_output=True, text=True)
            return proc
        finally:
            os.unlink(path)

    def test_cli_exit_0_on_complete_report(self):
        proc = self.run_gate(full_report())
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertIn("OK", proc.stdout)

    def test_cli_exit_2_on_missing_digest(self):
        report = full_report()
        del report["image_digest"]
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("missing immutable image", proc.stdout)

    def test_cli_exit_2_on_partial_case_set(self):
        report = full_report()
        report["cases"] = [c for c in report["cases"] if c["id"] != "key_replay"]
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("key_replay: missing runtime proof", proc.stdout)

    def test_cli_exit_2_on_invalid_json(self):
        proc = self.run_gate("{not json")
        self.assertEqual(proc.returncode, 2)
        self.assertTrue(proc.stderr or proc.stdout)

    def test_cli_exit_2_on_missing_file(self):
        proc = subprocess.run(
            [sys.executable, GATE, os.path.join(HERE, "does-not-exist.json")],
            capture_output=True, text=True)
        self.assertEqual(proc.returncode, 2)
        self.assertTrue(proc.stderr)


if __name__ == "__main__":
    unittest.main()
