"""V03 evaluation harness: scoring, cost/latency aggregation and backends.

Experiment tooling only (never imported by production code). Provides:

- score_case: evidence precision/recall arithmetic for one question case.
- leak_gate: the permission-leak hard gate - a single restricted-content hit
  fails it regardless of any averaged quality score.
- CLI: run the frozen question dataset against the "semantica" backend
  (authorized-subgraph retrieval over the controlled fixtures, real isolated
  Neo4j) or the "native" backend (existing graph retrieval via an explicitly
  configured isolated TEST endpoint; never a default production URL).

Usage (from the worktree root):
  uv run --project semantic/experiments python semantic/experiments/evaluate.py \
      --backend semantica --dataset semantic/experiments/fixtures/questions.jsonl \
      --output docs/superpowers/plans/semantica/semantica-results.jsonl

Exit codes: 0 = run completed with zero permission leaks; 2 = run completed
but the leak gate tripped or the backend was unavailable (failure rows are
still written - never silently dropped); 1 = usage/dataset error.
"""

from __future__ import annotations

import argparse
import json
import os
import statistics
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURES_DIR = Path(__file__).resolve().parent / "fixtures"
NATIVE_URL_VAR = "SEMANTICA_EXPERIMENT_NATIVE_URL"


def score_case(expected_evidence: set[str], actual_evidence: set[str]) -> dict:
    """Evidence precision/recall for one case.

    - precision: of the evidence actually cited, the fraction that was
      expected (unsupported citations are penalized).
    - recall: of the expected evidence, the fraction actually cited; an empty
      expectation is fully satisfied by definition (unanswerable cases are
      judged by "actual must be empty", a separate metric).
    """
    expected = set(expected_evidence)
    actual = set(actual_evidence)
    overlap = len(expected & actual)
    return {
        "precision": overlap / len(actual) if actual else 0.0,
        "recall": overlap / len(expected) if expected else 1.0,
    }


def leak_gate(payload_text: str, restricted_terms: Iterable[str]) -> dict:
    """Hard permission gate: any restricted term present in the payload fails.

    This gate is intentionally not a score - it cannot be compensated by
    averages and short-circuits the whole case (and the run) to failed.
    """
    payload = payload_text.casefold()
    unique_terms = sorted({term for term in restricted_terms if term})
    leaked = [term for term in unique_terms if term.casefold() in payload]
    return {"leaked": bool(leaked), "leaked_terms": leaked}


CATEGORIES = {"fact", "multihop", "conflict", "unanswerable", "zh_locate", "permission"}


def _validate_case(case: dict, line_no: int) -> None:
    required = ["case_id", "category", "query", "fixture", "authorized_documents", "expected_evidence", "expected_nodes"]
    missing = [key for key in required if key not in case]
    if missing:
        raise SystemExit(f"dataset line {line_no} ({case.get('case_id', '?')}): missing keys {missing}")
    if case["category"] not in CATEGORIES:
        raise SystemExit(
            f"dataset line {line_no} ({case['case_id']}): unknown category {case['category']!r}; "
            f"expected one of {sorted(CATEGORIES)}"
        )
    if case["category"] not in {"unanswerable", "permission"} and not case["expected_nodes"]:
        raise SystemExit(
            f"dataset line {line_no} ({case['case_id']}): {case['category']} cases need non-empty expected_nodes"
        )
    if case["category"] == "permission" and not case.get("restricted_terms"):
        raise SystemExit(
            f"dataset line {line_no} ({case['case_id']}): permission cases need non-empty restricted_terms"
        )
    fixture_name = case["fixture"]
    if not isinstance(fixture_name, str) or "/" in fixture_name or ".." in fixture_name:
        raise SystemExit(f"dataset line {line_no} ({case['case_id']}): fixture must be a plain filename")
    if not (FIXTURES_DIR / fixture_name).is_file():
        raise SystemExit(
            f"dataset line {line_no} ({case['case_id']}): fixture {fixture_name!r} not found in {FIXTURES_DIR}"
        )


