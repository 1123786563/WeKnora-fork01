"""Frozen V03 Chinese-corpus evaluator.

This is an experiment-only CLI.  It accepts only explicit loopback experiment
configuration and retains every parse, transport, extraction, and query error.
"""
from __future__ import annotations

import argparse
import json
import os
import statistics
import time
import urllib.error
import urllib.request
from hashlib import sha256
from importlib.metadata import version
from pathlib import Path
from typing import Any, Iterable, Mapping

from semantica.semantic_extract import NERExtractor, RelationExtractor
from semantica.semantic_extract.providers import BaseProvider
from semantica.semantic_extract.registry import provider_registry
from semantica.reasoning import GraphReasoner


GATEWAY = "http://127.0.0.1:18092"
PROVIDER_NAME = "semantica-v03-go-loopback"


class UsageCollector:
    """One run-scoped ledger shared by every extractor-created provider."""
    def __init__(self) -> None:
        self.attempts: list[dict[str, Any]] = []

    def record(self, stage: str, usage: Mapping[str, Any], **metadata: Any) -> None:
        if usage.get("total_tokens") != usage.get("prompt_tokens", 0) + usage.get("completion_tokens", 0):
            raise ValueError("inconsistent raw usage")
        self.attempts.append({"stage": stage, "raw_usage": dict(usage), **metadata})

    def record_failure(self, stage: str, error: str, *, prompt: str) -> None:
        self.attempts.append({
            "stage": stage,
            "status": "failed",
            "error": error,
            "raw_usage": None,
            "prompt_sha256": sha256(prompt.encode()).hexdigest(),
        })

    def total(self) -> dict[str, int] | None:
        usages = [entry["raw_usage"] for entry in self.attempts if entry.get("raw_usage")]
        if not usages:
            return None
        return {key: sum(int(usage[key]) for usage in usages) for key in ("prompt_tokens", "completion_tokens", "total_tokens")}


def score_case(expected_evidence: set[str], actual_evidence: set[str]) -> dict[str, float]:
    overlap = len(expected_evidence & actual_evidence)
    return {
        "precision": overlap / len(actual_evidence) if actual_evidence else 0.0,
        "recall": overlap / len(expected_evidence) if expected_evidence else (1.0 if not actual_evidence else 0.0),
    }


def assess_answer(case: Mapping[str, Any], answer: str, actual_evidence: set[str]) -> dict[str, bool]:
    """Assess conclusion and abstention separately from retrieval evidence."""
    if case.get("unanswerable"):
        abstained = "证据不足" in answer and not actual_evidence
        return {"correct": abstained, "unanswerable_correct": abstained}
    oracle = case.get("conclusion_oracle", {})
    required = [str(value) for value in oracle.get("must_contain", case.get("expected_answer_terms", []))]
    forbidden = [str(value) for value in oracle.get("must_not_contain", [])]
    polarity = case.get("expected_polarity")
    negated = any(marker in answer for marker in ("不依赖", "并非", "不是", "未依赖", "没有依赖"))
    correct = bool(actual_evidence) and bool(required) and all(term in answer for term in required) and not any(term in answer for term in forbidden) and (polarity != "positive" or not negated)
    return {"correct": correct, "unanswerable_correct": False}


def has_privacy_leak(answer: str, forbidden_sources: Iterable[Mapping[str, Any]]) -> bool:
    return any(str(item.get("evidence_id", "")) in answer or (item.get("canary") and str(item["canary"]) in answer) for item in forbidden_sources)


def observed_privacy_violation(answer: str, references: Iterable[Mapping[str, Any]], forbidden_sources: Iterable[Mapping[str, Any]]) -> bool:
    """Fail closed when an answer or an observed backend reference reaches forbidden data."""
    forbidden = list(forbidden_sources)
    forbidden_runtime_ids = {str(item["runtime_id"]) for item in forbidden if item.get("runtime_id")}
    return has_privacy_leak(answer, forbidden) or any(
        str(reference.get("knowledge_id", "")) in forbidden_runtime_ids
        for reference in references
    )


def build_provenance_graph(relations: Iterable[Any], documents: Iterable[Mapping[str, Any]]) -> dict[str, Any]:
    """Accept only extractor relation contexts that are present in a frozen source."""
    docs = list(documents); edges = []
    for relation in relations:
        value = relation if isinstance(relation, Mapping) else {"subject": getattr(relation.subject, "text", str(relation.subject)), "predicate": relation.predicate, "object": getattr(relation.object, "text", str(relation.object)), "context": relation.context}
        matching = [doc for doc in docs if value.get("context") and value["context"] in doc.get("text", "")]
        if matching:
            edges.append({"source": value["subject"], "target": value["object"], "type": value["predicate"], "properties": {"evidence_ids": [doc["evidence_id"] for doc in matching], "quote": value["context"]}})
    names = sorted({edge["source"] for edge in edges} | {edge["target"] for edge in edges})
    return {"entities": [{"id": name, "type": "实体", "properties": {}} for name in names], "relationships": edges}


