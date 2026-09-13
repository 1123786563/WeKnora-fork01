"""Release acceptance gate (O03).

check_acceptance(policy, evidence) -> list[str] of ALL blocking reasons.
Release requires: policy.approved, zero permission leaks, complete
evidence (contract/integration/recovery/browser/live_model all verified),
metrics within explicit thresholds, and version/lock-hash consistency.
Threshold DIRECTION is encoded by suffix: names ending _max compare
measured <= limit; names ending _min compare measured >= limit; any other
name is rejected as invalid_threshold (fail closed on ambiguity).
"""

from __future__ import annotations

REQUIRED_LAYERS = ("contract", "integration", "recovery", "browser", "live_model")


def check_acceptance(policy: dict, evidence: dict) -> list[str]:
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
    thresholds = policy.get("thresholds", {})
    metrics = evidence.get("metrics", {})
    for name, limit in thresholds.items():
        measured = metrics.get(name)
        if measured is None:
            errors.append(f"missing_metric:{name}")
            continue
        if not isinstance(measured, (int, float)) or not isinstance(limit, (int, float)):
            errors.append(f"invalid_metric:{name}")
            continue
        if name.endswith("_max"):
            if measured > limit:
                errors.append(f"threshold_exceeded:{name}")
        elif name.endswith("_min"):
            if measured < limit:
                errors.append(f"threshold_below:{name}")
        else:
            errors.append(f"invalid_threshold_name:{name}")
    for key in ("version", "lock_hash"):
        expected = policy.get(key)
        actual = evidence.get(key)
        if expected is not None and actual != expected:
            errors.append("version_mismatch" if key == "version" else "lock_hash_mismatch")
    return errors


if __name__ == "__main__":
    import json
    import sys

    try:
        policy = json.loads(sys.argv[1]) if len(sys.argv) > 1 else {}
        evidence = json.loads(sys.argv[2]) if len(sys.argv) > 2 else {}
    except (json.JSONDecodeError, IndexError) as exc:
        print(f"usage: check_acceptance POLICY_JSON EVIDENCE_JSON ({exc})", file=sys.stderr)
        sys.exit(2)
    problems = check_acceptance(policy, evidence)
    for problem in problems:
        print(problem)
    sys.exit(1 if problems else 0)
