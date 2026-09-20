"""Behavioral contracts for the V03 frozen-corpus evaluator."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

import evaluate
from evaluate import UsageCollector, assess_answer, build_provenance_graph, evaluate_rows, has_privacy_leak, observed_privacy_violation, retrieve_provenance_graph, score_case


def test_evidence_score_penalizes_unsupported_source() -> None:
    score = score_case({"e1"}, {"e1", "hidden"})
    assert score["precision"] == 0.5
    assert score["recall"] == 1.0


def test_evidence_score_empty_expected_is_recalled_only_when_no_evidence_is_claimed() -> None:
    assert score_case(set(), set()) == {"precision": 0.0, "recall": 1.0}
    assert score_case(set(), {"e-hidden"}) == {"precision": 0.0, "recall": 0.0}


def test_privacy_leak_is_a_hard_gate_even_when_other_scores_are_perfect() -> None:
    rows = [
        {"case_id": "allowed", "expected_evidence": ["e1"], "evidence_ids": ["e1"], "status": "completed", "latency_ms": 10, "tokens": {"total_tokens": 3}},
        {"case_id": "revoked", "expected_evidence": [], "evidence_ids": ["e-secret"], "status": "completed", "latency_ms": 20, "tokens": {"total_tokens": 4}, "privacy_violation": True},
    ]
    summary = evaluate_rows(rows)
    assert summary["source_precision"] == 0.5
    assert summary["privacy_hardgate_passed"] is False
    assert summary["quality_gate_passed"] is False


def test_empty_evidence_requires_explicit_abstention_not_a_fabricated_answer() -> None:
    fabricated = assess_answer({"unanswerable": True}, "风控服务依赖甲服务。", set())
    abstained = assess_answer({"unanswerable": True}, "证据不足", set())
    assert fabricated == {"correct": False, "unanswerable_correct": False}
    assert abstained == {"correct": True, "unanswerable_correct": True}


def test_citation_does_not_make_an_incorrect_conclusion_correct() -> None:
    assessment = assess_answer({"expected_answer_terms": ["丙服务"]}, "甲服务最终依赖乙服务。", {"e-d1"})
    assert assessment["correct"] is False


def test_conclusion_oracle_rejects_wrong_final_dependency_that_mentions_expected_entity() -> None:
    case = {"conclusion_oracle": {"must_contain": ["甲服务最终依赖丙服务"], "must_not_contain": ["甲服务最终依赖乙服务"]}}
    assert assess_answer(case, "甲服务最终依赖乙服务，因为乙服务依赖丙服务。", {"e1"})["correct"] is False


def test_summary_separates_query_phases_from_indexing() -> None:
    summary = evaluate_rows([{"status": "completed", "expected_evidence": [], "evidence_ids": [], "latency_ms": 99, "indexing_ms": 20, "query_attempts": [{"query_phase": "cold", "latency_ms": 3}, {"query_phase": "warm", "latency_ms": 2}]}])
    assert summary["cold_query_latency"] == {"available_count": 1, "p50_ms": 3.0, "p95_ms": 3.0}
    assert summary["indexing_latency"]["p95_ms"] == 20.0


def test_expected_positive_conclusion_rejects_a_negated_term_match() -> None:
    assessment = assess_answer(
        {"expected_answer_terms": ["支付服务", "依赖", "库存服务"], "expected_polarity": "positive"},
        "支付服务不依赖库存服务。",
        {"e-payment"},
    )
    assert assessment["correct"] is False


def test_observed_forbidden_reference_is_a_privacy_leak_without_answer_citation() -> None:
    assert observed_privacy_violation(
        "安全回答。",
        [{"knowledge_id": "k-secret", "match_type": "graph"}],
        [{"evidence_id": "e-secret", "runtime_id": "k-secret", "canary": "CANARY-REVOKED-71"}],
    ) is True


def test_shared_usage_collector_keeps_ner_re_and_query_attempts_from_distinct_provider_instances() -> None:
    collector = UsageCollector()
    collector.record("ner", {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5})
    collector.record("relation", {"prompt_tokens": 3, "completion_tokens": 4, "total_tokens": 7})
    collector.record("query", {"prompt_tokens": 5, "completion_tokens": 6, "total_tokens": 11})
    assert collector.total() == {"prompt_tokens": 10, "completion_tokens": 13, "total_tokens": 23}
    assert [entry["stage"] for entry in collector.attempts] == ["ner", "relation", "query"]


def test_usage_collector_retains_failed_attempt_without_inventing_usage() -> None:
    collector = UsageCollector()
    collector.record_failure("relation", "structured output invalid", prompt="relation prompt")
    assert collector.attempts == [{"stage": "relation", "status": "failed", "error": "structured output invalid", "raw_usage": None, "prompt_sha256": collector.attempts[0]["prompt_sha256"]}]
    assert collector.total() is None


def test_provenance_graph_keeps_only_relation_context_found_in_frozen_source() -> None:
    graph = build_provenance_graph(
        [{"subject": "甲", "predicate": "depends_on", "object": "乙", "context": "甲依赖乙。"}, {"subject": "甲", "predicate": "depends_on", "object": "秘密", "context": "invented"}],
        [{"evidence_id": "e1", "text": "甲依赖乙。", "quote": "甲依赖乙。"}],
    )
    assert graph["relationships"] == [{"source": "甲", "target": "乙", "type": "depends_on", "properties": {"evidence_ids": ["e1"], "quote": "甲依赖乙。"}}]


def test_retrieval_passes_only_source_validated_connected_component_to_reasoner() -> None:
    graph = {
        "entities": [{"id": item, "type": "实体", "properties": {}} for item in ("甲服务", "乙服务", "丙服务", "秘密服务", "旁路")],
        "relationships": [
            {"source": "甲服务", "target": "乙服务", "type": "depends_on", "properties": {"evidence_ids": ["e1"]}},
            {"source": "乙服务", "target": "丙服务", "type": "depends_on", "properties": {"evidence_ids": ["e2"]}},
            {"source": "秘密服务", "target": "旁路", "type": "depends_on", "properties": {"evidence_ids": ["hidden"]}},
        ],
    }
    retrieved = retrieve_provenance_graph(graph, "甲服务最终依赖什么？")
    assert [edge["properties"]["evidence_ids"] for edge in retrieved["relationships"]] == [["e1"], ["e2"]]


def test_candidate_reasoner_receives_retrieved_provenance_not_complete_source_prompt(monkeypatch: pytest.MonkeyPatch) -> None:
    received: dict[str, object] = {}

    class FakeNER:
        def __init__(self, **_: object) -> None: pass
        def extract(self, _: str) -> list[object]: return []

    class FakeRelation:
        def __init__(self, **_: object) -> None: pass
        def extract(self, text: str, _: list[object]) -> list[dict[str, str]]:
            return [{"subject": "甲服务", "predicate": "depends_on", "object": "乙服务", "context": text}]

    class FakeReasoner:
        def __init__(self, **_: object) -> None: pass
        def reason(self, graph: dict[str, object], _: str, **__: object) -> str:
            received["graph"] = graph
            return "甲服务依赖乙服务。"

    monkeypatch.setattr(evaluate, "NERExtractor", FakeNER)
    monkeypatch.setattr(evaluate, "RelationExtractor", FakeRelation)
    monkeypatch.setattr(evaluate, "GraphReasoner", FakeReasoner)
    monkeypatch.setattr(evaluate, "version", lambda _: "0.6.8")
    case = {"case_id": "graph-only", "document_revision": "v", "question": "甲服务依赖什么？", "expected_evidence": ["e1"], "allowed_evidence": ["e1"], "expected_answer_terms": ["乙服务"], "documents": [{"evidence_id": "e1", "text": "甲服务依赖乙服务。", "quote": "甲服务依赖乙服务。"}]}
    row = evaluate._run_semantica(case)
    assert row["status"] == "completed"
    assert row["references"] == [{"evidence_id": "e1", "quote": "甲服务依赖乙服务。", "match_type": "graph"}]
    assert received["graph"] == row["retrieved_graph"]


def test_candidate_retains_cold_and_warm_graph_reasoner_measurements(monkeypatch: pytest.MonkeyPatch) -> None:
    class FakeNER:
        def __init__(self, **_: object) -> None: pass
        def extract(self, _: str) -> list[object]: return []

    class FakeRelation:
        def __init__(self, **_: object) -> None: pass
        def extract(self, text: str, _: list[object]) -> list[dict[str, str]]:
            return [{"subject": "甲", "predicate": "depends_on", "object": "乙", "context": text}]

    class FakeReasoner:
        def __init__(self, **_: object) -> None: pass
        def reason(self, _: dict[str, object], __: str, **options: object) -> str:
            return "甲依赖乙。" if options["stage"] == "query-cold" else "甲依赖乙。"

    monkeypatch.setattr(evaluate, "NERExtractor", FakeNER)
    monkeypatch.setattr(evaluate, "RelationExtractor", FakeRelation)
    monkeypatch.setattr(evaluate, "GraphReasoner", FakeReasoner)
    monkeypatch.setattr(evaluate, "version", lambda _: "0.6.8")
    row = evaluate._run_semantica({"case_id": "phases", "document_revision": "v", "question": "甲依赖什么？", "expected_evidence": ["e"], "allowed_evidence": ["e"], "expected_answer_terms": ["乙"], "documents": [{"evidence_id": "e", "text": "甲依赖乙。", "quote": "甲依赖乙。"}]})
    assert [item["query_phase"] for item in row["query_attempts"]] == ["cold", "warm"]
    assert row["indexing_ms"] >= 0


def test_unknown_canary_is_a_privacy_leak_even_without_a_citation() -> None:
    assert has_privacy_leak("答案包含 CANARY-DELETE-91", [{"evidence_id": "e-deleted", "canary": "CANARY-DELETE-91"}])


def test_fixture_requires_forbidden_sources_to_be_outside_allowed_projection(tmp_path: Path) -> None:
    dataset = tmp_path / "cases.jsonl"
    dataset.write_text('{"case_id":"x","document_revision":"r","question":"q","expected_evidence":[],"allowed_evidence":["e-public"],"documents":[{"evidence_id":"e-public","text":"公开。","quote":"公开。"}],"forbidden_sources":[{"evidence_id":"e-deleted","state":"deleted","canary":"CANARY-DELETE-91"}]}\n', encoding="utf-8")
    rows, failures = evaluate_rows(dataset)
    assert failures == 0
    assert rows[0]["forbidden_sources"][0]["evidence_id"] not in rows[0]["allowed_evidence"]


def test_malformed_case_is_retained_as_failed_row_not_dropped(tmp_path: Path) -> None:
    dataset = tmp_path / "questions.jsonl"
    dataset.write_text('{"case_id":"ok","document_revision":"r1","question":"问题","expected_evidence":["e1"],"allowed_evidence":["e1"],"documents":[]}\nnot-json\n', encoding="utf-8")
    rows, failures = evaluate_rows(dataset)
    assert [row["case_id"] for row in rows] == ["ok", "dataset-line-2"]
    assert rows[1]["status"] == "failed"
    assert "JSON" in rows[1]["error"]
    assert failures == 1