def retrieve_provenance_graph(graph: Mapping[str, Any], question: str) -> dict[str, Any]:
    """Select the connected source-validated subgraph rooted in named question entities."""
    edges = list(graph.get("relationships", []))
    roots = {str(entity["id"]) for entity in graph.get("entities", []) if str(entity.get("id", "")) in question}
    if not roots:
        return {"entities": [], "relationships": []}
    selected: list[Mapping[str, Any]] = []
    reachable = set(roots)
    changed = True
    while changed:
        changed = False
        for edge in edges:
            if edge["source"] in reachable or edge["target"] in reachable:
                if edge not in selected:
                    selected.append(edge)
                before = len(reachable)
                reachable.update((str(edge["source"]), str(edge["target"])))
                changed = changed or len(reachable) != before
    return {
        "entities": [entity for entity in graph.get("entities", []) if str(entity.get("id", "")) in reachable],
        "relationships": selected,
    }


def graph_references(graph: Mapping[str, Any]) -> list[dict[str, Any]]:
    """Expose only evidence IDs that survived source validation and retrieval."""
    return [
        {"evidence_id": evidence_id, "quote": edge.get("properties", {}).get("quote", ""), "match_type": "graph"}
        for edge in graph.get("relationships", [])
        for evidence_id in edge.get("properties", {}).get("evidence_ids", [])
    ]


def _percentile(values: list[float], fraction: float) -> float | None:
    if not values:
        return None
    return round(sorted(values)[max(0, min(len(values) - 1, int((len(values) - 1) * fraction)))], 3)


def _summary(rows: Iterable[Mapping[str, Any]]) -> dict[str, Any]:
    materialized = list(rows)
    scores = [score_case(set(row.get("expected_evidence", [])), set(row.get("evidence_ids", []))) for row in materialized]
    latencies = [float(row["latency_ms"]) for row in materialized if isinstance(row.get("latency_ms"), (int, float))]
    token_values = [int((row.get("tokens") or {}).get("total_tokens", 0)) for row in materialized if isinstance(row.get("tokens"), Mapping)]
    privacy = any(bool(row.get("privacy_violation")) for row in materialized)
    def phase_values(phase: str) -> list[float]:
        return [float(item["latency_ms"]) for row in materialized for item in row.get("query_attempts", []) if item.get("query_phase") == phase and isinstance(item.get("latency_ms"), (int, float))]
    cold, warm = phase_values("cold"), phase_values("warm")
    indexing = [float(row["indexing_ms"]) for row in materialized if isinstance(row.get("indexing_ms"), (int, float))]
    return {
        "cases": len(materialized),
        "source_precision": round(sum(score["precision"] for score in scores) / len(scores), 4) if scores else 0.0,
        "source_recall": round(sum(score["recall"] for score in scores) / len(scores), 4) if scores else 0.0,
        "correct": sum(bool(row.get("correct")) for row in materialized),
        "unanswerable_correct": sum(bool(row.get("unanswerable_correct")) for row in materialized),
        "privacy_hardgate_passed": not privacy,
        "execution_complete": all(row.get("status") == "completed" for row in materialized),
        # A completed program is not an accepted quality result.  Only the
        # separately approved policy may promote this value.
        "quality_gate_passed": False,
        "latency_p50_ms": _percentile(latencies, 0.5), "latency_p95_ms": _percentile(latencies, 0.95),
        "cold_query_latency": {"available_count": len(cold), "p50_ms": _percentile(cold, .5), "p95_ms": _percentile(cold, .95)},
        "warm_query_latency": {"available_count": len(warm), "p50_ms": _percentile(warm, .5), "p95_ms": _percentile(warm, .95)},
        "indexing_latency": {"available_count": len(indexing), "p50_ms": _percentile(indexing, .5), "p95_ms": _percentile(indexing, .95)},
        "actual_total_tokens": sum(token_values) if token_values else None,
    }


def _load_dataset(path: Path) -> tuple[list[dict[str, Any]], int]:
    rows: list[dict[str, Any]] = []
    failures = 0
    for line_number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not raw.strip():
            continue
        try:
            row = json.loads(raw)
            required = {"case_id", "document_revision", "question", "expected_evidence", "allowed_evidence", "documents"}
            missing = sorted(required - set(row))
            if missing:
                raise ValueError("missing " + ", ".join(missing))
            forbidden = {item.get("evidence_id") for item in row.get("forbidden_sources", [])}
            if forbidden & set(row["allowed_evidence"]):
                raise ValueError("forbidden source may not be in allowed_evidence")
            rows.append(row)
        except (json.JSONDecodeError, ValueError) as exc:
            failures += 1
            rows.append({"case_id": f"dataset-line-{line_number}", "document_revision": None, "expected_evidence": [], "allowed_evidence": [], "documents": [], "status": "failed", "error": f"JSON/dataset error: {exc}"})
    return rows, failures


