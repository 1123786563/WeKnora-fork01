"""Behavioral contracts for the V03 frozen-corpus evaluator."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from evaluate import assess_answer, evaluate_rows, score_case


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


def test_malformed_case_is_retained_as_failed_row_not_dropped(tmp_path: Path) -> None:
    dataset = tmp_path / "questions.jsonl"
    dataset.write_text('{"case_id":"ok","document_revision":"r1","question":"问题","expected_evidence":["e1"],"allowed_evidence":["e1"],"documents":[]}\nnot-json\n', encoding="utf-8")
    rows, failures = evaluate_rows(dataset)
    assert [row["case_id"] for row in rows] == ["ok", "dataset-line-2"]
    assert rows[1]["status"] == "failed"
    assert "JSON" in rows[1]["error"]
    assert failures == 1
