"""Release gate for the SaaS billing connectors acceptance evidence.

Mirroring scripts/saas/gate.py (V03): only runtime evidence recorded in the
progress ledger authorizes a check.  skip / blocked-env / pending rows and mock
substitutes for real providers never pass, and the CLI always lists the
unfinished scope -- including capabilities excluded via --without.
"""
from __future__ import annotations

import argparse
import json
import re
from pathlib import Path

PROVIDER_CASES = {"WX-05", "ALI-05", "FS-04", "NO-04"}

CAPABILITIES = {
    "om": ("OM",),
    "com": ("COM",),
    "wx": ("WX",),
    "ali": ("ALI",),
    "bud": ("BUD",),
    "use": ("USE",),
    "con": ("CON",),
    "fs": ("FS",),
    "no": ("NO",),
    "sync": ("SYNC",),
    "ops": ("OPS",),
    "product": ("AC",),
}

_INTERFACE_ID = re.compile(
    r"^\|\s*((?:OM|COM|WX|ALI|BUD|USE|CON|FS|NO|SYNC|OPS)-\d{2})\s*\|", re.MULTILINE
)
_AC_ID = re.compile(r"^\|\s*(AC-\d{2})\s*\|", re.MULTILINE)

DEFAULT_SPEC = Path("docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md")
DEFAULT_DESIGN = Path("docs/superpowers/specs/2026-09-10-saas-billing-connectors-design.md")


def require_pass(records: dict, required: set[str]) -> None:
    provider_cases = {"WX-05", "ALI-05", "FS-04", "NO-04"}
    for key in required:
        row = records.get(key, {})
        if row.get("status") != "pass" or not row.get("evidence"):
            raise ValueError(f"missing passing evidence: {key}")
        if key in provider_cases and row.get("level") != "provider":
            raise ValueError(f"real provider evidence required: {key}")


def parse_required_ids(spec_path: Path | str, design_path: Path | str) -> set[str]:
    """Read the 66 interface check IDs and AC-01..AC-20 from the spec tables."""
    ids = set(_INTERFACE_ID.findall(Path(spec_path).read_text(encoding="utf-8")))
    ids.update(_AC_ID.findall(Path(design_path).read_text(encoding="utf-8")))
    return ids


def capability_prefixes(without: set[str], all_ids: set[str]) -> set[str]:
    """IDs excluded from the required set by --without capability selections."""
    excluded: set[str] = set()
    for capability in without:
        prefixes = CAPABILITIES[capability]
        excluded.update(i for i in all_ids if i.split("-", 1)[0] in prefixes)
    return excluded


def unfinished_scope(records: dict, required: set[str], excluded: set[str]) -> list[dict]:
    """Every required-but-not-passing ID plus every excluded ID, with reasons."""
    rows: list[dict] = []
    for key in sorted(required | excluded):
        row = records.get(key, {})
        if key in excluded:
            rows.append({"id": key, "status": row.get("status", "missing"), "reason": "excluded"})
            continue
        if row.get("status") == "pass" and row.get("evidence"):
            continue
        status = row.get("status", "missing")
        if key in PROVIDER_CASES:
            reason = "real provider evidence required"
        else:
            reason = status
        rows.append({"id": key, "status": status, "reason": reason})
    return rows


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="validate SaaS billing connector acceptance evidence against the spec checklists"
    )
    parser.add_argument("--report", type=Path, required=True,
                        help="JSON report with a records object keyed by check ID")
    parser.add_argument("--spec", type=Path, default=DEFAULT_SPEC)
    parser.add_argument("--design", type=Path, default=DEFAULT_DESIGN)
    parser.add_argument("--without", action="append", default=[], metavar="CAPABILITY",
                        choices=sorted(CAPABILITIES),
                        help="exclude a capability's IDs from the required set (repeatable); "
                             "the excluded scope is still listed as unfinished")
    args = parser.parse_args(argv)
    try:
        report = json.loads(args.report.read_text(encoding="utf-8"))
        records = report.get("records", {}) if isinstance(report, dict) else {}
        required = parse_required_ids(args.spec, args.design)
    except (OSError, json.JSONDecodeError) as exc:
        print(f"gate blocked: {exc}")
        return 2
    excluded = capability_prefixes(set(args.without), required)
    active = required - excluded
    try:
        require_pass(records, active)
    except ValueError as exc:
        print(f"gate blocked: {exc}")
        for row in unfinished_scope(records, active, excluded):
            print(f"  unfinished: {row['id']} status={row['status']} reason={row['reason']}")
        return 1
    unfinished = unfinished_scope(records, active, excluded)
    for row in unfinished:
        print(f"  unfinished (excluded scope): {row['id']} status={row['status']} reason={row['reason']}")
    if unfinished:
        print("gate passed with excluded scope: " + ", ".join(row["id"] for row in unfinished))
    else:
        print("gate passed: all required checks have runtime evidence")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
