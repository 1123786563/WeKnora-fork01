from __future__ import annotations


def test_registered_rule_requires_both_premises() -> None:
    from bridge_probe import probe_rule

    result = probe_rule(["a-d1"], "technical-dependency-transitivity@v1")

    assert result["status"] == "insufficient_evidence"
    assert result["conclusions"] == []


def test_registered_rule_returns_exact_provenance() -> None:
    from bridge_probe import probe_rule

    result = probe_rule(["a-d1", "a-d2"], "technical-dependency-transitivity@v1")

    assert result == {
        "status": "derived",
        "conclusions": [
            {
                "assertion_id": "r-a-d1-a-d2",
                "predicate": "indirectly_depends_on",
                "subject": "A",
                "object": "C",
                "kind": "rule",
                "rule_id": "technical-dependency-transitivity@v1",
                "premise_ids": ["a-d1", "a-d2"],
                "evidence_ids": ["e-d1", "e-d2"],
            }
        ],
    }