def evaluate_rows(value: list[Mapping[str, Any]] | Path) -> Any:
    """Summarize supplied result rows, or retain malformed JSONL as failed rows."""
    if isinstance(value, Path):
        rows, failures = _load_dataset(value)
        return rows, failures
    return _summary(value)


class _V03Provider(BaseProvider):
    """Public Semantica provider registered against the fixed Go transport."""
    active_collector: UsageCollector | None = None
    def __init__(self, **kwargs: Any) -> None:
        self.collector: UsageCollector | None = kwargs.pop("usage_collector", None) or self.active_collector
        super().__init__(**kwargs)
        self.calls: list[dict[str, Any]] = []

    def generate(self, prompt: str, **kwargs: Any) -> str:
        request = urllib.request.Request(f"{GATEWAY}/v1/semantica-v03/extract", data=json.dumps({"prompt": prompt}, ensure_ascii=False).encode(), headers={"Content-Type": "application/json"}, method="POST")
        started = time.monotonic()
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                payload = json.loads(response.read())
        except (urllib.error.URLError, urllib.error.HTTPError, json.JSONDecodeError) as exc:
            self.calls.append({"stage": kwargs.get("stage", "generate"), "status": "failed", "latency_ms": round((time.monotonic() - started) * 1000, 3), "error": str(exc), "raw_usage": None})
            if self.collector:
                self.collector.record_failure(kwargs.get("stage", "generate"), str(exc), prompt=prompt)
            raise RuntimeError(f"V03 Go loopback request failed: {exc}") from exc
        usage = payload.get("raw_usage")
        if payload.get("completed") is not True or payload.get("truncated") is not False or not isinstance(usage, Mapping):
            raise RuntimeError("V03 gateway returned incomplete response or missing raw usage")
        if usage.get("total_tokens") != usage.get("prompt_tokens", 0) + usage.get("completion_tokens", 0):
            raise RuntimeError("V03 gateway returned inconsistent raw usage")
        stage = kwargs.get("stage") or ("relation" if '"relations"' in prompt else "ner" if '"entities"' in prompt else "query")
        attempt = {"stage": stage, "status": "completed", "latency_ms": round((time.monotonic() - started) * 1000, 3), "raw_usage": dict(usage), "prompt_sha256": sha256(prompt.encode()).hexdigest()}
        self.calls.append(attempt)
        if self.collector:
            self.collector.record(stage, usage, status="completed", latency_ms=attempt["latency_ms"], prompt_sha256=attempt["prompt_sha256"])
        return str(payload.get("text", ""))


def _register_provider(name: str) -> None:
    provider_registry.register(name, _V03Provider)


def _source_valid_evidence(case: Mapping[str, Any], answer: str) -> set[str]:
    """Only quote-backed text found in both source and actual answer becomes evidence."""
    source = {item["evidence_id"]: item["quote"] for item in case.get("documents", []) if "evidence_id" in item and "quote" in item}
    return {evidence_id for evidence_id, quote in source.items() if quote and quote in answer and quote in str(next((item.get("text", "") for item in case["documents"] if item.get("evidence_id") == evidence_id), ""))}


