"""Regression contract for the bounded Semantica 0.6.8 capability probe."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).parent
PROBE = ROOT / "capability_probe.py"
RUNTIME_PROBE = ROOT / "runtime_probe.py"
FIXTURE = ROOT / "fixtures" / "dependency_chain.json"


def test_probe_writes_machine_readable_evidence(tmp_path: Path) -> None:
    output = tmp_path / "evidence.json"
    result = subprocess.run(
        [sys.executable, str(PROBE), "--fixture", str(FIXTURE), "--output", str(output)],
        cwd=ROOT.parents[1],
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 0, result.stderr
    payload = json.loads(output.read_text())
    assert payload["schema_version"] == 1
    assert payload["package"]["requested_version"] == "0.6.8"
    assert set(payload["capabilities"]) == {
        "rule_reasoner",
        "context_authorized_subgraph_bridge",
        "graph_reasoner_provider_injection",
    }
    assert {
        item["classification"] for item in payload["capabilities"].values()
    } == {"not-tested"}
    assert payload["public_import"] is None


def test_probe_rejects_missing_explicit_wheel(tmp_path: Path) -> None:
    result = subprocess.run(
        [sys.executable, str(PROBE), "--fixture", str(FIXTURE), "--wheel", str(tmp_path / "missing.whl"), "--output", str(tmp_path / "out.json")],
        check=False, capture_output=True, text=True,
    )
    assert result.returncode != 0
    assert "wheel does not exist" in result.stderr


def test_probe_rejects_bad_wheel_hash_before_execution(tmp_path: Path) -> None:
    wheel = tmp_path / "bad.whl"
    wheel.write_bytes(b"not the Semantica release wheel")
    result = subprocess.run(
        [sys.executable, str(PROBE), "--fixture", str(FIXTURE), "--wheel", str(wheel), "--output", str(tmp_path / "out.json")],
        check=False, capture_output=True, text=True,
    )
    assert result.returncode != 0
    assert "wheel SHA-256 mismatch" in result.stderr


def test_runtime_probe_rejects_bad_wheel_hash(tmp_path: Path) -> None:
    wheel = tmp_path / "bad.whl"
    wheel.write_bytes(b"not the Semantica release wheel")
    result = subprocess.run([sys.executable, str(RUNTIME_PROBE), "--wheel", str(wheel), "--output", str(tmp_path / "out.json")], check=False, capture_output=True, text=True)
    assert result.returncode != 0
    assert "wheel SHA-256 mismatch" in result.stderr
