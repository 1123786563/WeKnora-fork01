"""V02 registered-rule and loopback-model reasoning contracts."""

from __future__ import annotations

from bridge_probe import probe_model, probe_rule


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


def test_registered_rule_reports_conflicting_fact_without_deriving_it() -> None:
    result = probe_rule(
        ["depends_on(a,b)", "depends_on(b,c)", "not_depends_on(a,b)"],
        [RULE],
    )

    assert result["status"] == "conflicting_evidence"
    assert result["conflicts"] == ["not_depends_on(a,b)"]
    assert "not_depends_on(a,c)" not in result["conclusions"]


def test_registered_graph_reasoner_uses_loopback_gateway_and_excludes_hidden_source() -> None:
    graph = {
        "entities": [
            {"id": "构建", "type": "步骤", "properties": {}},
            {"id": "发布", "type": "步骤", "properties": {}},
        ],
        "relationships": [
            {
                "source": "构建",
                "target": "发布",
                "type": "depends_on",
                "properties": {"evidence_ids": ["e-d1"], "quote": "构建依赖发布。"},
            }
        ],
    }

    result = probe_model(graph, "构建依赖什么？")

    assert result["actual_backend"] == "semantica-graphreasoner-via-go-loopback"
    assert result["availability"] == "live-local-provider"
    assert result["evidence_ids"] == ["e-d1"]
    assert result["raw_usage"]["total_tokens"] == (
        result["raw_usage"]["prompt_tokens"] + result["raw_usage"]["completion_tokens"]
    )
    assert "隐藏" not in result["result"]
