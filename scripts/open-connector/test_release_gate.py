#!/usr/bin/env python3
"""Tests for the T18 rollout release gate (release_gate.py).

The gate fails closed: a release report passes only when ALL seven evidence
classes (contract / integration / browser / provider_read / provider_write /
billing / recovery) carry runtime evidence (kind=runtime, passed=true,
non-empty artifact) AND the CLI can independently verify every artifact file
(exists, sha256 digest matches) plus the single release context (same commit,
same image digest, same test namespace). passed=true alone never qualifies.
"""

import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

from release_gate import CONTEXT_KEYS, REQUIRED, release_errors, verify_report

HERE = os.path.dirname(os.path.abspath(__file__))
GATE = os.path.join(HERE, "release_gate.py")

COMMIT = "219ffe67" + "0" * 32
IMAGE = "sha256:" + "ab" * 32
NAMESPACE = "oc-release-probe"


def sha256_file(path):
    with open(path, "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()


def runtime_entry(name, base, **overrides):
    """A truthful runtime entry: real artifact file, real digest, one context."""
    artifact_dir = os.path.join(base, "artifacts")
    os.makedirs(artifact_dir, exist_ok=True)
    artifact_rel = "artifacts/%s.json" % name
    path = os.path.join(artifact_dir, "%s.json" % name)
    payload = {"class": name, "kind": "runtime", "note": "probe evidence"}
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, sort_keys=True)
    entry = {
        "kind": "runtime",
        "passed": True,
        "artifact": artifact_rel,
        "sha256": sha256_file(path),
        "commit": COMMIT,
        "image_digest": IMAGE,
        "test_namespace": NAMESPACE,
    }
    entry.update(overrides)
    return entry


def full_report(base):
    report = {
        "commit": COMMIT,
        "image_digest": IMAGE,
        "test_namespace": NAMESPACE,
        "open_unknown_count": 0,
    }
    for name in REQUIRED:
        report[name] = runtime_entry(name, base)
    return report


class GateWorkspace(unittest.TestCase):
    """Base class providing a disposable evidence workspace per test."""

    def setUp(self):
        self.base = tempfile.mkdtemp(prefix="oc-release-gate-")
        os.makedirs(os.path.join(self.base, "artifacts"), exist_ok=True)
        self.report = full_report(self.base)

    def tearDown(self):
        shutil.rmtree(self.base, ignore_errors=True)


class ReleaseGateTest(GateWorkspace):
    # Plan-verbatim: mock write evidence NEVER qualifies the rollout gate.
    def test_mock_write_never_qualifies(self):
        self.assertTrue(release_errors({"provider_write": {"kind": "mock", "passed": True}}))

    def test_empty_report_requires_every_class(self):
        errors = release_errors({})
        for name in REQUIRED:
            self.assertIn(name + ": runtime evidence required", errors)

    def minimal_report(self):
        report = {name: {"kind": "runtime", "passed": True, "artifact": "x"}
                  for name in REQUIRED}
        report["open_unknown_count"] = 0
        return report

    def test_single_missing_class_fails(self):
        report = self.minimal_report()
        del report["billing"]
        errors = release_errors(report)
        self.assertEqual(errors, ["billing: runtime evidence required"])

    def test_documentation_kind_is_not_runtime_evidence(self):
        report = self.minimal_report()
        report["contract"] = {"kind": "doc", "passed": True, "artifact": "x"}
        errors = release_errors(report)
        self.assertEqual(errors, ["contract: runtime evidence required"])

    def test_passed_true_without_artifact_fails(self):
        report = self.minimal_report()
        report["recovery"]["artifact"] = ""
        errors = release_errors(report)
        self.assertEqual(errors, ["recovery: runtime evidence required"])

    def test_failed_class_fails(self):
        report = self.minimal_report()
        report["integration"]["passed"] = False
        errors = release_errors(report)
        self.assertEqual(errors, ["integration: runtime evidence required"])

    def test_open_unknown_count_must_be_present_and_zero(self):
        report = {name: {"kind": "runtime", "passed": True, "artifact": "x"}
                  for name in REQUIRED}
        self.assertIn("unresolved executions in release test namespace",
                      release_errors(report))
        report["open_unknown_count"] = 2
        self.assertIn("unresolved executions in release test namespace",
                      release_errors(report))
        report["open_unknown_count"] = 0
        self.assertEqual(release_errors(report), [])

    def test_full_runtime_report_passes(self):
        self.assertEqual(release_errors(self.report), [])


