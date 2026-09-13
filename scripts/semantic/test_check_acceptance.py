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
    policy = {"approved": True, "thresholds": {"p95_latency_ms_max": 2000}}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "metrics": {"p95_latency_ms_max": 3500}}
    errors = check_acceptance(policy, evidence)
    assert "threshold_exceeded:p95_latency_ms_max" in errors


def test_lock_hash_mismatch_blocks():
    policy = {"approved": True, "version": "abc123", "lock_hash": "lh1"}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "version": "abc123", "lock_hash": "lh2"}
    errors = check_acceptance(policy, evidence)
    assert "lock_hash_mismatch" in errors


def test_min_threshold_below_floor_blocks():
    policy = {"approved": True, "thresholds": {"correct_rate_min": 0.7}}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "metrics": {"correct_rate_min": 0.5}}
    errors = check_acceptance(policy, evidence)
    assert "threshold_below:correct_rate_min" in errors, "below-floor must BLOCK"


def test_min_threshold_above_floor_passes():
    policy = {"approved": True, "thresholds": {"correct_rate_min": 0.7}}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "metrics": {"correct_rate_min": 0.9}}
    assert check_acceptance(policy, evidence) == []


def test_ambiguous_threshold_name_rejected():
    policy = {"approved": True, "thresholds": {"correct_rate": 0.7}}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "metrics": {"correct_rate": 0.9}}
    errors = check_acceptance(policy, evidence)
    assert "invalid_threshold_name:correct_rate" in errors, "ambiguous direction must fail closed"


def test_string_metric_rejected_not_crash():
    policy = {"approved": True, "thresholds": {"p95_max": 2000}}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "metrics": {"p95_max": "3500"}}
    errors = check_acceptance(policy, evidence)
    assert "invalid_metric:p95_max" in errors


def test_threshold_boundary_equal_passes():
    policy = {"approved": True, "thresholds": {"p95_max": 2000}}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "metrics": {"p95_max": 2000}}
    assert check_acceptance(policy, evidence) == []


def test_version_mismatch_blocks():
    policy = {"approved": True, "version": "v1", "lock_hash": "lh1"}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "version": "v2", "lock_hash": "lh1"}
    errors = check_acceptance(policy, evidence)
    assert "version_mismatch" in errors


def test_fully_satisfied_policy_passes():
    policy = {"approved": True, "version": "v1", "lock_hash": "lh1"}
    evidence = {"security_leaks": 0, "contract": "verified", "integration": "verified",
                "recovery": "verified", "browser": "verified", "live_model": "verified",
                "version": "v1", "lock_hash": "lh1"}
    assert check_acceptance(policy, evidence) == []
