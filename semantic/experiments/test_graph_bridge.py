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
    assert all(item["start_char"] >= 0 and item["end_char"] > item["start_char"] for item in fixture["assertions"])
    assert all(item["rule_version"] == "source@v1" for item in fixture["assertions"])


def test_fresh_reader_process_recovers_persisted_provenance() -> None:
    from run_v02 import from_environment, run_writer_then_reader

    result = run_writer_then_reader(from_environment(), FIXTURE)

    assert result["writer_pid"] != result["reader_pid"]
    assert result["restart_verified"] is True
    assert result["assertion_ids"] == {"a-d1", "a-d2"}
    assert result["evidence_ids"] == {"e-d1", "e-d2"}
    assert result["storage_returned_ids"] == {"a-d1", "a-d2"}
    assert result["foreign_ids"] == set()


def test_reader_only_process_recovers_seeded_fixture() -> None:
    from run_v02 import from_environment, reader_only, seed_writer

    config = from_environment()
    writer = seed_writer(config, FIXTURE)
    reader = reader_only(config, FIXTURE)

    assert writer["writer_pid"] != reader["reader_pid"]
    assert reader["storage_returned_ids"] == {"a-d1", "a-d2"}
    assert reader["foreign_ids"] == set()


def test_evidence_record_serializes_logical_sets() -> None:
    from run_v02 import evidence_record

    record = evidence_record({"assertion_ids": {"a-d2", "a-d1"}, "evidence_ids": {"e-d2", "e-d1"}})

    assert record == {"assertion_ids": ["a-d1", "a-d2"], "evidence_ids": ["e-d1", "e-d2"]}


def test_evidence_metadata_excludes_topology_password() -> None:
    from bridge_probe import TopologyConfig
    from run_v02 import evidence_metadata

    metadata = evidence_metadata(TopologyConfig("bolt://127.0.0.1:17687", "neo4j", "not-for-evidence", "neo4j", "dedicated"))

    assert metadata["topology"] == "dedicated"
    assert metadata["endpoint_alias"] == "loopback:17687"
    assert metadata["account_identity"] == "neo4j"
    assert metadata["v01_lock_sha256"] == "c643ce123490c93f56ed95e9d6501bbd90daf8b2cdc74b183067dccd63864c54"
    assert metadata["v02_lock_sha256"] != metadata["v01_lock_sha256"]
    assert len(metadata["commit_sha"]) == 40
    assert metadata["platform"]
    assert "password" not in metadata
