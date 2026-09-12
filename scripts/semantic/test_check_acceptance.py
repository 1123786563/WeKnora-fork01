"""O03 acceptance-gate tests: failure behavior for every blocking cause."""

import pytest

from check_acceptance import check_acceptance


def test_unapproved_policy_blocks_release():
    errors = check_acceptance({"approved": False}, {"security_leaks": 0})
    assert "policy_not_approved" in errors


def test_any_permission_leak_blocks_release():
    errors = check_acceptance({"approved": True}, {"security_leaks": 1})
    assert "permission_leak" in errors


def test_missing_security_measurement_blocks():
    errors = check_acceptance({"approved": True}, {})
    assert "missing_security_measurement" in errors


def test_missing_evidence_layers_block():
    full_security = {"security_leaks": 0}
    errors = check_acceptance({"approved": True}, dict(full_security))
    for layer in ("contract", "integration", "recovery", "browser", "live_model"):
        assert f"missing_{layer}" in errors


def test_threshold_breach_blocks():
    policy = {"approved": True, "thresholds": {"p95_latency_ms": 2000}}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "metrics": {"p95_latency_ms": 3500}}
    errors = check_acceptance(policy, evidence)
    assert "threshold_exceeded:p95_latency_ms" in errors


def test_version_mismatch_blocks():
    policy = {"approved": True, "version": "abc123", "lock_hash": "lh1"}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "version": "abc123", "lock_hash": "lh2"}
    errors = check_acceptance(policy, evidence)
    assert "lock_hash_mismatch" in errors


def test_fully_satisfied_policy_passes():
    policy = {"approved": True, "version": "v1", "lock_hash": "lh1"}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "version": "v1", "lock_hash": "lh1"}
    assert check_acceptance(policy, evidence) == []
