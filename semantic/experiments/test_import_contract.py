"""V01 public-import and machine-readable evidence contract."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).parent
VERIFY = ROOT / "verify_version.py"
WHEEL = Path("/tmp/semantica-0.6.8-py3-none-any.whl")


def test_required_import_contract() -> None:
    from semantica.semantic_extract import NERExtractor, RelationExtractor
    from semantica.context import ContextGraph, ContextRetriever
    from semantica.reasoning import Reasoner, GraphReasoner
    from semantica.graph_store import GraphStore

    assert all(
        callable(candidate)
        for candidate in (
            NERExtractor,
            RelationExtractor,
            ContextGraph,
            ContextRetriever,
            Reasoner,
            GraphReasoner,
            GraphStore,
        )
    )


def test_minimal_profile_includes_the_neo4j_driver() -> None:
    from neo4j import GraphDatabase

    assert callable(GraphDatabase.driver)


def test_verify_version_records_a_structured_hash_failure(tmp_path: Path) -> None:
    bad_wheel = tmp_path / "bad.whl"
    bad_wheel.write_bytes(b"not the frozen Semantica wheel")
    output = tmp_path / "evidence.json"

    result = subprocess.run(
        [sys.executable, str(VERIFY), "--wheel", str(bad_wheel), "--output", str(output)],
        cwd=ROOT.parents[1],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode != 0
    payload = json.loads(output.read_text())
    assert payload["schema_version"] == 1
    assert payload["status"] == "failed"
    assert payload["failure"]["kind"] == "wheel-sha256-mismatch"
    assert payload["command"]["exit_code"] != 0


def test_verify_version_records_required_signatures(tmp_path: Path) -> None:
    output = tmp_path / "evidence.json"
    result = subprocess.run(
        [sys.executable, str(VERIFY), "--wheel", str(WHEEL), "--output", str(output)],
        cwd=ROOT.parents[1],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    payload = json.loads(output.read_text())
    assert payload["schema_version"] == 1
    assert payload["status"] == "passed"
    assert payload["semantica_version"] == "0.6.8"
    assert payload["wheel_sha256"] == "0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7"
    assert set(payload["capabilities"]) == {
        "NERExtractor",
        "RelationExtractor",
        "ContextGraph",
        "ContextRetriever",
        "Reasoner",
        "GraphReasoner",
        "GraphStore",
    }
    assert all(item["status"] == "passed" and item["signature"] for item in payload["capabilities"].values())


def test_verify_version_accepts_an_explicit_wheel_environment_variable(tmp_path: Path) -> None:
    output = tmp_path / "evidence.json"
    result = subprocess.run(
        [sys.executable, str(VERIFY), "--output", str(output)],
        cwd=ROOT.parents[1],
        capture_output=True,
        text=True,
        check=False,
        env={**os.environ, "SEMANTICA_WHEEL": str(WHEEL)},
    )

    assert result.returncode == 0, result.stderr
    payload = json.loads(output.read_text())
    assert payload["wheel_path"] == str(WHEEL)
    assert payload["lock_wheel_sha256"] == payload["wheel_sha256"]
