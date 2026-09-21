"""Installed-distribution contract for the Semantica V01 candidate."""

from __future__ import annotations

from hashlib import sha256
from importlib.metadata import version
from pathlib import Path
from types import SimpleNamespace

import pytest


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


def test_verifier_rejects_changed_signature() -> None:
    from verify_version import _record_capabilities

    def observed(unexpected: str) -> None:
        return None

    records, failures = _record_capabilities(
        {"Changed": ("fake.module", "observed", "(expected: str)")},
        importer=lambda _: SimpleNamespace(observed=observed),
    )

    assert records["Changed"]["status"] == "unavailable"
    assert "expected (expected: str)" in records["Changed"]["reason"]
    assert failures


def test_verifier_rejects_lock_entry_without_frozen_wheel(tmp_path: Path) -> None:
    from verify_version import _locked_semantica

    lock = tmp_path / "uv.lock"
    lock.write_text('[[package]]\nname = "semantica"\nversion = "0.6.8"\nwheels = []\n')

    with pytest.raises(ValueError, match="frozen wheel hash"):
        _locked_semantica(lock)


def test_evidence_output_must_stay_under_repository_root(tmp_path: Path) -> None:
    from verify_version import evidence_output_path

    assert evidence_output_path("docs/plans/semantica/evidence/2026-09-20/test.json").name == "test.json"
    with pytest.raises(ValueError, match="evidence root"):
        evidence_output_path(tmp_path / "escaped.json")
