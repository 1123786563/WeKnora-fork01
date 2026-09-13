"""V03 evaluator correctness tests.

score_case is the arithmetic core of the quality gate; these tests pin its
exact semantics independently of any backend, plus the permission-leak hard
gate that no average may override.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from evaluate import leak_gate, score_case  # noqa: E402


def test_evidence_score_penalizes_unsupported_source():
    score = score_case({"e1"}, {"e1", "hidden"})
    assert score["precision"] == 0.5
    assert score["recall"] == 1.0


def test_perfect_evidence_scores_one():
    score = score_case({"e1", "e2"}, {"e1", "e2"})
    assert score["precision"] == 1.0
    assert score["recall"] == 1.0


def test_unanswerable_case_defines_full_recall():
    # Expected empty (no evidence may be cited): recall is 1.0 by definition;
    # citing anything yields zero precision. unanswerable_correctness is a
    # separate metric (actual must be empty), not an average of these.
    clean = score_case(set(), set())
    assert clean["recall"] == 1.0
    assert clean["precision"] == 0.0
    polluted = score_case(set(), {"e9"})
    assert polluted["recall"] == 1.0
    assert polluted["precision"] == 0.0


def test_missing_expected_evidence_lowers_recall():
    score = score_case({"e1", "e2"}, {"e1"})
    assert score["recall"] == 0.5
    assert score["precision"] == 1.0


def test_leak_gate_matches_case_insensitive_ids():
    # Latin entity ids may be cased differently by backends; the hard gate
    # must never under-report because of case.
    verdict = leak_gate("found ENT-SONG in results", restricted_terms=["ent-song"])
    assert verdict["leaked"] is True
    assert verdict["leaked_terms"] == ["ent-song"]


def test_leak_gate_reports_each_term_once():
    verdict = leak_gate("松柏 与 松柏", restricted_terms=["松柏", "松柏"])
    assert verdict["leaked_terms"] == ["松柏"]


def test_case_correct_requires_expected_nodes_for_answer_cases():
    from evaluate import _case_correct

    case = {"case_id": "x", "category": "fact", "expected_nodes": ["ent-x"], "expected_evidence": ["e1"]}
    passing = _case_correct(case, {"e1"}, {"ent-x"}, None, {"leaked": False, "leaked_terms": []}, set())
    failing = _case_correct(case, {"e1"}, set(), None, {"leaked": False, "leaked_terms": []}, set())
    assert passing is True
    assert failing is False


def test_case_correct_judges_unanswerable_and_permission():
    from evaluate import _case_correct

    unanswerable = {"case_id": "u", "category": "unanswerable", "expected_nodes": []}
    assert _case_correct(unanswerable, set(), set(), None, {"leaked": False}, set()) is True
    assert _case_correct(unanswerable, {"e1"}, set(), None, {"leaked": False}, set()) is False

    permission = {"case_id": "p", "category": "permission", "expected_nodes": []}
    clean = _case_correct(permission, {"e-visible"}, set(), None, {"leaked": False}, {"e-secret"})
    leaked = _case_correct(permission, set(), set(), None, {"leaked": True}, set())
    cited_secret = _case_correct(permission, {"e-secret"}, set(), None, {"leaked": False}, {"e-secret"})
    assert clean is True
    assert leaked is False
    assert cited_secret is False


def test_percentile_interpolates_and_handles_empty():
    from evaluate import _percentile

    assert _percentile([], 0.5) == 0.0
    assert _percentile([5.0], 0.5) == 5.0
    assert _percentile([1.0, 2.0, 3.0, 4.0], 0.5) == 2.5
    assert _percentile([1.0, 2.0, 3.0, 4.0], 0.95) == 3.85


def test_load_dataset_rejects_invalid_cases(tmp_path):
    import json as _json

    import pytest

    from evaluate import load_dataset

    bad_category = tmp_path / "bad1.jsonl"
    bad_category.write_text(_json.dumps({"case_id": "a", "category": "facts", "query": "q", "fixture": "evaluation_corpus.json", "authorized_documents": ["d01"], "expected_evidence": [], "expected_nodes": ["ent-x"]}), encoding="utf-8")
    with pytest.raises(SystemExit):
        load_dataset(str(bad_category))

    empty_nodes = tmp_path / "bad2.jsonl"
    empty_nodes.write_text(_json.dumps({"case_id": "b", "category": "fact", "query": "q", "fixture": "evaluation_corpus.json", "authorized_documents": ["d01"], "expected_evidence": [], "expected_nodes": []}), encoding="utf-8")
    with pytest.raises(SystemExit):
        load_dataset(str(empty_nodes))

    permission_without_terms = tmp_path / "bad3.jsonl"
    permission_without_terms.write_text(_json.dumps({"case_id": "c", "category": "permission", "query": "q", "fixture": "evaluation_corpus.json", "authorized_documents": ["d01"], "expected_evidence": [], "expected_nodes": []}), encoding="utf-8")
    with pytest.raises(SystemExit):
        load_dataset(str(permission_without_terms))


def test_load_dataset_rejects_empty_dataset(tmp_path):
    import pytest

    from evaluate import load_dataset

    empty = tmp_path / "empty.jsonl"
    empty.write_text("", encoding="utf-8")
    with pytest.raises(SystemExit):
        load_dataset(str(empty))


def test_leak_gate_is_hard_not_averageable():
    # One restricted-content hit fails the gate regardless of quality scores;
    # the verdict names exactly which terms leaked.
    verdict = leak_gate("代号是松柏", restricted_terms=["松柏", "密钥"])
    assert verdict["leaked"] is True
    assert verdict["leaked_terms"] == ["松柏"]
    ok = leak_gate("甲公司控股乙公司", restricted_terms=["松柏", "密钥"])
    assert ok["leaked"] is False
    assert ok["leaked_terms"] == []
