"""Real-storage contract for the V02 disposable Neo4j bridge."""

from __future__ import annotations

from pathlib import Path

from bridge_probe import build_probe_graph, probe_roundtrip


FIXTURE = Path(__file__).parent / "fixtures" / "controlled_graph.json"


def test_persistent_bridge_keeps_source_ids() -> None:
    result = probe_roundtrip(FIXTURE)

    assert result["restart_verified"] is True
    assert set(result["evidence_ids"]) == {"e-d1", "e-d2"}
    assert result["actual_backend"] == "neo4j-dedicated-instance"
    assert result["engine_version"]
    assert result["client_reopened"] is True
    assert result["storage_rows"] == 3
    assert "e-hidden" not in result["evidence_ids"]
    assert "隐藏组件" not in str(result["result"])


def test_authorized_projection_preserves_assertion_revision_and_quote() -> None:
    rows = [
        {
            "source": "构建",
            "target": "发布",
            "type": "depends_on",
            "assertion_id": "a-d1-build-release",
            "document_id": "d1",
            "revision": "r1",
            "evidence_ids": ["e-d1"],
            "quote": "构建依赖发布。",
        }
    ]

    graph = build_probe_graph(rows)

    assert graph["relationships"][0]["properties"] == {
        "assertion_id": "a-d1-build-release",
        "document_id": "d1",
        "revision": "r1",
        "evidence_ids": ["e-d1"],
        "quote": "构建依赖发布。",
    }
