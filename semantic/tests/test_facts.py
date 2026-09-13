"""C03 fact identity and provenance tests.

Entity identity is a server-generated UUID; entity names/aliases are sourced
assertions (never free-floating strings); source facts need evidence, rule/
model derivations need same-scope premises; dangling references, cross-tenant
premises, derivation cycles and object/value ambiguity are rejected; and
conflicting values coexist instead of being overwritten.
"""

import uuid

import pytest

from semantic_service.contracts import Assertion, Evidence, ScopeKey
from semantic_service.facts import (
    new_entity_id,
    upsert_preserving_conflicts,
    validate_assertion,
    validate_derivation_dag,
)

SCOPE = ScopeKey(tenant_id=1, kb_id="kb")
OTHER_TENANT = ScopeKey(tenant_id=2, kb_id="kb")
OTHER_KB = ScopeKey(tenant_id=1, kb_id="other-kb")


def ev(evidence_id: str) -> Evidence:
    return Evidence(
        evidence_id=evidence_id, document_id="d1", revision=1, chunk_id="c1",
        content_hash="h1", quote="原文")


def source_fact(assertion_id="a1", subject="ent-1", predicate="controls",
                object_id="ent-2", evidence=("e1",), scope=SCOPE):
    return Assertion(
        assertion_id=assertion_id, scope=scope, subject_id=subject,
        predicate=predicate, object_id=object_id, kind="source",
        evidence_ids=list(evidence), premise_ids=[])


def rule_fact(assertion_id="r1", subject="ent-1", predicate="controls",
              object_id="ent-3", premises=("a1",), scope=SCOPE):
    return Assertion(
        assertion_id=assertion_id, scope=scope, subject_id=subject,
        predicate=predicate, object_id=object_id, kind="rule",
        evidence_ids=[], premise_ids=list(premises))


def test_new_entity_id_is_unique_uuid():
    first, second = new_entity_id(), new_entity_id()
    uuid.UUID(first)
    uuid.UUID(second)
    assert first != second


def test_source_fact_passes_with_evidence():
    validate_assertion(source_fact(), {"e1": ev("e1")}, {})


def test_source_fact_requires_evidence():
    fact = source_fact(evidence=())
    with pytest.raises(ValueError, match="evidence"):
        validate_assertion(fact, {}, {})


def test_object_value_exactly_one():
    both = Assertion(
        assertion_id="a2", scope=SCOPE, subject_id="s", predicate="p",
        object_id="o", value="v", kind="source", evidence_ids=["e1"], premise_ids=[])
    with pytest.raises(ValueError, match="exactly one"):
        validate_assertion(both, {"e1": ev("e1")}, {})
    neither = Assertion(
        assertion_id="a3", scope=SCOPE, subject_id="s", predicate="p",
        kind="source", evidence_ids=["e1"], premise_ids=[])
    with pytest.raises(ValueError, match="exactly one"):
        validate_assertion(neither, {"e1": ev("e1")}, {})


def test_dangling_evidence_rejected():
    fact = source_fact(evidence=("e-missing",))
    with pytest.raises(ValueError, match="e-missing"):
        validate_assertion(fact, {"e1": ev("e1")}, {})


def test_dangling_premise_rejected():
    derived = rule_fact(premises=("a-missing",))
    with pytest.raises(ValueError, match="a-missing"):
        validate_assertion(derived, {}, {})


def test_cross_scope_premise_rejected():
    for foreign_scope in (OTHER_TENANT, OTHER_KB):
        premise = source_fact(assertion_id="a1", scope=foreign_scope)
        derived = rule_fact(premises=("a1",))
        with pytest.raises(ValueError, match="scope"):
            validate_assertion(derived, {"e1": ev("e1")}, {"a1": premise})


