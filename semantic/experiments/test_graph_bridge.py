"""Real-storage contract for the V02 disposable Neo4j bridge."""

from __future__ import annotations

from pathlib import Path

import subprocess

import pytest

import bridge_probe
from bridge_probe import _OwnedNeo4j, build_probe_graph, probe_roundtrip


FIXTURE = Path(__file__).parent / "fixtures" / "controlled_graph.json"


def test_persistent_bridge_keeps_source_ids() -> None:
    result = probe_roundtrip(FIXTURE)

    assert result["restart_verified"] is True
    assert set(result["evidence_ids"]) == {"e-d1", "e-d2"}
    assert result["actual_backend"] == "neo4j-dedicated-instance"
    assert result["engine_version"]
    assert result["client_reopened"] is True
    assert result["storage_rows"] == 3
    assert result["run_nonce"]
    assert result["resource_cleanup_verified"] is True
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


def test_create_rolls_back_verified_run_resources_when_first_operational_inspect_fails(monkeypatch: pytest.MonkeyPatch) -> None:
    calls: list[list[str]] = []

    def run(args: list[str], **_: object) -> subprocess.CompletedProcess[str]:
        calls.append(args)
        if args[:3] == ["docker", "run", "-d"]:
            return subprocess.CompletedProcess(args, 0, "owned-container\n", "")
        return subprocess.CompletedProcess(args, 0, "", "")

    monkeypatch.setattr(bridge_probe.subprocess, "run", run)
    monkeypatch.setattr(bridge_probe, "_inspect_owned_identity", lambda *_: {"Id": "owned-container", "Config": {"Labels": {}}})
    monkeypatch.setattr(bridge_probe, "_inspect_owned_volume", lambda *_: {"Labels": {}})
    monkeypatch.setattr(bridge_probe, "_verify_owned_mutation", lambda *_: (_ for _ in ()).throw(RuntimeError("initial inspect failed")))

    with pytest.raises(RuntimeError, match="initial inspect failed"):
        _OwnedNeo4j.create()

    assert ["docker", "rm", "-f", "owned-container"] in calls
    assert any(call[:3] == ["docker", "volume", "rm"] for call in calls)


def test_cleanup_removes_stopped_verified_resource_and_never_unowned_resource(monkeypatch: pytest.MonkeyPatch) -> None:
    calls: list[list[str]] = []
    owned = _OwnedNeo4j("nonce", "owned-container", "owned-volume", 17687)

    def run(args: list[str], **_: object) -> subprocess.CompletedProcess[str]:
        calls.append(args)
        return subprocess.CompletedProcess(args, 0, "", "")

    monkeypatch.setattr(bridge_probe.subprocess, "run", run)
    monkeypatch.setattr(bridge_probe, "_inspect_owned_identity", lambda *_: {"State": {"Running": False}})
    monkeypatch.setattr(bridge_probe, "_inspect_owned_volume", lambda *_: {"Labels": {}})
    assert owned.cleanup() == []
    assert ["docker", "rm", "-f", "owned-container"] in calls
    assert ["docker", "volume", "rm", "owned-volume"] in calls

    calls.clear()
    monkeypatch.setattr(bridge_probe, "_inspect_owned_identity", lambda *_: (_ for _ in ()).throw(RuntimeError("unowned")))
    assert owned.cleanup()[0]["resource"] == "container"
    assert ["docker", "rm", "-f", "owned-container"] not in calls
    assert ["docker", "volume", "rm", "owned-volume"] in calls


def test_restart_failure_can_be_reported_without_masking_owned_cleanup(monkeypatch: pytest.MonkeyPatch) -> None:
    owned = _OwnedNeo4j("nonce", "owned-container", "owned-volume", 17687)
    monkeypatch.setattr(_OwnedNeo4j, "verify", lambda *_: None)
    monkeypatch.setattr(bridge_probe.subprocess, "run", lambda args, **_: subprocess.CompletedProcess(args, 1, "", "restart failed"))

    with pytest.raises(RuntimeError, match="restart failed"):
        owned.restart()


def test_cleanup_returns_structured_container_and_volume_failures(monkeypatch: pytest.MonkeyPatch) -> None:
    owned = _OwnedNeo4j("nonce", "owned-container", "owned-volume", 17687)
    monkeypatch.setattr(bridge_probe, "_inspect_owned_identity", lambda *_: {})
    monkeypatch.setattr(bridge_probe, "_inspect_owned_volume", lambda *_: {})

    def run(args: list[str], **_: object) -> subprocess.CompletedProcess[str]:
        raise subprocess.CalledProcessError(1, args, stderr="remove failed")

    monkeypatch.setattr(bridge_probe.subprocess, "run", run)
    assert owned.cleanup() == [
        {"resource": "container", "error": "Command '['docker', 'rm', '-f', 'owned-container']' returned non-zero exit status 1."},
        {"resource": "volume", "error": "Command '['docker', 'volume', 'rm', 'owned-volume']' returned non-zero exit status 1."},
    ]
