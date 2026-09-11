"""V02 reasoning bridge experiments: registered rules and model inference.

Rule experiments prove the upstream Reasoner derives a conclusion only when
every premise is present and that each derivation carries its premises and
rule reference. The model experiment proves the probe fails closed — no call
is made and the capability stays unverified — unless an approved model entry
is configured in the experiment environment.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from bridge_probe import probe_model, probe_rule  # noqa: E402

CONTROLS_RULE = "controls(x,y)&controls(y,z)->controls(x,z)"
MINI_GRAPH = {
    "entities": [
        {"id": "ent-jia", "name": "甲公司", "type": "Company"},
        {"id": "ent-yi", "name": "乙公司", "type": "Company"},
        {"id": "ent-bing", "name": "丙公司", "type": "Company"},
    ],
    "relationships": [
        {"source": "ent-jia", "target": "ent-yi", "type": "controls"},
        {"source": "ent-yi", "target": "ent-bing", "type": "controls"},
    ],
}


def test_rule_requires_both_premises():
    result = probe_rule(["controls(a,b)"], [CONTROLS_RULE])
    assert "controls(a,c)" not in result["conclusions"]


def test_rule_derives_transitive_conclusion_with_both_premises():
    result = probe_rule(["controls(a,b)", "controls(b,c)"], [CONTROLS_RULE])
    assert "controls(a,c)" in result["conclusions"]


def test_derivation_keeps_premises_and_rule_reference():
    result = probe_rule(["controls(a,b)", "controls(b,c)"], [CONTROLS_RULE])
    derived = [d for d in result["derivations"] if d["conclusion"] == "controls(a,c)"]
    assert derived, "transitive conclusion must carry derivation detail"
    assert set(derived[0]["premises"]) == {"controls(a,b)", "controls(b,c)"}
    assert derived[0]["rule_id"]


def test_rule_on_chinese_authorized_facts():
    full = probe_rule(
        ["controls(甲公司,乙公司)", "controls(乙公司,丙公司)"], [CONTROLS_RULE])
    assert "controls(甲公司,丙公司)" in full["conclusions"]
    partial = probe_rule(["controls(甲公司,乙公司)"], [CONTROLS_RULE])
    assert "controls(甲公司,丙公司)" not in partial["conclusions"]


def test_model_probe_reports_failure_for_broken_approved_entry(monkeypatch):
    # Upstream GraphReasoner swallows provider failures into "Error: ..." strings
    # instead of raising. A configured-but-broken entry must surface as an
    # explicit failure with model_call_made False — never as "completed".
    monkeypatch.setenv("SEMANTICA_EXPERIMENT_MODEL_PROVIDER", "nonexistent-provider-xyz")
    monkeypatch.setenv("SEMANTICA_EXPERIMENT_MODEL_API_KEY", "dummy-key-for-test")
    monkeypatch.delenv("SEMANTICA_EXPERIMENT_MODEL_NAME", raising=False)
    result = probe_model(MINI_GRAPH, "甲公司是否控制丙公司？")
    assert result["status"] == "failed"
    assert result["model_call_made"] is False
    assert result["reason"]
    assert result["usage"] is None
    assert result["result"] is None


def test_model_probe_reports_failure_when_generation_errors(monkeypatch):
    # Upstream swallows generate() failures into 'Error ...' strings; the probe
    # must report failure with model_call_made None (call outcome unknown).
    monkeypatch.setenv("SEMANTICA_EXPERIMENT_MODEL_PROVIDER", "openai")
    monkeypatch.setenv("SEMANTICA_EXPERIMENT_MODEL_API_KEY", "dummy-key-for-test")
    monkeypatch.delenv("SEMANTICA_EXPERIMENT_MODEL_NAME", raising=False)

    import semantica.reasoning as reasoning_module

    class _StubProvider:
        last_usage = None

    class _StubGraphReasoner:
        def __init__(self, config=None, **kwargs):
            self.provider = _StubProvider()

        def reason(self, graph, query, **options):
            return "Error during reasoning: stubbed failure"

    monkeypatch.setattr(reasoning_module, "GraphReasoner", _StubGraphReasoner)
    result = probe_model(MINI_GRAPH, "甲公司是否控制丙公司？")
    assert result["status"] == "failed"
    assert result["model_call_made"] is None
    assert result["model_call_made_note"]
    assert result["result"] is None
    assert result["usage"] is None


def test_model_probe_fails_closed_without_approved_entry(monkeypatch):
    for var in (
        "SEMANTICA_EXPERIMENT_MODEL_PROVIDER",
        "SEMANTICA_EXPERIMENT_MODEL_NAME",
        "SEMANTICA_EXPERIMENT_MODEL_API_KEY",
        "OPENAI_API_KEY",
        "ANTHROPIC_API_KEY",
    ):
        monkeypatch.delenv(var, raising=False)
    result = probe_model(MINI_GRAPH, "甲公司是否控制丙公司？")
    assert result["status"] == "unverified"
    assert result["model_call_made"] is False
    assert result["reason"]
    assert result["usage"] is None
    assert result["actual_backend"] == "none"
