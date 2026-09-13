"""C01 Python contract golden tests.

Pin the shared domain DTOs against the wire schema and the cross-language
fixture: optional spans stay None (never zero), uint64 values survive wire
round-trips without float loss, Chinese text survives, unknown enum values
map to an explicit unspecified state (never a default success), and the JSON
public boundary encodes 64-bit integers as decimal strings.
"""

import dataclasses
import json
from pathlib import Path

import pytest

from semantic_service import contracts

FIXTURE = Path(__file__).parent / "fixtures" / "contract-v1.json"
MAX_UINT64 = 18446744073709551615


def _load():
    return json.loads(FIXTURE.read_text(encoding="utf-8"))


def test_optional_evidence_span_is_not_zero():
    evidence = contracts.Evidence(
        evidence_id="e1", document_id="d1", revision=1,
        chunk_id="c1", content_hash="h", quote="甲",
        start_char=None, end_char=None)
    assert evidence.start_char is None
    assert evidence.revision == 1


def test_frozen_dto_rejects_mutation():
    scope = contracts.ScopeKey(tenant_id=1, kb_id="kb")
    with pytest.raises(dataclasses.FrozenInstanceError):
        scope.tenant_id = 2


def test_mutable_collections_are_copied_on_construction():
    evidence_ids = ["e1"]
    assertion = contracts.Assertion(
        assertion_id="a1", scope=contracts.ScopeKey(tenant_id=1, kb_id="kb"),
        subject_id="s", predicate="controls", kind="source",
        evidence_ids=evidence_ids, premise_ids=[])
    evidence_ids.append("e2")
    assert list(assertion.evidence_ids) == ["e1"]


def test_json_boundary_encodes_uint64_as_decimal_string():
    data = _load()
    scope = contracts.scope_from_json(data["scope"])
    assert scope.tenant_id == MAX_UINT64
    encoded = contracts.scope_to_json(scope)
    assert encoded["tenant_id"] == "18446744073709551615"

    evidence = contracts.evidence_from_json(data["evidence_with_span"])
    encoded_evidence = contracts.evidence_to_json(evidence)
    assert encoded_evidence["start_char"] == "3"
    assert encoded_evidence["revision"] == "2"
    empty_span = contracts.evidence_to_json(
        contracts.evidence_from_json(data["evidence_without_span"]))
    assert empty_span["start_char"] is None


def test_fixture_roundtrips_through_wire():
    data = _load()
    for name, build, to_wire, from_wire in (
        ("scope", contracts.scope_from_json, contracts.scope_to_wire, contracts.scope_from_wire),
        ("document_revision", contracts.document_revision_from_json, contracts.document_revision_to_wire, contracts.document_revision_from_wire),
        ("evidence_with_span", contracts.evidence_from_json, contracts.evidence_to_wire, contracts.evidence_from_wire),
        ("evidence_without_span", contracts.evidence_from_json, contracts.evidence_to_wire, contracts.evidence_from_wire),
        ("assertion", contracts.assertion_from_json, contracts.assertion_to_wire, contracts.assertion_from_wire),
        ("access_scope", contracts.access_scope_from_json, contracts.access_scope_to_wire, contracts.access_scope_from_wire),
        ("query_limits", contracts.query_limits_from_json, contracts.query_limits_to_wire, contracts.query_limits_from_wire),
        ("search_request", contracts.search_request_from_json, contracts.search_request_to_wire, contracts.search_request_from_wire),
        ("operation", contracts.operation_from_json, contracts.operation_to_wire, contracts.operation_from_wire),
        ("reason_response", contracts.reason_response_from_json, contracts.reason_response_to_wire, contracts.reason_response_from_wire),
        ("chunk", contracts.chunk_from_json, contracts.chunk_to_wire, contracts.chunk_from_wire),
        ("apply_request", contracts.apply_request_from_json, contracts.apply_request_to_wire, contracts.apply_request_from_wire),
        ("apply_request_with_manifest", contracts.apply_request_from_json, contracts.apply_request_to_wire, contracts.apply_request_from_wire),
        ("search_response", contracts.search_response_from_json, contracts.search_response_to_wire, contracts.search_response_from_wire),
        ("capabilities", contracts.capabilities_from_json, contracts.capabilities_to_wire, contracts.capabilities_from_wire),
        ("reason_request", contracts.reason_request_from_json, contracts.reason_request_to_wire, contracts.reason_request_from_wire),
    ):
        original = build(data[name])
        wire = to_wire(original)
        parsed = type(wire).FromString(wire.SerializeToString())
        restored = from_wire(parsed)
        assert restored == original, f"{name} did not survive the wire round-trip"