def test_source_fact_cannot_cite_premises():
    fact = Assertion(
        assertion_id="a4", scope=SCOPE, subject_id="s", predicate="p",
        object_id="o", kind="source", evidence_ids=["e1"], premise_ids=["a1"])
    with pytest.raises(ValueError, match="premise"):
        validate_assertion(fact, {"e1": ev("e1")}, {"a1": source_fact()})


def test_rule_derivation_requires_premises():
    derived = rule_fact(premises=())
    with pytest.raises(ValueError, match="premise"):
        validate_assertion(derived, {}, {})


def test_unknown_kind_rejected():
    fact = Assertion(
        assertion_id="a5", scope=SCOPE, subject_id="s", predicate="p",
        object_id="o", kind="hallucinated", evidence_ids=["e1"], premise_ids=[])
    with pytest.raises(ValueError, match="kind"):
        validate_assertion(fact, {"e1": ev("e1")}, {})


def test_derivation_cycle_rejected():
    first = rule_fact(assertion_id="r1", premises=("r2",))
    second = rule_fact(assertion_id="r2", premises=("r1",))
    with pytest.raises(ValueError, match="cycle"):
        validate_derivation_dag([first, second])


def test_deep_derivation_chain_ok():
    base = source_fact(assertion_id="a1")
    mid = rule_fact(assertion_id="r1", premises=("a1",))
    top = rule_fact(assertion_id="r2", premises=("r1",))
    validate_derivation_dag([base, mid, top])


def test_conflicting_values_coexist_not_overwritten():
    fact_a = source_fact(assertion_id="f1", subject="ent-甲", predicate="controls", object_id="ent-乙")
    fact_b = source_fact(assertion_id="f2", subject="ent-甲", predicate="controls", object_id="ent-丙")
    merged = upsert_preserving_conflicts([fact_a], fact_b)
    assert len(merged) == 2
    assert {item.object_id for item in merged} == {"ent-乙", "ent-丙"}


def test_identical_fact_upsert_is_idempotent():
    fact = source_fact(assertion_id="f1")
    merged = upsert_preserving_conflicts([fact], fact)
    assert len(merged) == 1


def test_multi_source_support_is_never_dropped():
    # Spec section 4 + must-test scenario 4: the same claim supported by TWO
    # documents keeps BOTH support entries - deleting one document later
    # revokes only its own support.
    from_doc1 = source_fact(assertion_id="f-d1", subject="ent-甲", predicate="controls",
                            object_id="ent-乙", evidence=("e-d1",))
    from_doc2 = source_fact(assertion_id="f-d2", subject="ent-甲", predicate="controls",
                            object_id="ent-乙", evidence=("e-d2",))
    merged = upsert_preserving_conflicts([from_doc1], from_doc2)
    assert len(merged) == 2, "a second document's support must never be silently dropped"
    assert {tuple(item.evidence_ids) for item in merged} == {("e-d1",), ("e-d2",)}


def test_identical_claim_same_support_deduplicated():
    once = source_fact(assertion_id="f-d1", evidence=("e-d1",))
    again = source_fact(assertion_id="f-d1", evidence=("e-d1",))
    merged = upsert_preserving_conflicts([once], again)
    assert len(merged) == 1


def test_evidence_revision_zero_rejected_by_validate():
    # The C01 DTO allows revision=0 at construction (uint64 >= 0); the
    # domain gate must reject it.
    from semantic_service.contracts import ChunkSnapshot, Evidence
    from semantic_service.evidence import validate_evidence

    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e0", document_id="d1", revision=0, chunk_id="c1",
        content_hash="h1", quote="控股", start_char=None, end_char=None)
    with pytest.raises(ValueError, match="revision"):
        validate_evidence(chunk, evidence)


def test_model_derivation_requires_premises():
    model_conclusion = Assertion(
        assertion_id="m1", scope=SCOPE, subject_id="s", predicate="p",
        object_id="o", kind="model", evidence_ids=[], premise_ids=[])
    with pytest.raises(ValueError, match="premise"):
        validate_assertion(model_conclusion, {}, {})


