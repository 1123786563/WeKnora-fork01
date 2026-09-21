from __future__ import annotations


def test_registered_rule_requires_both_premises() -> None:
    from bridge_probe import probe_rule

    result = probe_rule([{"semantic_id": "a-d1", "predicate": "depends_on", "subject": "A", "object": "B", "evidence_ids": ["e-d1"]}], "technical-dependency-transitivity@v1")

    assert result["status"] == "insufficient_evidence"
    assert result["conclusions"] == []


def test_registered_rule_returns_exact_provenance() -> None:
    from bridge_probe import probe_rule

    result = probe_rule([
        {"semantic_id": "a-d1", "predicate": "depends_on", "subject": "A", "object": "B", "evidence_ids": ["e-d1"]},
        {"semantic_id": "a-d2", "predicate": "depends_on", "subject": "B", "object": "C", "evidence_ids": ["e-d2"]},
    ], "technical-dependency-transitivity@v1")

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


def test_registered_rule_rejects_wrong_persisted_predicate() -> None:
    from bridge_probe import probe_rule

    result = probe_rule([
        {"semantic_id": "a-d1", "predicate": "alias", "subject": "A", "object": "B", "evidence_ids": ["e-d1"]},
        {"semantic_id": "a-d2", "predicate": "depends_on", "subject": "B", "object": "C", "evidence_ids": ["e-d2"]},
    ], "technical-dependency-transitivity@v1")

    assert result == {"status": "insufficient_evidence", "conclusions": []}
