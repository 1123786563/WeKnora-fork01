import json
from pathlib import Path

import pytest

from evaluate import aggregate_cases, evaluate_run, load_cases, load_observations, score_case


FIXTURES = Path(__file__).parent / "fixtures"
POLICY = Path(__file__).resolve().parents[2] / "docs/plans/semantica/acceptance-policy.json"


def test_score_case_penalizes_unsupported_evidence():
    case = {
        "requested_mode": "reason",
        "expected_evidence_ids": ["e1"],
        "expected_conclusion_ids": ["c1"],
        "answerable": True,
        "forbidden_evidence_ids": [],
    }
    observation = {
        "actual_evidence_ids": ["e1", "e-hidden"],
        "actual_conclusion_ids": ["c1"],
        "actual_status": "answered",
        "requested_mode": "reason",
        "actual_mode": "reason",
    }

    score = score_case(case, observation)

    assert score["source_precision"] == 0.5
    assert score["source_recall"] == 1.0
    assert score["permission_leak"] is True
    assert score["hard_gate_pass"] is False
    assert score["correct"] is False


def test_empty_prediction_does_not_score_as_perfect_for_answerable_case():
    case = {
        "requested_mode": "reason",
        "expected_evidence_ids": ["e1"],
        "expected_conclusion_ids": ["c1"],
        "answerable": True,
        "forbidden_evidence_ids": [],
    }
    observation = {
        "actual_evidence_ids": [],
        "actual_conclusion_ids": [],
        "actual_status": "insufficient_evidence",
        "requested_mode": "reason",
        "actual_mode": "reason",
    }

    score = score_case(case, observation)

    assert score["source_precision"] == 0.0
    assert score["source_recall"] == 0.0
    assert score["correct"] is False


def test_unanswerable_requires_no_claim_and_no_evidence():
    case = {
        "requested_mode": "reason",
        "expected_evidence_ids": [],
        "expected_conclusion_ids": [],
        "answerable": False,
        "forbidden_evidence_ids": [],
    }
    observation = {
        "actual_evidence_ids": [],
        "actual_conclusion_ids": [],
        "actual_status": "insufficient_evidence",
        "requested_mode": "reason",
        "actual_mode": "reason",
    }

    assert score_case(case, observation)["unanswerable_correct"] is True


def test_permission_leak_is_hard_gate():
    case = {
        "requested_mode": "graph_rag",
        "expected_evidence_ids": ["e1"],
        "expected_conclusion_ids": ["c1"],
        "answerable": True,
        "forbidden_evidence_ids": ["e-forbidden"],
    }
    observation = {
        "actual_evidence_ids": ["e1", "e-forbidden"],
        "actual_conclusion_ids": ["c1"],
        "actual_status": "answered",
        "requested_mode": "graph_rag",
        "actual_mode": "reason",
    }

    score = score_case(case, observation)

    assert score["permission_leak"] is True
    assert score["mode_match"] is False
    assert score["hard_gate_pass"] is False


def test_aggregates_exclude_missing_usage_instead_of_coercing_to_zero():
    summary = aggregate_cases(
        [
            {
                "correct": True,
                "source_precision": 1.0,
                "source_recall": 1.0,
                "unanswerable_correct": None,
                "permission_leak": False,
                "mode_match": True,
                "latency_ms": None,
                "input_tokens": None,
                "output_tokens": None,
            }
        ]
    )

    assert summary["latency_p50_ms"] is None
    assert summary["reported_input_tokens"] is None


def test_synthetic_question_set_has_unique_cases_and_permission_counterexamples():
    cases = load_cases(FIXTURES / "questions.jsonl")

    assert len({case["case_id"] for case in cases}) == len(cases)
    assert all(case["synthetic"] is True for case in cases)
    assert {case["scenario"] for case in cases} >= {
        "dependency_chain",
        "missing_premise",
        "conflict",
        "insufficient_evidence",
        "revoked_source",
        "foreign_alias",
    }
    assert any("e-foreign" in case["forbidden_evidence_ids"] for case in cases)


def test_case_loader_rejects_non_synthetic_or_forbidden_expected_overlap(tmp_path):
    path = tmp_path / "invalid.jsonl"
    path.write_text(
        json.dumps(
            {
                "case_id": "case-invalid",
                "scenario": "direct_lookup",
                "synthetic": False,
                "tenant_id": "tenant-synthetic",
                "kb_id": "kb-synthetic",
                "document_revisions": {"doc-a": "r1"},
                "query": "合成问题",
                "requested_mode": "graph_rag",
                "expected_evidence_ids": ["e1"],
                "expected_conclusion_ids": ["c1"],
                "answerable": True,
                "forbidden_evidence_ids": ["e1"],
            },
            ensure_ascii=False,
        )
        + "\n"
    )

    with pytest.raises(ValueError):
        load_cases(path)


def test_evaluate_run_requires_one_observation_for_every_case():
    case = {
        "case_id": "case-1",
        "requested_mode": "reason",
        "expected_evidence_ids": [],
        "expected_conclusion_ids": [],
        "answerable": False,
        "forbidden_evidence_ids": [],
    }

    with pytest.raises(ValueError):
        evaluate_run([case], [])


def test_case_mode_is_authoritative_over_observation_claim():
    case = {
        "case_id": "graph-case",
        "requested_mode": "graph_rag",
        "expected_evidence_ids": ["e1"],
        "expected_conclusion_ids": ["c1"],
        "answerable": True,
        "forbidden_evidence_ids": [],
    }
    observation = {
        "case_id": "graph-case",
        "requested_mode": "reason",
        "actual_mode": "reason",
        "actual_status": "answered",
        "actual_evidence_ids": ["e1"],
        "actual_conclusion_ids": ["c1"],
        "access_violations": [],
        "evidence_layer": "synthetic",
    }

    score = score_case(case, observation)

    assert score["mode_match"] is False
    assert score["hard_gate_pass"] is False


def test_empty_case_or_observation_set_is_rejected(tmp_path):
    empty_cases = tmp_path / "empty-cases.jsonl"
    empty_observations = tmp_path / "empty-observations.jsonl"
    empty_cases.write_text("")
    empty_observations.write_text("")

    with pytest.raises(ValueError):
        load_cases(empty_cases)
    with pytest.raises(ValueError):
        load_observations(empty_observations)
    with pytest.raises(ValueError):
        evaluate_run([], [])


def test_example_report_is_synthetic_and_never_approves_policy():
    cases = load_cases(FIXTURES / "questions.jsonl")
    observations = load_observations(FIXTURES / "evaluation-example-observations.jsonl")

    report = evaluate_run(cases, observations)
    policy = json.loads(POLICY.read_text())

    assert report["synthetic_only"] is True
    assert report["evidence_layer"] == "synthetic"
    assert policy["approved"] is False
    assert policy["proposed_thresholds"] is None
