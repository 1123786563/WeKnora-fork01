#!/usr/bin/env python3
"""Evidence gate for the pinned open-connector runtime contract (T01).

Fails closed: a report passes only when it pins the reviewed upstream SHA,
carries an immutable sha256 image digest, and every REQUIRED contract case
is backed by runtime evidence (kind=runtime, passed=true, non-empty
artifact). Documentation-only evidence never satisfies a required case.
"""

import json
import sys

PINNED_SHA = "33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a"

REQUIRED = {
    "cross_connection", "empty_grant", "default_alias", "no_auth",
    "admin_denied", "proxy_denied", "oauth_correlation", "key_replay",
    "key_conflict", "in_progress", "expired_key", "audit_failure",
}


def validate(report):
    """Return a list of human-readable errors; empty list means the report passes."""
    if not isinstance(report, dict):
        return ["report: not a JSON object"]
    errors = []
    if report.get("sha") != PINNED_SHA:
        errors.append("unreviewed upstream SHA")
    digest = report.get("image_digest")
    if not isinstance(digest, str) or not digest.startswith("sha256:"):
        errors.append("missing immutable image")
    cases = {}
    raw_cases = report.get("cases")
    if isinstance(raw_cases, list):
        for case in raw_cases:
            if isinstance(case, dict) and case.get("id"):
                case_id = case["id"]
                if case_id in cases:
                    errors.append(case_id + ": duplicate case id")
                else:
                    cases[case_id] = case
    for name in sorted(REQUIRED):
        case = cases.get(name, {})
        if (not isinstance(case, dict)
                or case.get("kind") != "runtime"
                or case.get("passed") is not True
                or not case.get("artifact")):
            errors.append(name + ": missing runtime proof")
    return errors


def main(argv):
    if len(argv) != 2:
        print("usage: contract_gate.py <report.json>", file=sys.stderr)
        return 2
    try:
        with open(argv[1], "r", encoding="utf-8") as handle:
            report = json.load(handle)
    except (OSError, ValueError) as exc:
        print("invalid report file: %s" % exc, file=sys.stderr)
        return 2
    errors = validate(report)
    if errors:
        for error in errors:
            print("GATE FAIL: %s" % error)
        return 2
    print("GATE OK: pinned runtime contract evidence complete")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
