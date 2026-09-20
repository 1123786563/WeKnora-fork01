"""V02 registered-rule and loopback-model reasoning contracts."""

from __future__ import annotations

import pytest
from pathlib import Path

from bridge_probe import probe_model, probe_roundtrip, probe_rule, validate_gateway_url


FIXTURE = Path(__file__).parent / "fixtures" / "controlled_graph.json"


RULE = "depends_on(x,y)&depends_on(y,z)->indirectly_depends_on(x,z)"


def test_registered_depends_on_rule_derives_only_with_both_premises() -> None:
    result = probe_rule(
        ["depends_on(a,b)", "depends_on(b,c)"],
        [RULE],
    )

    assert "indirectly_depends_on(a,c)" in result["conclusions"]
    assert result["actual_backend"] == "semantica-reasoner-0.6.8"
    assert result["engine_version"] == "0.6.8"


def test_registered_depends_on_rule_requires_both_premises() -> None:
    result = probe_rule(["depends_on(a,b)"], [RULE])

    assert "indirectly_depends_on(a,c)" not in result["conclusions"]
    assert result["conclusions"] == []


def test_registered_rule_marks_upstream_conflict_capability_unavailable_without_fabricating_it() -> None:
    result = probe_rule(
        ["depends_on(a,b)", "depends_on(b,c)", "not_depends_on(a,b)"],
        [RULE],
    )

    assert result["status"] == "unavailable"
    assert result["conflict_capability"]["classification"] == "actual-runtime"
    assert result["conflict_capability"]["reason"]
    assert result["result"]["inferences"] == [
        {"conclusion": "indirectly_depends_on(a,c)", "premises": ["depends_on(a,b)", "depends_on(b,c)"]}
    ]
    assert "not_depends_on(a,c)" not in result["conclusions"]


@pytest.mark.parametrize("url", [
    "http://127.0.0.1:18092@evil.example",
    "http://localhost:18092",
    "http://127.0.0.1:18093",
    "http://[::1]:18092",
    "https://127.0.0.1:18092",
    "http://127.0.0.1:18092?redirect=http://evil.example",
])
def test_gateway_url_rejects_every_non_exact_loopback_form(url: str) -> None:
    with pytest.raises(ValueError):
        validate_gateway_url(url)


def test_registered_graph_reasoner_receives_persisted_authorized_projection_only() -> None:
    graph = probe_roundtrip(FIXTURE)["result"]

    result = probe_model(graph, "构建依赖什么？")

    assert result["actual_backend"] == "semantica-graphreasoner-via-go-loopback"
    assert result["availability"] == "live-local-provider"
    assert result["evidence_ids"] == ["e-d1", "e-d2"]
    assert result["raw_usage"]["total_tokens"] == (
        result["raw_usage"]["prompt_tokens"] + result["raw_usage"]["completion_tokens"]
    )
    assert "隐藏" not in result["result"]
    assert result["prompt_provenance"] == {
        "allowed_assertion_ids": ["a-d1-build-release", "a-d2-release-deploy"],
        "allowed_revisions": ["r1", "r3"],
        "allowed_evidence_ids": ["e-d1", "e-d2"],
        "allowed_quotes": ["发布完成后才能部署。", "构建依赖发布。"],
        "hidden_absent": True,
    }
    assert result["prompt_sha256"]
