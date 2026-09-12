"""Q02 rule reasoning tests: premises, conflicts, cycles, limits."""

import pytest

from semantic_service.query.reason_rules import (InvalidProof, ReasonRequest, ReasonResult,
                                                  RuleReasoner)


def write_ruleset(path, payload: dict) -> None:
    """Write a temp ruleset WITH its content digest (the registry requires
    a declared digest)."""
    import hashlib
    import json as _json

    canonical = _json.dumps({"version": payload["version"], "rules": payload["rules"]},
                            sort_keys=True, ensure_ascii=False)
    payload["digest"] = "sha256:" + hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    path.write_text(_json.dumps(payload), encoding="utf-8")
from semantic_service.query.rules import RuleGrammarError, RuleRegistry


@pytest.fixture()
def rule_reasoner():
    return RuleReasoner(RuleRegistry())


def test_transitive_rule_cannot_infer_without_second_edge(rule_reasoner):
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1")], goal=("a", "controls", "c"))
    assert result.status == "insufficient_evidence"


def test_proof_lists_both_premises(rule_reasoner):
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", "a2")],
        goal=("a", "controls", "c"))
    assert set(result.premise_ids) == {"a1", "a2"}


def test_supported_conclusion_carries_rule_id(rule_reasoner):
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", "a2")],
        goal=("a", "controls", "c"))
    assert result.status == "supported"
    assert result.rule_ids == ["control-transitivity"]
    assert "control-transitivity" in result.explanation


def test_multi_hop_derivation(rule_reasoner):
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", "a2"),
               ("c", "controls", "d", "a3")],
        goal=("a", "controls", "d"))
    assert result.status == "supported"
    assert set(result.premise_ids) == {"a1", "a2", "a3"}


def test_unknown_rule_version_rejected(rule_reasoner):
    with pytest.raises(FileNotFoundError):
        rule_reasoner.reason(ReasonRequest(
            rule_set_version="evil-unregistered", facts=(), goal=("a", "controls", "c")))


def test_round_budget_exhausted_reported(rule_reasoner):
    # Chain too long for the budget: explicit budget_exhausted, not a hang.
    facts = [("x0", "controls", "x1", "s0")]
    for i in range(1, 12):
        facts.append((f"x{i}", "controls", f"x{i+1}", f"s{i}"))
    result = rule_reasoner.reason(ReasonRequest(
        rule_set_version="test-control-v1", facts=tuple(facts),
        goal=("x0", "controls", "x11"), max_rounds=2))
    assert result.status == "budget_exhausted"


def test_missing_premise_is_not_negation(rule_reasoner):
    """insufficient_evidence - never a derived negative."""
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1")], goal=("a", "controls", "z"))
    assert result.status == "insufficient_evidence"
    assert result.premise_ids == []


def test_explanation_from_proof_structure_only(rule_reasoner):
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", "a2")],
        goal=("a", "controls", "c"))
    assert "a1" in result.explanation and "a2" in result.explanation
    assert result.explanation.startswith("by rule")


def test_registry_digest_stable(rule_reasoner):
    registry = RuleRegistry()
    ruleset = registry.load("test-control-v1")
    assert ruleset.digest.startswith("sha256:")
    assert registry.load("test-control-v1").digest == ruleset.digest


def test_premises_subset_of_input_fact_ids(rule_reasoner):
    """The real invariant the reasoner guarantees: every premise id cited
    by a proof comes from the PROVIDED facts (authorization filtering of
    inputs happens upstream, spec section 5 - the reasoner never sees
    unauthorized facts to cite)."""
    facts = [("a", "controls", "b", "a1"), ("b", "controls", "c", "a2")]
    result = rule_reasoner.reason_fixture(facts=facts, goal=("a", "controls", "c"))
    assert result.status == "supported"
    input_ids = {f[3] for f in facts}
    assert set(result.premise_ids) <= input_ids, \
        "reasoner cited a premise that was never among its inputs"


def test_self_deriving_rule_raises_invalid_proof_for_new_facts(tmp_path):
    """With the cycle check ordered FIRST, a self-deriving rule genuinely
    raises InvalidProof when it attempts to conclude its own premise as a
    NEW fact (goal not already a source)."""
    loop = tmp_path / "selfloop-v1.json"
    write_ruleset(loop, {
        "version": "selfloop-v1",
        "rules": [{
            "rule_id": "selfloop",
            "if": [{"subject": "?x", "predicate": "controls", "object": "?x"}],
            "then": {"subject": "?x", "predicate": "controls", "object": "?x"},
        }],
    })
    reasoner = RuleReasoner(RuleRegistry(tmp_path))
    with pytest.raises(InvalidProof, match="derives its own premise"):
        reasoner.reason_fixture(
            facts=[("a", "controls", "a", "a1")],
            goal=("b", "controls", "b"), rule_set_version="selfloop-v1")


