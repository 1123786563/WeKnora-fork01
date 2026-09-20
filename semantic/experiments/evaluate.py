"""Deterministic, offline-only scoring for Semantica evaluation observations."""

from __future__ import annotations

import argparse
import hashlib
import json
from math import ceil
from pathlib import Path
import tempfile


def score_case(case: dict[str, object], observation: dict[str, object]) -> dict[str, object]:
    """Score one observation against one immutable evaluation case."""
    expected_evidence = set(case["expected_evidence_ids"])
    actual_evidence = set(observation["actual_evidence_ids"])
    forbidden_evidence = set(case["forbidden_evidence_ids"])
    expected_conclusions = set(case["expected_conclusion_ids"])
    actual_conclusions = set(observation["actual_conclusion_ids"])
    access_violations = observation.get("access_violations", [])

    evidence_overlap = expected_evidence & actual_evidence
    source_precision = (
        len(evidence_overlap) / len(actual_evidence)
        if actual_evidence
        else (1.0 if not expected_evidence else 0.0)
    )
    source_recall = (
        len(evidence_overlap) / len(expected_evidence)
        if expected_evidence
        else 1.0
    )
    unexpected_evidence = actual_evidence - expected_evidence
    permission_leak = (
        bool(unexpected_evidence)
        or bool(actual_evidence & forbidden_evidence)
        or bool(access_violations)
    )
    mode_match = (
        case["requested_mode"] == observation["requested_mode"]
        and case["requested_mode"] == observation["actual_mode"]
    )
    unanswerable_correct = (
        case["answerable"] is False
        and observation["actual_status"] == "insufficient_evidence"
        and not actual_evidence
        and not actual_conclusions
    )
    correct = (
        actual_conclusions == expected_conclusions
        and expected_evidence <= actual_evidence
        and not permission_leak
        and mode_match
        and (
            observation["actual_status"] == "answered"
            if case["answerable"] is True
            else unanswerable_correct
        )
    )

    return {
        "source_precision": source_precision,
        "source_recall": source_recall,
        "correct": correct,
        "unanswerable_correct": unanswerable_correct if case["answerable"] is False else None,
        "permission_leak": permission_leak,
        "mode_match": mode_match,
        "hard_gate_pass": not permission_leak and mode_match,
    }


def aggregate_cases(rows: list[dict[str, object]]) -> dict[str, object]:
    """Aggregate score rows while preserving unavailable runtime measurements."""
    def mean(name: str) -> float | None:
        values = [float(row[name]) for row in rows if row.get(name) is not None]
        return sum(values) / len(values) if values else None

    def total(name: str) -> int | float | None:
        values = [row[name] for row in rows if row.get(name) is not None]
        return sum(values) if values else None

    def percentile(name: str, percent: float) -> int | float | None:
        values = sorted(row[name] for row in rows if row.get(name) is not None)
        if not values:
            return None
        return values[max(1, ceil(percent * len(values))) - 1]

    unanswerable_rows = [row for row in rows if row.get("unanswerable_correct") is not None]
    return {
        "case_count": len(rows),
        "correctness_rate": mean("correct"),
        "mean_source_precision": mean("source_precision"),
        "mean_source_recall": mean("source_recall"),
        "unanswerable_accuracy": (
            sum(bool(row["unanswerable_correct"]) for row in unanswerable_rows)
            / len(unanswerable_rows)
            if unanswerable_rows
            else None
        ),
        "hard_gate_failures": sum(
            not bool(row.get("hard_gate_pass", not row.get("permission_leak") and row.get("mode_match")))
            for row in rows
        ),
        "permission_leak_count": sum(bool(row.get("permission_leak")) for row in rows),
        "mode_mismatch_count": sum(not bool(row.get("mode_match")) for row in rows),
        "latency_p50_ms": percentile("latency_ms", 0.50),
        "latency_p95_ms": percentile("latency_ms", 0.95),
        "reported_input_tokens": total("input_tokens"),
        "reported_output_tokens": total("output_tokens"),
    }


CASE_REQUIRED_FIELDS = {
    "case_id",
    "scenario",
    "synthetic",
    "tenant_id",
    "kb_id",
    "document_revisions",
    "query",
    "requested_mode",
    "expected_evidence_ids",
    "expected_conclusion_ids",
    "answerable",
    "forbidden_evidence_ids",
}
OBSERVATION_REQUIRED_FIELDS = {
    "case_id",
    "backend",
    "requested_mode",
    "actual_mode",
    "actual_status",
    "actual_evidence_ids",
    "actual_conclusion_ids",
    "access_violations",
    "evidence_layer",
}
OPTIONAL_MEASUREMENTS = (
    "latency_ms",
    "input_tokens",
    "output_tokens",
    "engine_version",
    "model_version",
)


def _read_jsonl(path: Path, required_fields: set[str], record_name: str) -> list[dict[str, object]]:
    records: list[dict[str, object]] = []
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as error:
        raise ValueError(f"cannot read {record_name} JSONL: {path}") from error
    if not lines:
        raise ValueError(f"{record_name} JSONL must contain at least one record")
    for line_number, line in enumerate(lines, start=1):
        if not line.strip():
            raise ValueError(f"{record_name} JSONL contains a blank line at {line_number}")
        try:
            record = json.loads(line)
        except json.JSONDecodeError as error:
            raise ValueError(f"invalid {record_name} JSON at line {line_number}") from error
        if not isinstance(record, dict):
            raise ValueError(f"{record_name} at line {line_number} must be an object")
        missing = required_fields - record.keys()
        if missing:
            raise ValueError(f"{record_name} at line {line_number} is missing: {sorted(missing)}")
        records.append(record)
    return records


