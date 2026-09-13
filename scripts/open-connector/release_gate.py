#!/usr/bin/env python3
"""Rollout release gate for the open-connector integration (T18).

A release report is a JSON object binding SEVEN evidence classes
(contract / integration / browser / provider_read / provider_write /
billing / recovery) to one release context (the WeKnora commit, the pinned
runtime image digest, the release test namespace). The gate fails closed:

  * every class needs runtime evidence — kind=runtime, passed is True, and a
    non-empty artifact reference (documentation/mock evidence never counts;
    a mock provider_write is exactly what this gate exists to reject);
  * the release test namespace must end with zero unresolved executions
    (open_unknown_count == 0);
  * the CLI never trusts passed=true alone: it re-verifies every artifact
    file on disk (exists + sha256 digest match) and that each entry was
    produced under the SAME commit / image digest / test namespace as the
    report header.

Report shape (artifact paths are relative to the report file's directory):

  {
    "commit": "<40-hex WeKnora release commit>",
    "image_digest": "sha256:<64-hex>",
    "test_namespace": "oc-release-<id>",
    "open_unknown_count": 0,
    "contract":       {"kind": "runtime", "passed": true,
                       "artifact": "artifacts/contract.json", "sha256": "<64-hex>",
                       "commit": "...", "image_digest": "sha256:...",
                       "test_namespace": "..."},
    ...
  }

A class that cannot be evidenced yet is recorded with
{"kind": "blocked-env", "blocked_env": "missing: <specific item>"} — the gate
still FAILS for that class (that is the point), and the CLI echoes the marker
so the rollout report states the missing item instead of a summary-only
claim. See docs/integrations/open-connector-release.md.
"""

import hashlib
import json
import os
import sys

CONTEXT_KEYS = ("commit", "image_digest", "test_namespace")

REQUIRED = ("contract", "integration", "browser", "provider_read",
            "provider_write", "billing", "recovery")


def release_errors(report):
    errors = []
    for name in ("contract", "integration", "browser", "provider_read", "provider_write", "billing", "recovery"):
        entry = report.get(name, {})
        if entry.get("kind") != "runtime" or entry.get("passed") is not True or not entry.get("artifact"):
            errors.append(name + ": runtime evidence required")
    if report.get("open_unknown_count", -1) != 0:
        errors.append("unresolved executions in release test namespace")
    return errors


def _sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_report(report, base_path):
    """Independently verify artifact files and the single release context.

    Never trusts passed=true: an entry only counts when its artifact file
    exists, its recorded sha256 matches the file on disk, and the entry was
    produced under the same commit / image digest / test namespace as the
    report header. Returns a list of human-readable errors (empty = clean).
    """
    errors = []
    if not isinstance(report, dict):
        return ["report: not a JSON object"]
    for key in CONTEXT_KEYS:
        if not report.get(key):
            errors.append("report: missing %s binding" % key)
    for name in REQUIRED:
        entry = report.get(name, {})
        if not isinstance(entry, dict):
            errors.append("%s: evidence entry is not an object" % name)
            continue
        artifact = entry.get("artifact")
        if entry.get("kind") != "runtime" or entry.get("passed") is not True or not artifact:
            # Already flagged by release_errors; nothing on disk to verify.
            continue
        if not isinstance(artifact, str):
            # QR-F1 type guard: a truthy non-string artifact (list/dict/…)
            # would crash path joining below; it is malformed evidence.
            errors.append("%s: artifact reference is not a string" % name)
            continue
        for key in CONTEXT_KEYS:
            if entry.get(key) != report.get(key):
                errors.append("%s: %s does not match report context" % (name, key))
        path = artifact if os.path.isabs(artifact) else os.path.join(base_path, artifact)
        if not os.path.isfile(path):
            errors.append("%s: artifact file missing: %s" % (name, artifact))
            continue
        digest = _sha256_file(path)
        if entry.get("sha256") != digest:
            errors.append("%s: artifact digest mismatch (recorded %r, file %s)"
                          % (name, entry.get("sha256"), digest))
    return errors


def _blocked_env_markers(report):
    markers = []
    for name in REQUIRED:
        entry = report.get(name, {})
        if isinstance(entry, dict) and entry.get("blocked_env"):
            markers.append("blocked-env: %s: %s" % (name, entry["blocked_env"]))
    return markers


def _cli_precheck_errors(report):
    """CLI-boundary shape precheck (QR-F1/QR-F3).

    release_errors stays plan-verbatim, so known malformed shapes are caught
    HERE before it runs: a non-dict report/entry would crash it with an
    AttributeError, and bool open_unknown_count compares equal to 0 in Python
    (False == 0), which must never qualify as "zero unresolved executions".
    """
    errors = []
    for name in REQUIRED:
        if name in report and not isinstance(report[name], dict):
            errors.append("%s: evidence entry is not a JSON object" % name)
    unknown = report.get("open_unknown_count")
    if isinstance(unknown, bool) or not isinstance(unknown, int):
        errors.append("open_unknown_count must be an integer, got %s"
                      % type(unknown).__name__)
    return errors


def main(argv):
    if len(argv) != 2:
        print("usage: release_gate.py <release-report.json>", file=sys.stderr)
        return 2
    try:
        with open(argv[1], "r", encoding="utf-8") as handle:
            report = json.load(handle)
    except RecursionError as exc:
        print("invalid release report file: nesting too deep: %s" % exc, file=sys.stderr)
        return 2
    except (OSError, ValueError) as exc:
        print("invalid release report file: %s" % exc, file=sys.stderr)
        return 2
    if not isinstance(report, dict):
        print("GATE FAIL: release report is not a JSON object")
        return 2
    precheck = _cli_precheck_errors(report)
    if precheck:
        for error in precheck:
            print("GATE FAIL: %s" % error)
        return 2
    base_path = os.path.dirname(os.path.abspath(argv[1]))
    try:
        errors = release_errors(report) + verify_report(report, base_path)
        markers = _blocked_env_markers(report)
    except Exception as exc:  # CLI boundary: fail closed, never a traceback
        print("GATE FAIL: release report is not analyzable: %s" % exc)
        return 2
    if errors:
        for error in errors:
            print("GATE FAIL: %s" % error)
        for marker in markers:
            print(marker)
        return 2
    print("GATE OK: seven evidence classes verified on one commit/image/namespace")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
