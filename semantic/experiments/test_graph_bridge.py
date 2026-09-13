"""V02 persistent-graph bridge experiments against a real isolated Neo4j.

Capability experiments for the frozen Semantica release (V01): a Chinese,
source-attributed fixture graph must survive a real Neo4j round-trip, be
re-read by a fresh process, be filtered down to the authorized documents, and
answer a two-hop question from the rebuilt in-memory subgraph — while the
restricted document's content never appears anywhere in the results.

The isolated Neo4j endpoint comes from the experiment environment
(SEMANTICA_EXPERIMENT_NEO4J_* variables, see bridge_probe.py). A missing
environment fails these tests explicitly; it is never skipped.

The round-trip is deterministic for a given fixture and endpoint, so the
tests share one module-scoped probe run instead of paying the write+read
cost per test.
"""

import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent))

from bridge_probe import probe_roundtrip  # noqa: E402

FIXTURE = "semantic/experiments/fixtures/controlled_graph.json"


@pytest.fixture(scope="module")
def roundtrip():
    return probe_roundtrip(FIXTURE)


def test_persistent_bridge_keeps_source_ids(roundtrip):
    result = roundtrip
    assert result["restart_verified"] is True
    assert set(result["evidence_ids"]) == {"e-d1", "e-d2"}


def test_all_fixture_documents_really_reached_neo4j(roundtrip):
    # The authorization filter is only meaningful if the restricted rows were
    # actually persisted first and filtered downstream, not never written.
    result = roundtrip
    assert set(result["persistent_document_ids"]) == {"d1", "d2", "d3", "d4"}
    assert result["persistent_edge_count"] == 4


def test_every_persisted_row_keeps_its_attribution(roundtrip):
    result = roundtrip
    assert result["attribution_fidelity_ok"] is True


def test_rebuilt_subgraph_supports_two_hop_path(roundtrip):
    result = roundtrip
    assert result["graph"]["path_jia_to_bing"] == [
        ["ent-jia", "controls", "ent-yi"],
        ["ent-yi", "controls", "ent-bing"],
    ]


def test_retriever_expands_two_hops_on_rebuilt_subgraph(roundtrip):
    result = roundtrip
    assert result["retrieval"]["hit_count"] >= 1
    assert result["retrieval"]["bing_related_to_jia"] is True


def test_restricted_document_never_leaks_into_probe_result(roundtrip):
    result = roundtrip
    blob = json.dumps(result, ensure_ascii=False)
    for secret in ("松柏", "密钥", "ent-song", "e-d4"):
        assert secret not in blob, f"restricted content leaked: {secret}"
    assert result["graph"]["visible_documents"] == ["d1", "d2"]
