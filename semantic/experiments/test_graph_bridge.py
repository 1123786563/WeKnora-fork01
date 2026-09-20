from __future__ import annotations

from pathlib import Path


FIXTURE = Path(__file__).parent / "fixtures" / "v02-controlled-graph.json"


def test_scope_query_never_returns_foreign_distractor() -> None:
    from bridge_probe import Scope, scope_read_query

    query, parameters = scope_read_query(
        Scope(tenant_id="T1", kb_id="K1", document_id="D1", revision=1, generation="g1")
    )

    assert "tenant_id: $tenant_id" in query
    assert "kb_id: $kb_id" in query
    assert "document_id: $document_id" in query
    assert "revision: $revision" in query
    assert "generation: $generation" in query
    assert parameters == {
        "tenant_id": "T1",
        "kb_id": "K1",
        "document_id": "D1",
        "revision": 1,
        "generation": "g1",
        "record_kind": "assertion",
    }


def test_fixture_declares_target_and_foreign_records() -> None:
    from bridge_probe import load_fixture

    fixture = load_fixture(FIXTURE)

    assert {item["semantic_id"] for item in fixture["assertions"]} == {"a-d1", "a-d2", "a-foreign"}
    assert {item["semantic_id"] for item in fixture["assertions"] if item["tenant_id"] == "T1"} == {"a-d1", "a-d2"}
