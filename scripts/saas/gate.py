"""Admission gate for the experimentally validated commercial model.

The gate intentionally accepts only runtime evidence.  Schema or documentation
evidence is useful input to an experiment, but cannot authorize an adapter.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path

REQUIRED_CHECKS = {f"OM-{number:02d}" for number in range(1, 10)}


def validate_gate(report: dict) -> None:
    if not isinstance(report, dict):
        raise ValueError("gate report must be an object")
    if report.get("family") not in {"official_v1_v2", "official_v3"}:
        raise ValueError("one official family required")
    checks = report.get("checks")
    if not isinstance(checks, dict):
        raise ValueError("commercial capability gate incomplete: checks required")
    failed = sorted(key for key in REQUIRED_CHECKS if checks.get(key) != "pass")
    if failed:
        raise ValueError("commercial capability gate incomplete: " + ", ".join(failed))
    evidence = report.get("evidence")
    if not isinstance(evidence, (dict, list)) or not evidence:
        raise ValueError("runtime evidence and acknowledgement strategy required")
    if not isinstance(report.get("settlement_ack_strategy"), str) or not report["settlement_ack_strategy"].strip():
        raise ValueError("runtime evidence and acknowledgement strategy required")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="validate the selected commercial model gate")
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        validate_gate(json.loads(args.report.read_text(encoding="utf-8")))
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        print(f"gate blocked: {exc}")
        return 2
    print("gate passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