def test_max_uint64_survives_wire_without_float_loss():
    scope = contracts.ScopeKey(tenant_id=MAX_UINT64, kb_id="kb")
    wire = contracts.scope_to_wire(scope)
    parsed = type(wire).FromString(wire.SerializeToString())
    assert contracts.scope_from_wire(parsed).tenant_id == MAX_UINT64


def test_unknown_enum_is_not_defaulted_to_success():
    from semantic_service.proto import semantic_pb2 as pb

    wire = pb.Assertion(
        assertion_id="a1", subject_id="s", predicate="controls", kind=99,
        evidence_ids=["e1"])
    mapped = contracts.assertion_from_wire(wire)
    assert mapped.kind == "unspecified"


def test_unknown_domain_enums_raise_instead_of_encoding():
    scope = contracts.ScopeKey(tenant_id=1, kb_id="kb")
    with pytest.raises(ValueError):
        contracts.assertion_to_wire(contracts.Assertion(
            assertion_id="a", scope=scope, subject_id="s", predicate="p",
            kind="bogus", evidence_ids=[], premise_ids=[]))
    with pytest.raises(ValueError):
        contracts.access_scope_to_wire(contracts.AccessScope(
            scope=scope, subject_id="s", scope_ref="r", scope_hash="h",
            permission_epoch=1, expires_at="t", audience="a", purpose="bogus"))
    with pytest.raises(ValueError):
        contracts.reason_request_to_wire(contracts.ReasonRequest(
            search=contracts.SearchRequest(query_id="q", query="q"), reasoning_mode="bogus"))
    with pytest.raises(ValueError):
        contracts.reason_response_to_wire(contracts.ReasonResponse(
            retrieval=contracts.SearchResponse(query_id="q", generation="g", mode="m"),
            status="supported", conclusion="c", conclusion_kind="bogus"))
    with pytest.raises(ValueError):
        contracts.operation_to_wire(contracts.Operation(
            operation_id="o", scope=scope, document_id="d", revision=1, state="bogus"))


def test_non_null_error_code_roundtrips():
    data = _load()
    operation = contracts.operation_from_json(data["operation_failed"])
    assert operation.error_code == "E_DOCUMENT_CONFLICT"
    wire = contracts.operation_to_wire(operation)
    parsed = type(wire).FromString(wire.SerializeToString())
    assert contracts.operation_from_wire(parsed).error_code == "E_DOCUMENT_CONFLICT"


def test_json_boundary_rejects_non_canonical_uint_strings():
    data = _load()
    scope_data = dict(data["scope"])
    scope_data["tenant_id"] = " 1"
    with pytest.raises(ValueError):
        contracts.scope_from_json(scope_data)
    hex_data = dict(data["scope"])
    hex_data["tenant_id"] = "0x1"
    with pytest.raises(ValueError):
        contracts.scope_from_json(hex_data)


def test_out_of_range_spans_fail_fast():
    with pytest.raises(ValueError):
        contracts.Evidence(
            evidence_id="e", document_id="d", revision=1, chunk_id="c",
            content_hash="h", quote="甲", start_char=2 ** 32)
    with pytest.raises(ValueError):
        contracts.Evidence(
            evidence_id="e", document_id="d", revision=1, chunk_id="c",
            content_hash="h", quote="甲", start_char=-1)
    with pytest.raises(ValueError):
        contracts.ScopeKey(tenant_id=-1, kb_id="kb")


def test_wire_enum_values_match_domain_strings():
    from semantic_service.proto import semantic_pb2 as pb

    assert contracts.assertion_from_wire(
        pb.Assertion(kind=pb.Assertion.Kind.KIND_SOURCE)).kind == "source"
    assert contracts.assertion_to_wire(
        contracts.Assertion(assertion_id="a", scope=contracts.ScopeKey(1, "kb"),
                            subject_id="s", predicate="p", kind="model",
                            evidence_ids=[], premise_ids=[])).kind == pb.Assertion.Kind.KIND_MODEL