class VerificationTest(GateWorkspace):
    """CLI-level verification: artifacts must exist and share one context."""

    def test_valid_report_verifies_clean(self):
        self.assertEqual(verify_report(self.report, self.base), [])

    def test_artifact_file_missing_fails(self):
        os.unlink(os.path.join(self.base, self.report["browser"]["artifact"]))
        errors = verify_report(self.report, self.base)
        self.assertTrue(any(e.startswith("browser: artifact file missing") for e in errors),
                        errors)

    def test_digest_mismatch_fails(self):
        path = os.path.join(self.base, self.report["contract"]["artifact"])
        with open(path, "a", encoding="utf-8") as handle:
            handle.write(" tampered after digest\n")
        errors = verify_report(self.report, self.base)
        self.assertTrue(any(e.startswith("contract: artifact digest mismatch") for e in errors),
                        errors)

    def test_passed_true_but_missing_artifact_file_fails(self):
        self.report["provider_write"]["artifact"] = "artifacts/never-produced.json"
        errors = verify_report(self.report, self.base)
        self.assertTrue(any(e.startswith("provider_write: artifact file missing") for e in errors),
                        errors)

    def test_cross_namespace_fails(self):
        self.report["integration"]["test_namespace"] = "oc17-other-run"
        errors = verify_report(self.report, self.base)
        self.assertIn("integration: test_namespace does not match report context", errors)

    def test_cross_image_fails(self):
        self.report["contract"]["image_digest"] = "sha256:" + "cd" * 32
        errors = verify_report(self.report, self.base)
        self.assertIn("contract: image_digest does not match report context", errors)

    def test_cross_commit_fails(self):
        self.report["billing"]["commit"] = "deadbeef" + "0" * 32
        errors = verify_report(self.report, self.base)
        self.assertIn("billing: commit does not match report context", errors)

    def test_entry_without_context_binding_fails(self):
        del self.report["recovery"]["commit"]
        errors = verify_report(self.report, self.base)
        self.assertIn("recovery: commit does not match report context", errors)

    def test_report_without_context_binding_fails(self):
        del self.report["image_digest"]
        errors = verify_report(self.report, self.base)
        self.assertIn("report: missing image_digest binding", errors)

    def test_missing_sha256_binding_fails(self):
        del self.report["browser"]["sha256"]
        errors = verify_report(self.report, self.base)
        self.assertTrue(any(e.startswith("browser: artifact digest mismatch") for e in errors),
                        errors)

    def test_blocked_env_entries_are_reported_not_trusted(self):
        self.report["provider_write"] = {
            "kind": "blocked-env",
            "passed": False,
            "blocked_env": "missing: authorized provider test account/target/content",
        }
        errors = release_errors(self.report)
        self.assertIn("provider_write: runtime evidence required", errors)


class GateCliTest(GateWorkspace):
    def run_gate(self, report):
        path = os.path.join(self.base, "release-report.json")
        with open(path, "w", encoding="utf-8") as handle:
            if isinstance(report, str):
                handle.write(report)
            else:
                json.dump(report, handle)
        return subprocess.run([sys.executable, GATE, path],
                              capture_output=True, text=True)

    def test_cli_exit_0_on_truthful_report(self):
        proc = self.run_gate(self.report)
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertIn("GATE OK", proc.stdout)

    def test_cli_exit_2_on_mock_write(self):
        report = full_report(self.base)
        report["provider_write"] = {"kind": "mock", "passed": True}
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("provider_write: runtime evidence required", proc.stdout)

    def test_cli_exit_2_on_missing_class(self):
        report = full_report(self.base)
        del report["billing"]
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("billing: runtime evidence required", proc.stdout)

    def test_cli_exit_2_on_missing_artifact_file(self):
        report = full_report(self.base)
        os.unlink(os.path.join(self.base, report["browser"]["artifact"]))
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("browser: artifact file missing", proc.stdout)

    def test_cli_exit_2_on_digest_mismatch(self):
        report = full_report(self.base)
        path = os.path.join(self.base, report["contract"]["artifact"])
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("x")
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("contract: artifact digest mismatch", proc.stdout)

    def test_cli_exit_2_on_cross_namespace(self):
        report = full_report(self.base)
        report["integration"]["test_namespace"] = "oc17-other-run"
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("integration: test_namespace does not match report context", proc.stdout)

    def test_cli_prints_blocked_env_marker(self):
        report = full_report(self.base)
        report["billing"] = {
            "kind": "blocked-env", "passed": False,
            "blocked_env": "missing: commercial metering environment authorization",
        }
        proc = self.run_gate(report)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("billing: runtime evidence required", proc.stdout)
        self.assertIn("blocked-env: billing: missing: commercial metering environment authorization",
                      proc.stdout)

    def test_cli_exit_2_on_invalid_json(self):
        proc = self.run_gate("{not json")
        self.assertEqual(proc.returncode, 2)
        self.assertTrue(proc.stderr or proc.stdout)

    def test_cli_exit_2_on_missing_report_file(self):
        proc = subprocess.run(
            [sys.executable, GATE, os.path.join(self.base, "no-such-report.json")],
            capture_output=True, text=True)
        self.assertEqual(proc.returncode, 2)
        self.assertTrue(proc.stderr)

    def test_cli_exit_2_without_arguments(self):
        proc = subprocess.run([sys.executable, GATE], capture_output=True, text=True)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("usage", proc.stderr)


if __name__ == "__main__":
    unittest.main()
