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


def main(argv):
    if len(argv) != 2:
        print("usage: release_gate.py <release-report.json>", file=sys.stderr)
        return 2
    try:
        with open(argv[1], "r", encoding="utf-8") as handle:
            report = json.load(handle)
    except (OSError, ValueError) as exc:
        print("invalid release report file: %s" % exc, file=sys.stderr)
        return 2
    base_path = os.path.dirname(os.path.abspath(argv[1]))
    errors = release_errors(report) + verify_report(report, base_path)
    if errors:
        for error in errors:
            print("GATE FAIL: %s" % error)
        for marker in _blocked_env_markers(report):
            print(marker)
        return 2
    print("GATE OK: seven evidence classes verified on one commit/image/namespace")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
