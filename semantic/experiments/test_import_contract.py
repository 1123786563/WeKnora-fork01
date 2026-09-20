"""Installed-distribution contract for the Semantica V01 candidate."""

from __future__ import annotations

from hashlib import sha256
from importlib.metadata import version
from pathlib import Path


def test_required_import_contract() -> None:
    from semantica.context import ContextGraph, ContextRetriever
    from semantica.graph_store import GraphStore
    from semantica.reasoning import GraphReasoner, Reasoner
    from semantica.semantic_extract import NERExtractor, RelationExtractor

    assert all(
        callable(item)
        for item in (
            NERExtractor,
            RelationExtractor,
            ContextGraph,
            ContextRetriever,
            Reasoner,
            GraphReasoner,
            GraphStore,
        )
    )


def test_installed_distribution_is_the_frozen_candidate() -> None:
    assert version("semantica") == "0.6.8"


def test_verifier_records_runtime_contract(tmp_path: Path) -> None:
    from verify_version import build_evidence

    evidence = build_evidence(command=["verify_version.py", "--output", "out.json"])

    assert evidence["schema_version"] == 1
    assert evidence["distribution"]["version"] == "0.6.8"
    assert evidence["lock_hash"] == sha256((Path(__file__).parent / "uv.lock").read_bytes()).hexdigest()
    assert evidence["command"] == ["verify_version.py", "--output", "out.json"]
    assert evidence["exit_code"] == 0
    assert evidence["evidence_layer"] == "actual-runtime"
    assert set(evidence["capabilities"]) == {
        "NERExtractor",
        "RelationExtractor",
        "ContextGraph",
        "ContextRetriever",
        "Reasoner",
        "GraphReasoner",
        "GraphStore",
    }
    assert all(
        capability["status"] == "available"
        and capability["signature"]
        and capability["evidence_layer"] == "actual-runtime"
        for capability in evidence["capabilities"].values()
    )