def test_same_assertion_id_different_content_raises():
    # Same id claiming a DIFFERENT object is an upstream id bug: fail loudly
    # instead of silently keeping either version.
    original = source_fact(assertion_id="f-1", object_id="ent-乙")
    imposter = source_fact(assertion_id="f-1", object_id="ent-丙")
    with pytest.raises(ValueError, match="assertion_id"):
        upsert_preserving_conflicts([original], imposter)


def test_self_referencing_premise_is_a_cycle():
    self_loop = rule_fact(assertion_id="r-self", premises=("r-self",))
    with pytest.raises(ValueError, match="cycle"):
        validate_derivation_dag([self_loop])


def test_duplicate_assertion_ids_in_dag_input_rejected():
    first = source_fact(assertion_id="dup")
    second = source_fact(assertion_id="dup", object_id="ent-丙")
    with pytest.raises(ValueError, match="duplicate"):
        validate_derivation_dag([first, second])


def test_deep_derivation_chain_does_not_crash():
    # A long top-down chain must not blow the recursion limit: cycles and
    # acyclicity are judged iteratively.
    depth = 3000
    base = source_fact(assertion_id="a0")
    chain = [base] + [
        rule_fact(assertion_id=f"r{i}", premises=(f"r{i-1}" if i > 1 else "a0",))
        for i in range(1, depth)
    ]
    validate_derivation_dag(chain)  # must not raise (RecursionError included)


def test_empty_object_id_or_value_rejected():
    empty_object = Assertion(
        assertion_id="a-e1", scope=SCOPE, subject_id="s", predicate="p",
        object_id="", kind="source", evidence_ids=["e1"], premise_ids=[])
    with pytest.raises(ValueError, match="exactly one"):
        validate_assertion(empty_object, {"e1": ev("e1")}, {})
    empty_value = Assertion(
        assertion_id="a-e2", scope=SCOPE, subject_id="s", predicate="p",
        value="", kind="source", evidence_ids=["e1"], premise_ids=[])
    with pytest.raises(ValueError, match="exactly one"):
        validate_assertion(empty_value, {"e1": ev("e1")}, {})


def test_derivation_must_not_cite_evidence_directly():
    # Derivations cite premises; their evidence is transitive through the
    # premise chain. Direct evidence on a derived conclusion muddies the
    # rule-proof vs source-fact provenance the spec separates.
    mixed = Assertion(
        assertion_id="r-mixed", scope=SCOPE, subject_id="s", predicate="p",
        object_id="o", kind="rule", evidence_ids=["e1"], premise_ids=["a1"])
    with pytest.raises(ValueError, match="evidence"):
        validate_assertion(mixed, {"e1": ev("e1")}, {"a1": source_fact()})


def test_scope_validation_rejects_zero_tenant_and_blank_kb():
    zero_tenant = source_fact(scope=ScopeKey(tenant_id=0, kb_id="kb"))
    with pytest.raises(ValueError, match="tenant"):
        validate_assertion(zero_tenant, {"e1": ev("e1")}, {})
    blank_kb = source_fact(scope=ScopeKey(tenant_id=1, kb_id="   "))
    with pytest.raises(ValueError, match="kb"):
        validate_assertion(blank_kb, {"e1": ev("e1")}, {})


def test_entity_name_and_alias_are_sourced_assertions():
    name = Assertion(
        assertion_id="n1", scope=SCOPE, subject_id="ent-1", predicate="name",
        value="甲公司", kind="source", evidence_ids=["e1"], premise_ids=[])
    validate_assertion(name, {"e1": ev("e1")}, {})
    alias_without_evidence = Assertion(
        assertion_id="n2", scope=SCOPE, subject_id="ent-1", predicate="alias",
        value="甲", kind="source", evidence_ids=[], premise_ids=[])
    with pytest.raises(ValueError, match="evidence"):
        validate_assertion(alias_without_evidence, {}, {})
