from datetime import datetime, timezone
import uuid

import pytest

from semantic_service.contracts import Evidence, ScopeKey
from semantic_service.facts import (
    Assertion,
    AssertionKind,
    Derivation,
    Entity,
    EntityRef,
    ScopedEvidence,
    new_entity_id,
    validate_assertion,
)


SCOPE = ScopeKey(1, "kb-a")
OTHER_SCOPE = ScopeKey(2, "kb-a")
ENTITY_1 = "00000000-0000-4000-8000-000000000001"
ENTITY_2 = "00000000-0000-4000-8000-000000000002"


def make_assertion(**overrides):
    values = dict(
        assertion_id="a1", scope=SCOPE, subject=EntityRef(ENTITY_1, SCOPE),
        predicate="depends_on", object_id=EntityRef(ENTITY_2, SCOPE), value=None,
        kind=AssertionKind.SOURCE, evidence_ids=("src-1",), derivation=None,
        valid_from=None, valid_until=None,
    )
    values.update(overrides)
    return Assertion(**values)


def source_assertion(assertion_id="a1", scope=SCOPE, evidence_id="src-1", value="claimed"):
    return make_assertion(assertion_id=assertion_id, scope=scope,
                          subject=EntityRef(ENTITY_1, scope), object_id=None,
                          value=value, kind=AssertionKind.SOURCE,
                          evidence_ids=(evidence_id,), derivation=None)


def rule_assertion(assertion_id="r1", scope=SCOPE, premise_id="p1"):
    derivation = Derivation(scope=scope, conclusion_id=assertion_id, premise_ids=(premise_id,),
                            rule_id="depends_on_transitive", rule_version="v1")
    return make_assertion(assertion_id=assertion_id, scope=scope,
                          kind=AssertionKind.RULE_DERIVED, evidence_ids=(),
                          derivation=derivation)


def evidence_map(evidence_id, scope=SCOPE):
    evidence = Evidence(evidence_id, "doc-1", 1, "chunk-1", "opaque-hash", "claimed", None, None)
    return {evidence_id: ScopedEvidence(scope=scope, evidence=evidence)}


def test_new_entity_id_is_uuid_and_entity_has_no_name_identity():
    entity_id = new_entity_id()
    assert str(uuid.UUID(entity_id)) == entity_id
    entity = Entity(entity_id, SCOPE, "component")
    assert entity.entity_id == entity_id
    assert not hasattr(entity, "name")


def test_assertion_requires_exactly_one_object_or_value():
    with pytest.raises(ValueError):
        make_assertion(object_id=None, value=None)
    with pytest.raises(ValueError):
        make_assertion(object_id=EntityRef(ENTITY_2, SCOPE), value="literal")


def test_source_assertion_retains_conflicting_values():
    first = source_assertion(assertion_id="a1", value="1", evidence_id="src-1")
    second = source_assertion(assertion_id="a2", value="2", evidence_id="src-2")
    validate_assertion(first, evidence_map("src-1"), {})
    validate_assertion(second, evidence_map("src-2"), {})
    assert first != second


def test_cross_scope_evidence_and_premises_are_rejected():
    assertion = source_assertion(scope=SCOPE, evidence_id="src-1")
    with pytest.raises(ValueError):
        validate_assertion(assertion, evidence_map("src-1", scope=OTHER_SCOPE), {})
    derived = rule_assertion(scope=SCOPE, premise_id="p1")
    with pytest.raises(ValueError):
        validate_assertion(derived, {}, {"p1": source_assertion(assertion_id="p1", scope=ScopeKey(1, "kb-b"))})


def test_subject_and_object_refs_must_share_assertion_scope():
    with pytest.raises(ValueError):
        make_assertion(subject=EntityRef(ENTITY_1, OTHER_SCOPE))
    with pytest.raises(ValueError):
        make_assertion(object_id=EntityRef(ENTITY_2, OTHER_SCOPE))


def test_records_require_scope_keys_and_derivation_scope_matches():
    evidence = Evidence("src-1", "doc-1", 1, "chunk-1", "opaque-hash", "claimed", None, None)
    for record in (
        lambda: Entity(ENTITY_1, None, "component"),
        lambda: EntityRef(ENTITY_1, None),
        lambda: ScopedEvidence(None, evidence),
        lambda: make_assertion(scope=None),
        lambda: Derivation(scope=None, conclusion_id="r1", premise_ids=("p1",),
                           rule_id="rule", rule_version="v1"),
    ):
        with pytest.raises(ValueError):
            record()

    derivation = Derivation(scope=OTHER_SCOPE, conclusion_id="r1", premise_ids=("p1",),
                            rule_id="rule", rule_version="v1")
    with pytest.raises(ValueError):
        make_assertion(assertion_id="r1", kind=AssertionKind.RULE_DERIVED,
                       evidence_ids=(), derivation=derivation)


def test_source_assertion_requires_resolvable_evidence():
    with pytest.raises(ValueError):
        validate_assertion(source_assertion(evidence_id="missing"), {}, {})


def test_dangling_and_cyclic_premises_are_rejected():
    dangling = rule_assertion(premise_id="missing")
    with pytest.raises(ValueError):
        validate_assertion(dangling, {}, {})
    cyclic = rule_assertion(assertion_id="a1", premise_id="a1")
    with pytest.raises(ValueError):
        validate_assertion(cyclic, {}, {"a1": cyclic})

def test_multi_node_derivation_cycle_is_rejected():
    first = rule_assertion(assertion_id="a1", premise_id="a2")
    second = rule_assertion(assertion_id="a2", premise_id="a1")
    with pytest.raises(ValueError):
        validate_assertion(first, {}, {"a1": first, "a2": second})


def test_rule_and_model_derivations_require_matching_version_pairs():
    premise = source_assertion(assertion_id="p1")
    rule = rule_assertion(assertion_id="r1", premise_id="p1")
    validate_assertion(rule, evidence_map("src-1"), {"p1": premise})
    model_derivation = Derivation(scope=SCOPE, conclusion_id="m1", premise_ids=("p1",),
                                 model_version="model-v1", prompt_version="prompt-v1")
    model = make_assertion(assertion_id="m1", kind=AssertionKind.MODEL_INFERRED,
                           evidence_ids=(), derivation=model_derivation)
    validate_assertion(model, evidence_map("src-1"), {"p1": premise})
    with pytest.raises(ValueError):
        make_assertion(kind=AssertionKind.RULE_DERIVED, evidence_ids=(),
                       derivation=model_derivation)


def test_validity_window_requires_aware_increasing_times():
    with pytest.raises(ValueError):
        make_assertion(valid_from=datetime(2026, 1, 2))
    with pytest.raises(ValueError):
        make_assertion(valid_from=datetime(2026, 1, 2, tzinfo=timezone.utc),
                       valid_until=datetime(2026, 1, 1, tzinfo=timezone.utc))


def test_validation_checks_map_keys_and_reachable_support_closure():
    assertion = source_assertion(evidence_id="src-1")
    with pytest.raises(ValueError):
        validate_assertion(assertion, {"wrong-key": evidence_map("src-1")["src-1"]}, {})

    premise = source_assertion(assertion_id="p1", evidence_id="src-1")
    derived = rule_assertion(premise_id="p1")
    with pytest.raises(ValueError):
        validate_assertion(derived, {}, {"p1": premise})