def _require_unique_case_ids(records: list[dict[str, object]], record_name: str) -> None:
    case_ids = [record.get("case_id") for record in records]
    if any(not isinstance(case_id, str) or not case_id for case_id in case_ids):
        raise ValueError(f"every {record_name} requires a non-empty string case_id")
    if len(case_ids) != len(set(case_ids)):
        raise ValueError(f"duplicate {record_name} case_id")


def load_cases(path: Path) -> list[dict[str, object]]:
    """Load immutable synthetic cases without normalizing their text fields."""
    cases = _read_jsonl(path, CASE_REQUIRED_FIELDS, "case")
    _require_unique_case_ids(cases, "case")
    for case in cases:
        if case["synthetic"] is not True:
            raise ValueError("offline V03 cases must have synthetic: true")
        if not isinstance(case["document_revisions"], dict):
            raise ValueError("case document_revisions must be an object")
        for name in ("expected_evidence_ids", "expected_conclusion_ids", "forbidden_evidence_ids"):
            if not isinstance(case[name], list) or not all(isinstance(item, str) for item in case[name]):
                raise ValueError(f"case {name} must be a string list")
        if set(case["expected_evidence_ids"]) & set(case["forbidden_evidence_ids"]):
            raise ValueError("case forbidden evidence cannot be expected evidence")
        if not isinstance(case["answerable"], bool):
            raise ValueError("case answerable must be boolean")
    return cases


def load_observations(path: Path) -> list[dict[str, object]]:
    """Load observations only; this function never invokes a backend."""
    observations = _read_jsonl(path, OBSERVATION_REQUIRED_FIELDS, "observation")
    _require_unique_case_ids(observations, "observation")
    for observation in observations:
        for name in ("actual_evidence_ids", "actual_conclusion_ids", "access_violations"):
            if not isinstance(observation[name], list) or not all(isinstance(item, str) for item in observation[name]):
                raise ValueError(f"observation {name} must be a string list")
        for name in OPTIONAL_MEASUREMENTS:
            observation.setdefault(name, None)
    return observations


def _lock_sha256() -> str | None:
    lock_path = Path(__file__).with_name("uv.lock")
    return hashlib.sha256(lock_path.read_bytes()).hexdigest() if lock_path.is_file() else None


def evaluate_run(
    cases: list[dict[str, object]], observations: list[dict[str, object]]
) -> dict[str, object]:
    """Join an exact case/observation set and emit deterministic offline scores."""
    if not cases or not observations:
        raise ValueError("evaluation requires at least one case and one observation")
    _require_unique_case_ids(cases, "case")
    _require_unique_case_ids(observations, "observation")
    cases_by_id = {case["case_id"]: case for case in cases}
    observations_by_id = {observation["case_id"]: observation for observation in observations}
    missing = set(cases_by_id) - set(observations_by_id)
    extra = set(observations_by_id) - set(cases_by_id)
    if missing or extra:
        raise ValueError(f"case/observation IDs do not match (missing={sorted(missing)}, extra={sorted(extra)})")

    rows: list[dict[str, object]] = []
    for case in cases:
        observation = observations_by_id[case["case_id"]]
        score = score_case(case, observation)
        rows.append(
            {
                "case_id": case["case_id"],
                "backend": observation["backend"],
                "evidence_layer": observation["evidence_layer"],
                "latency_ms": observation["latency_ms"],
                "input_tokens": observation["input_tokens"],
                "output_tokens": observation["output_tokens"],
                **score,
            }
        )
    evidence_layers = {observation["evidence_layer"] for observation in observations}
    all_synthetic = all(case.get("synthetic") is True for case in cases) and evidence_layers == {"synthetic"}
    return {
        "schema_version": 1,
        "evaluator": "semantica-v03-offline",
        "lock_sha256": _lock_sha256(),
        "evidence_layer": next(iter(evidence_layers)) if len(evidence_layers) == 1 else "mixed",
        "synthetic_only": all_synthetic,
        "per_case_scores": rows,
        "aggregate_metrics": aggregate_cases(rows),
        "hard_gate_pass": all(bool(row["hard_gate_pass"]) for row in rows),
    }


def _write_report_atomically(path: Path, report: dict[str, object]) -> None:
    with tempfile.NamedTemporaryFile(
        mode="w", encoding="utf-8", dir=path.parent, prefix=f".{path.name}.", delete=False
    ) as temporary_file:
        temporary_file.write(json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True))
        temporary_file.write("\n")
        temporary_path = Path(temporary_file.name)
    temporary_path.replace(path)


def main() -> int:
    parser = argparse.ArgumentParser(description="Score pre-recorded Semantica evaluation observations offline.")
    parser.add_argument("--dataset", required=True, type=Path)
    parser.add_argument("--observations", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    report = evaluate_run(load_cases(args.dataset), load_observations(args.observations))
    _write_report_atomically(args.output, report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