def test_traversal_version_rejected(rule_reasoner):
    with pytest.raises(FileNotFoundError):
        rule_reasoner.reason(ReasonRequest(
            rule_set_version="../../etc/passwd", facts=(), goal=("a", "controls", "c")))


def test_id_collision_with_derived_namespace_is_safe(rule_reasoner):
    """Caller-supplied source ids colliding with the derived-id namespace
    cannot corrupt proofs: premise edges are keyed by fact TUPLE, not id."""
    sneaky = "control-transitivity:b:c"  # would collide with a derived id
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", sneaky)],
        goal=("a", "controls", "c"))
    assert result.status == "supported"
    assert sneaky in result.premise_ids, "the colliding SOURCE id must survive attribution"


def test_id_collision_with_derived_namespace_is_safe(rule_reasoner):
    """Caller-supplied source ids colliding with the derived-id namespace
    cannot corrupt proofs: premise edges are keyed by fact TUPLE, not id."""
    sneaky = "control-transitivity:b:c"  # would collide with a derived id
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", sneaky)],
        goal=("a", "controls", "c"))
    assert result.status == "supported"
    assert sneaky in result.premise_ids, "the colliding SOURCE id must survive attribution"


def test_digest_tamper_rejected(tmp_path):
    import json as _json

    tampered = tmp_path / "tampered-v1.json"
    write_ruleset(tampered, {
        "version": "tampered-v1",
        "rules": [{
            "rule_id": "r",
            "if": [{"subject": "?x", "predicate": "controls", "object": "?y"}],
            "then": {"subject": "?x", "predicate": "controls", "object": "?y"},
        }],
    })
    payload = _json.loads(tampered.read_text(encoding="utf-8"))
    payload["digest"] = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    tampered.write_text(_json.dumps(payload), encoding="utf-8")
    with pytest.raises(Exception, match="digest"):
        RuleRegistry(tmp_path).load("tampered-v1")


def test_unregistered_rule_id_invalid_proof(rule_reasoner):
    from semantic_service.query.rules import RuleSet
    ruleset = RuleRegistry().load("test-control-v1")
    assert ruleset.contains("control-transitivity", "test-control-v1") is True
    assert ruleset.contains("control-transitivity", "other-version") is False
    assert ruleset.contains("evil-rule", "test-control-v1") is False


def test_conclusion_kind_is_rule_not_source(rule_reasoner):
    """Rule conclusions are never persisted as source facts (spec 10)."""
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", "a2")],
        goal=("a", "controls", "c"))
    derived_id = f"control-transitivity:a:c"
    assert derived_id not in result.premise_ids, "conclusion id must not appear as a source premise"


def test_self_deriving_rule_preserves_proof_dag(tmp_path):
    """DAG invariant (plan step 4): a self-deriving rule can never corrupt
    the proof structure — forward chaining only concludes NEW facts, so a
    conclusion equal to an existing premise is skipped (already-known) and
    the source attribution survives untouched."""
    import json as _json

    loop = tmp_path / "selfloop-v1.json"
    write_ruleset(loop, {
        "version": "selfloop-v1",
        "rules": [{
            "rule_id": "selfloop",
            "if": [{"subject": "?x", "predicate": "controls", "object": "?x"}],
            "then": {"subject": "?x", "predicate": "controls", "object": "?x"},
        }],
    })
    reasoner = RuleReasoner(RuleRegistry(tmp_path))
    result = reasoner.reason_fixture(
        facts=[("a", "controls", "a", "a1")],
        goal=("a", "controls", "a"), rule_set_version="selfloop-v1")
    # The goal holds via its SOURCE fact; the looping rule added nothing
    # and the premise stays the pure source id.
    assert result.status == "supported"
    assert result.premise_ids == ["a1"]
    assert result.rule_ids == []


def test_grammar_rejects_unknown_predicate(rule_reasoner, tmp_path):
    evil = tmp_path / "evil-v1.json"
    write_ruleset(evil, {
        "version": "evil-v1",
        "rules": [{
            "rule_id": "x",
            "if": [{"subject": "?x", "predicate": "execute_sql", "object": "?y"}],
            "then": {"subject": "?x", "predicate": "execute_sql", "object": "?y"},
        }],
    })
    registry = RuleRegistry(tmp_path)
    with pytest.raises(RuleGrammarError, match="whitelist"):
        registry.load("evil-v1")
