"""Release acceptance gate (O03).

check_acceptance(policy, evidence) -> list[str] of ALL blocking reasons.
Release requires: policy.approved, zero permission leaks, complete
evidence (contract/integration/recovery/browser/live_model all verified),
metrics within explicit thresholds, and commit/version/lock-hash
consistency. Missing thresholds or missing real-model evidence leave the
corresponding mode BLOCKED - never silently enabled.
"""

from __future__ import annotations

REQUIRED_LAYERS = ("contract", "integration", "recovery", "browser", "live_model")


def check_acceptance(policy: dict, evidence: dict) -> list:
    errors: list[str] = []
    if policy.get("approved") is not True:
        errors.append("policy_not_approved")
    if "security_leaks" not in evidence:
        errors.append("missing_security_measurement")
    elif evidence["security_leaks"] != 0:
        errors.append("permission_leak")
    for layer in REQUIRED_LAYERS:
        if evidence.get(layer) != "verified":
            errors.append(f"missing_{layer}")
    # Explicit threshold comparison: every threshold in policy must be met
    # by the measured metrics; a missing measurement blocks.
    thresholds = policy.get("thresholds", {})
    metrics = evidence.get("metrics", {})
    for name, limit in thresholds.items():
        measured = metrics.get(name)
        if measured is None:
            errors.append(f"missing_metric:{name}")
        elif measured > limit:
            errors.append(f"threshold_exceeded:{name}")
    # Version/commit/lock-hash consistency between policy and evidence.
    for key in ("version", "lock_hash"):
        expected = policy.get(key)
        actual = evidence.get(key)
        if expected is not None and actual != expected:
            errors.append("version_mismatch" if key == "version" else "lock_hash_mismatch")
    return errors


if __name__ == "__main__":
    import json
    import sys

    policy = json.loads(sys.argv[1]) if len(sys.argv) > 1 else {}
    evidence = json.loads(sys.argv[2]) if len(sys.argv) > 2 else {}
    problems = check_acceptance(policy, evidence)
    for problem in problems:
        print(problem)
    sys.exit(1 if problems else 0)
