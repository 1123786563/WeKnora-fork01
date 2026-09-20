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

    def total(self) -> dict[str, int]:
        return {key: sum(int(entry["raw_usage"][key]) for entry in self.attempts if entry.get("raw_usage")) for key in ("prompt_tokens", "completion_tokens", "total_tokens")}


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
    expected_terms = [str(term) for term in case.get("expected_answer_terms", [])]
    correct = bool(actual_evidence) and bool(expected_terms) and all(term in answer for term in expected_terms)
    return {"correct": correct, "unanswerable_correct": False}


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
    def __init__(self, **kwargs: Any) -> None:
        super().__init__(**kwargs)
        self.collector: UsageCollector | None = kwargs.pop("usage_collector", None)
        self.calls: list[dict[str, Any]] = []

    def generate(self, prompt: str, **kwargs: Any) -> str:
        request = urllib.request.Request(f"{GATEWAY}/v1/semantica-v03/extract", data=json.dumps({"prompt": prompt}, ensure_ascii=False).encode(), headers={"Content-Type": "application/json"}, method="POST")
        started = time.monotonic()
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                payload = json.loads(response.read())
        except (urllib.error.URLError, urllib.error.HTTPError, json.JSONDecodeError) as exc:
            self.calls.append({"stage": kwargs.get("stage", "generate"), "status": "failed", "latency_ms": round((time.monotonic() - started) * 1000, 3), "error": str(exc), "raw_usage": None})
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


def _register_provider() -> None:
    provider_registry.register(PROVIDER_NAME, _V03Provider)


def _source_valid_evidence(case: Mapping[str, Any], answer: str) -> set[str]:
    """Only quote-backed text found in both source and actual answer becomes evidence."""
    source = {item["evidence_id"]: item["quote"] for item in case.get("documents", []) if "evidence_id" in item and "quote" in item}
    return {evidence_id for evidence_id, quote in source.items() if quote and quote in answer and quote in str(next((item.get("text", "") for item in case["documents"] if item.get("evidence_id") == evidence_id), ""))}


def _run_semantica(case: Mapping[str, Any]) -> dict[str, Any]:
    started = time.monotonic()
    row = {"case_id": case["case_id"], "document_revision": case["document_revision"], "requested_mode": "semantica", "actual_mode": "semantica-llm-ner-re", "engine_version": version("semantica"), "model_version": "qwen2.5:0.5b", "expected_evidence": case["expected_evidence"], "allowed_evidence": case["allowed_evidence"], "evidence_ids": [], "attempts": [], "tokens": None, "error": None}
    try:
        _register_provider()
        collector = UsageCollector()
        provider = _V03Provider(usage_collector=collector)
        entities: list[Any] = []
        relations: list[Any] = []
        for document in case["documents"]:
            ner = NERExtractor(method="llm", provider=PROVIDER_NAME, llm_model="qwen2.5:0.5b", silent_fail=False, entity_types=["服务", "组件", "系统"], usage_collector=collector)
            extracted = ner.extract(document["text"])
            entities.extend(extracted)
            re = RelationExtractor(method="llm", provider=PROVIDER_NAME, llm_model="qwen2.5:0.5b", silent_fail=False, relation_types=["depends_on", "conflicts_with"], usage_collector=collector)
            relations.extend(re.extract(document["text"], extracted))
        query_prompt = "只根据以下中文原文回答问题；逐字引用支持答案的原句。若证据不足，回答‘证据不足’。\n问题：" + case["question"] + "\n原文：\n" + "\n".join(document["text"] for document in case["documents"])
        answer = provider.generate(query_prompt, stage="query")
        row["answer"] = answer
        row["entities"] = [str(entity) for entity in entities]
        row["relations"] = [str(relation) for relation in relations]
        row["evidence_ids"] = sorted(_source_valid_evidence(case, answer))
        row.update(assess_answer(case, answer, set(row["evidence_ids"])))
        row["privacy_violation"] = bool(set(row["evidence_ids"]) - set(case["allowed_evidence"]))
        row["attempts"] = collector.attempts
        row["tokens"] = collector.total() if collector.attempts else None
        row["status"] = "completed"
    except Exception as exc:
        row["status"] = "failed"; row["error"] = str(exc)
    row["latency_ms"] = round((time.monotonic() - started) * 1000, 3)
    return row


def _run_native(case: Mapping[str, Any], native_artifact: Path | None) -> dict[str, Any]:
    """Retain the real native run outcome; never turn allowed scope into refs."""
    row = {"case_id": case["case_id"], "document_revision": case["document_revision"], "requested_mode": "native", "actual_mode": "native", "engine_version": None, "model_version": "qwen2.5:0.5b", "expected_evidence": case["expected_evidence"], "allowed_evidence": case["allowed_evidence"], "evidence_ids": [], "references": [], "tokens": None, "latency_ms": None, "privacy_violation": False, "unanswerable_correct": False}
    if not native_artifact or not native_artifact.is_file():
        row.update({"status": "failed", "error": "explicit V03_NATIVE_RESULT artifact is required; no default backend selected"}); return row
    actual = json.loads(native_artifact.read_text(encoding="utf-8"))
    row.update({key: actual.get(key) for key in ("engine_version", "model_version", "tokens", "latency_ms", "status", "error")})
    row["native_run_case_id"] = actual.get("case_id")
    row["failure_stage"] = actual.get("failure_stage")
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