def load_dataset(path_text: str) -> list[dict]:
    path = Path(path_text)
    if not path.is_absolute():
        path = REPO_ROOT / path
    cases = []
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not line.strip():
            continue
        try:
            case = json.loads(line)
        except json.JSONDecodeError as exc:
            raise SystemExit(f"dataset line {line_no} is not valid JSON: {exc}")
        _validate_case(case, line_no)
        cases.append(case)
    if not cases:
        raise SystemExit(f"dataset contains no cases: {path}")
    return cases


def _percentile(values: list[float], pct: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    rank = (len(ordered) - 1) * pct
    low = int(rank)
    high = min(low + 1, len(ordered) - 1)
    if low == high:
        return round(ordered[low], 3)
    weight = rank - low
    return round(ordered[low] * (1 - weight) + ordered[high] * weight, 3)


def _restricted_evidence(fixture: dict, case: dict) -> set[str]:
    """Evidence ids of restricted documents that this case is NOT authorized for."""
    restricted_docs = set(fixture.get("restricted_documents", []))
    allowed = set(case["authorized_documents"])
    hidden = restricted_docs - allowed
    return {
        evidence
        for edge in fixture["edges"]
        if edge["document_id"] in hidden
        for evidence in edge["evidence_ids"]
    }


def _case_correct(case: dict, actual: set[str], reachable: set[str], quote_ok, leak: dict, restricted_ids: set[str]) -> bool:
    category = case["category"]
    if category == "unanswerable":
        return not actual
    if category == "permission":
        # Permission correctness: no restricted evidence cited and no leak;
        # over-citing irrelevant visible evidence is a quality penalty, not a
        # permission failure.
        return (not leak["leaked"]) and not restricted_ids & actual
    if case.get("expect_quote") is not None and quote_ok is not True:
        return False
    return set(case["expected_nodes"]) <= reachable


def _eval_case_semantica(case: dict, fixture: dict, rows: dict) -> dict:
    from semantica.context import ContextRetriever

    import bridge_probe

    allowed = frozenset(case["authorized_documents"])
    graph, authorized_edges, _visible = bridge_probe.build_probe_graph(rows, allowed)

    start = time.perf_counter()
    retriever = ContextRetriever(knowledge_graph=graph)
    hits = retriever.retrieve(case["query"], max_results=5)
    cold_ms = round((time.perf_counter() - start) * 1000, 3)

    hot = []
    for _ in range(2):
        t0 = time.perf_counter()
        retriever.retrieve(case["query"], max_results=5)
        hot.append(round((time.perf_counter() - t0) * 1000, 3))
    hot_ms = round(statistics.median(hot), 3)

    hit_ids = [h.metadata.get("node_id") for h in hits if h.metadata.get("node_id")]
    related_ids = set()
    for node_id in hit_ids:
        for item in retriever.get_related(node_id, max_hops=2):
            if item.get("id"):
                related_ids.add(item["id"])
    reachable = set(hit_ids) | related_ids

    # Facts the retrieval result actually exposes: authorized edges incident
    # to the returned entities (hits and their related entities).
    cited_quotes = []
    actual_evidence: set[str] = set()
    for edge in authorized_edges:
        if edge["source_id"] in reachable or edge["target_id"] in reachable:
            actual_evidence.update(edge["props"]["evidence_ids"])
            cited_quotes.append(edge["props"]["quote"])

    expected = set(case["expected_evidence"])
    scores = score_case(expected, actual_evidence)

    quote_ok = None
    if case.get("expect_quote") is not None:
        quote_ok = any(case["expect_quote"] in quote for quote in cited_quotes)

    node_names = {
        node["entity_id"]: node["name"] for node in rows["nodes"]
    }
    # The hard gate must see every piece of text the retrieval result could
    # expose: hit content objects AND node names AND related ids AND quotes.
    payload = json.dumps(
        {
            "hit_content_objects": [str(getattr(hit, "content", "")) for hit in hits],
            "hit_node_names": [node_names.get(node_id, node_id) for node_id in hit_ids],
            "hits_without_node_id": len(hits) - len(hit_ids),
            "related_ids": sorted(related_ids),
            "cited_quotes": cited_quotes,
            "evidence_ids": sorted(actual_evidence),
        },
        ensure_ascii=False,
    )
    leak = leak_gate(payload, case.get("restricted_terms", []))
    restricted_ids = _restricted_evidence(fixture, case)

    def _correct() -> bool:
        return _case_correct(case, actual_evidence, reachable, quote_ok, leak, restricted_ids)

    return {
        "case_id": case["case_id"],
        "category": case["category"],
        "query": case["query"],
        "backend": "semantica",
        "mode": "authorized-subgraph keyword retrieval (ContextRetriever, no model)",
        "engine": bridge_probe.ENGINE_VERSION,
        "model": "none (no model call)",
        "fixture": case["fixture"],
        "document_revision": max(item["revision"] for item in fixture["documents"]),
        "authorized_documents": sorted(allowed),
        "expected_evidence": sorted(expected),
        "actual_evidence": sorted(actual_evidence),
        "source_precision": scores["precision"],
        "source_recall": scores["recall"],
        "correct": _correct(),
        "quote_ok": quote_ok,
        "leak": leak,
        "permission_correct": _correct() if case["category"] == "permission" else None,
        "hit_count": len(hit_ids),
        "latency_ms": {"cold": cold_ms, "hot": hot_ms},
        "usage": {"tokens": 0, "model_calls": 0, "note": "retrieval-only evaluation; no model invoked"},
        "error": None,
    }


def run_semantica(dataset_path: str, output_path: str) -> int:
    import bridge_probe

    cases = load_dataset(dataset_path)
    fixtures: dict[str, dict] = {}
    fixture_rows: dict[str, dict] = {}
    index_ms: dict[str, float] = {}
    for fixture_name in sorted({case["fixture"] for case in cases}):
        fixture = json.loads((FIXTURES_DIR / fixture_name).read_text(encoding="utf-8"))
        start = time.perf_counter()
        bridge_probe.write_fixture_to_neo4j(fixture)
        index_ms[fixture_name] = round((time.perf_counter() - start) * 1000, 1)
        fixtures[fixture_name] = fixture
        fixture_rows[fixture_name] = bridge_probe.read_rows_from_neo4j()

    results = []
    for case in cases:
        try:
            results.append(_eval_case_semantica(case, fixtures[case["fixture"]], fixture_rows[case["fixture"]]))
        except Exception as exc:  # failure rows are written, never dropped
            results.append(
                _error_row(
                    case,
                    backend="semantica",
                    mode="authorized-subgraph keyword retrieval (ContextRetriever, no model)",
                    engine=bridge_probe.ENGINE_VERSION,
                    error=f"{type(exc).__name__}: {exc}",
                    usage_note="case failed before retrieval completed",
                )
            )
    leak_count = sum(1 for row in results if row["leak"]["leaked"])
    summary = _summarize("semantica", cases, results, index_ms, available=True)
    _write_output(output_path, results, summary)
    print(f"semantica evaluation: {summary['case_count']} cases, correct={summary['correct_rate']}, leaks={leak_count}")
    return 0 if leak_count == 0 else 2


def _error_row(case: dict, backend: str, mode: str, engine: str, error: str, usage_note: str) -> dict:
    return {
        "case_id": case["case_id"],
        "category": case["category"],
        "query": case["query"],
        "backend": backend,
        "mode": mode,
        "engine": engine,
        "model": "unavailable",
        "fixture": case["fixture"],
        "document_revision": None,
        "authorized_documents": sorted(case["authorized_documents"]),
        "expected_evidence": sorted(case["expected_evidence"]),
        "actual_evidence": [],
        "source_precision": 0.0,
        "source_recall": 0.0,
        "correct": False,
        "quote_ok": None,
        "leak": {"leaked": False, "leaked_terms": []},
        "permission_correct": None,
        "hit_count": 0,
        "latency_ms": {"cold": None, "hot": None},
        "usage": {"tokens": None, "model_calls": None, "note": usage_note},
        "error": error,
    }


def run_native(dataset_path: str, output_path: str) -> int:
    cases = load_dataset(dataset_path)
    endpoint = os.environ.get(NATIVE_URL_VAR, "").strip()
    if not endpoint:
        error = (
            "native backend unavailable: SEMANTICA_EXPERIMENT_NATIVE_URL is not set; "
            "point it at an isolated TEST deployment of the existing WeKnora graph "
            "retrieval entry (never a production URL)"
        )
    else:
        error = (
            "native backend not yet exercised: an isolated TEST deployment of the "
            f"WeKnora graph services is required to call {endpoint} with real "
            "extraction-backed graph data and credentials"
        )
    results = [
        _error_row(
            case,
            backend="native",
            mode="existing graph retrieval via isolated test deployment",
            engine="unavailable",
            error=error,
            usage_note="no native run",
        )
        for case in cases
    ]
    summary = _summarize("native", cases, results, {}, available=False)
    summary["blocked_reason"] = error
    _write_output(output_path, results, summary)
    print(error)
    return 2


def _summarize(backend: str, cases: list[dict], results: list[dict], index_ms: dict, available: bool) -> dict:
    by_category: dict[str, dict] = {}
    for case, row in zip(cases, results):
        bucket = by_category.setdefault(case["category"], {"total": 0, "correct": 0})
        bucket["total"] += 1
        if row["correct"]:
            bucket["correct"] += 1
    measured = [row for row in results if row["error"] is None]
    cold = [row["latency_ms"]["cold"] for row in measured if row["latency_ms"]["cold"] is not None]
    hot = [row["latency_ms"]["hot"] for row in measured if row["latency_ms"]["hot"] is not None]
    unanswerable = by_category.get("unanswerable", {"correct": 0, "total": 0})
    # Metric conventions (documented in evaluation-baseline.md):
    # - source_precision includes empty-actual rows as 0.0 (strict: a correct
    #   unanswerable case still drags the mean down - asymmetry is deliberate
    #   and conservative).
    # - source_recall mean excludes empty-expected rows, whose recall is 1.0 by
    #   definition and would otherwise inflate the headline.
    return {
        "type": "summary",
        "backend": backend,
        "available": available,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "case_count": len(cases),
        "correct_rate": round(sum(1 for row in results if row["correct"]) / len(results), 3) if results else 0.0,
        "unanswerable_correct": round(unanswerable["correct"] / unanswerable["total"], 3) if unanswerable["total"] else None,
        "by_category": {
            category: {"correct": bucket["correct"], "total": bucket["total"], "rate": round(bucket["correct"] / bucket["total"], 3)}
            for category, bucket in sorted(by_category.items())
        },
        "evidence_precision_mean": round(statistics.fmean([row["source_precision"] for row in measured]), 3) if measured else None,
        "evidence_recall_mean_nonempty": (
            round(
                statistics.fmean([row["source_recall"] for row in measured if row["expected_evidence"]]),
                3,
            )
            if any(row["expected_evidence"] for row in measured)
            else None
        ),
        "leak_count": sum(1 for row in results if row["leak"]["leaked"]),
        "latency_p50_ms": {"cold": _percentile(cold, 0.5), "hot": _percentile(hot, 0.5)} if measured else {"cold": None, "hot": None},
        "latency_p95_ms": {"cold": _percentile(cold, 0.95), "hot": _percentile(hot, 0.95)} if measured else {"cold": None, "hot": None},
        "latency_note": "cold includes retriever construction; hot is the median of two repeat queries on the same retriever",
        "index_ms": index_ms,
        "tokens_total": sum(row["usage"]["tokens"] or 0 for row in results),
    }


def _write_output(output_path: str, results: list[dict], summary: dict) -> None:
    path = Path(output_path)
    if not path.is_absolute():
        path = REPO_ROOT / path
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as handle:
        for row in results:
            print(json.dumps(row, ensure_ascii=False), file=handle)
        print(json.dumps(summary, ensure_ascii=False), file=handle)


class _EvalParser(argparse.ArgumentParser):
    def error(self, message: str):
        # Usage errors exit 1 so exit code 2 stays reserved for
        # "run completed but leak gate tripped / backend unavailable".
        self.print_usage(sys.stderr)
        print(f"error: {message}", file=sys.stderr)
        raise SystemExit(1)


def main(argv: list[str]) -> int:
    parser = _EvalParser(description="V03 evaluation harness")
    parser.add_argument("--backend", choices=["native", "semantica"], required=True)
    parser.add_argument("--dataset", default="semantic/experiments/fixtures/questions.jsonl")
    parser.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    if args.backend == "semantica":
        return run_semantica(args.dataset, args.output)
    return run_native(args.dataset, args.output)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