def _run_semantica(case: Mapping[str, Any]) -> dict[str, Any]:
    started = time.monotonic()
    row = {"case_id": case["case_id"], "document_revision": case["document_revision"], "requested_mode": "semantica", "actual_mode": "semantica-llm-ner-re", "engine_version": version("semantica"), "model_version": "qwen2.5:0.5b", "expected_evidence": case["expected_evidence"], "allowed_evidence": case["allowed_evidence"], "evidence_ids": [], "attempts": [], "tokens": None, "error": None}
    collector = UsageCollector()
    provider_name = f"{PROVIDER_NAME}-{case['case_id']}"
    _V03Provider.active_collector = collector
    try:
        _register_provider(provider_name)
        indexing_started = time.monotonic()
        entities: list[Any] = []
        relations: list[Any] = []
        for document in case["documents"]:
            ner = NERExtractor(method="llm", provider=provider_name, llm_model="qwen2.5:0.5b", silent_fail=False, entity_types=["服务", "组件", "系统"], usage_collector=collector)
            extracted = ner.extract(document["text"])
            entities.extend(extracted)
            re = RelationExtractor(method="llm", provider=provider_name, llm_model="qwen2.5:0.5b", silent_fail=False, relation_types=["depends_on", "conflicts_with"], usage_collector=collector)
            relations.extend(re.extract(document["text"], extracted))
        graph = build_provenance_graph(relations, case["documents"])
        if not graph["relationships"]:
            raise RuntimeError("automatic extraction produced no source-validated graph relation")
        retrieved_graph = retrieve_provenance_graph(graph, case["question"])
        if not retrieved_graph["relationships"]:
            raise RuntimeError("graph retrieval found no source-validated relation for the question")
        row["indexing_ms"] = round((time.monotonic() - indexing_started) * 1000, 3)
        reasoner = GraphReasoner(provider=provider_name, model="qwen2.5:0.5b")
        query_attempts: list[dict[str, Any]] = []
        for phase in ("cold", "warm"):
            query_started = time.monotonic()
            phase_answer = reasoner.reason(retrieved_graph, case["question"], max_tokens=512, temperature=0, stage=f"query-{phase}")
            query_attempts.append({"query_phase": phase, "answer": phase_answer, "latency_ms": round((time.monotonic() - query_started) * 1000, 3)})
        answer = query_attempts[0]["answer"]
        row["answer"] = answer
        row["entities"] = [str(entity) for entity in entities]
        row["relations"] = [str(relation) for relation in relations]
        row["retrieved_graph"] = retrieved_graph
        row["query_attempts"] = query_attempts
        row["references"] = graph_references(retrieved_graph)
        row["evidence_ids"] = sorted({reference["evidence_id"] for reference in row["references"]})
        row.update(assess_answer(case, answer, set(row["evidence_ids"])))
        row["privacy_violation"] = bool(set(row["evidence_ids"]) - set(case["allowed_evidence"])) or observed_privacy_violation(answer, row["references"], case.get("forbidden_sources", []))
        row["status"] = "completed"
    except Exception as exc:
        row["status"] = "failed"; row["error"] = str(exc)
        collector.record_failure("pipeline", str(exc), prompt=case.get("question", ""))
    row["latency_ms"] = round((time.monotonic() - started) * 1000, 3)
    row["attempts"] = collector.attempts
    row["tokens"] = collector.total()
    _V03Provider.active_collector = None
    return row


def _run_native(case: Mapping[str, Any], native_artifact: Path | None) -> dict[str, Any]:
    """Select only the matching real native transaction; never duplicate a run over cases."""
    row = {"case_id": case["case_id"], "document_revision": case["document_revision"], "requested_mode": "native", "actual_mode": "native", "engine_version": None, "model_version": "qwen2.5:0.5b", "expected_evidence": case["expected_evidence"], "allowed_evidence": case["allowed_evidence"], "evidence_ids": [], "references": [], "tokens": None, "latency_ms": None, "privacy_violation": False, "unanswerable_correct": False}
    if not native_artifact or not native_artifact.is_file():
        row.update({"status": "failed", "error": "explicit V03_NATIVE_RESULT artifact is required; no default backend selected"}); return row
    actual = json.loads(native_artifact.read_text(encoding="utf-8"))
    matches = [item for item in actual.get("cases", []) if item.get("case_id") == case["case_id"]]
    if len(matches) != 1:
        row.update({"status": "failed", "error": "native artifact lacks exactly one matching case transaction", "failure_stage": "artifact_contract"})
        return row
    measured = matches[0]
    row.update({key: measured.get(key) for key in ("engine_version", "model_version", "tokens", "latency_ms", "status", "error", "failure_stage", "indexing_ms", "query_attempts", "state_actions", "unsupported_stage")})
    row["native_run_case_id"] = measured["case_id"]
    row["references"] = measured.get("references", [])
    row["answer"] = measured.get("answer", "")
    row["evidence_ids"] = list(measured.get("evidence_ids", []))
    row["privacy_violation"] = bool(set(row["evidence_ids"]) - set(case["allowed_evidence"])) or observed_privacy_violation(row["answer"], row["references"], measured.get("forbidden_sources", case.get("forbidden_sources", [])))
    row.update(assess_answer(case, row["answer"], set(row["evidence_ids"])))
    return row


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--backend", choices=("native", "semantica"), required=True)
    parser.add_argument("--dataset", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    cases, _ = _load_dataset(args.dataset)
    artifact = Path(os.environ["V03_NATIVE_RESULT"]) if os.environ.get("V03_NATIVE_RESULT") else None
    rows = [case if case.get("status") == "failed" else (_run_native(case, artifact) if args.backend == "native" else _run_semantica(case)) for case in cases]
    summary = _summary(rows)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8") as stream:
        for row in rows:
            stream.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")
    args.output.with_suffix(".summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
