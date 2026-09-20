"""Emit installed-distribution evidence for the frozen Semantica V01 contract."""

from __future__ import annotations

import argparse
import json
import platform
import sys
from hashlib import sha256
from importlib import import_module
from importlib.metadata import PackageNotFoundError, distribution, version
from inspect import signature
from pathlib import Path
from typing import Any


EXPERIMENTS_DIR = Path(__file__).resolve().parent
LOCK_PATH = EXPERIMENTS_DIR / "uv.lock"
EXPECTED_VERSION = "0.6.8"
EXPECTED_WHEEL_SHA256 = "0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7"
CAPABILITIES = {
    "NERExtractor": ("semantica.semantic_extract", "NERExtractor"),
    "RelationExtractor": ("semantica.semantic_extract", "RelationExtractor"),
    "ContextGraph": ("semantica.context", "ContextGraph"),
    "ContextRetriever": ("semantica.context", "ContextRetriever"),
    "Reasoner": ("semantica.reasoning", "Reasoner"),
    "GraphReasoner": ("semantica.reasoning", "GraphReasoner"),
    "GraphStore": ("semantica.graph_store", "GraphStore"),
}


def _inside(path: Path, parent: Path) -> bool:
    try:
        path.resolve().relative_to(parent.resolve())
    except ValueError:
        return False
    return True


def _record_capabilities() -> tuple[dict[str, dict[str, str]], list[str]]:
    records: dict[str, dict[str, str]] = {}
    failures: list[str] = []
    for name, (module_name, attribute_name) in CAPABILITIES.items():
        try:
            value = getattr(import_module(module_name), attribute_name)
            if not callable(value):
                raise TypeError("not callable")
            records[name] = {
                "status": "available",
                "signature": str(signature(value)),
                "evidence_layer": "actual-runtime",
            }
        except Exception as exc:  # evidence must retain import/signature failures
            records[name] = {
                "status": "unavailable",
                "signature": "",
                "evidence_layer": "actual-runtime",
                "reason": f"{type(exc).__name__}: {exc}",
            }
            failures.append(f"{name}: {records[name]['reason']}")
    return records, failures


def build_evidence(command: list[str]) -> dict[str, Any]:
    """Build evidence without invoking providers, storage, or constructors."""
    failures: list[str] = []
    lock_hash = sha256(LOCK_PATH.read_bytes()).hexdigest() if LOCK_PATH.exists() else ""
    if not lock_hash:
        failures.append(f"missing lock file: {LOCK_PATH}")
    elif EXPECTED_WHEEL_SHA256 not in LOCK_PATH.read_text():
        failures.append("lock does not contain the frozen Semantica wheel hash")

    installed_version: str | None
    distribution_path = ""
    package_file = ""
    try:
        installed_version = version("semantica")
        installed_distribution = distribution("semantica")
        distribution_path = str(installed_distribution.locate_file(""))
        package_file = str(Path(import_module("semantica").__file__).resolve())
        if not _inside(Path(package_file), Path(sys.prefix)):
            failures.append(f"semantica is not installed under sys.prefix: {package_file}")
    except (PackageNotFoundError, ImportError, AttributeError) as exc:
        installed_version = None
        failures.append(f"installed distribution unavailable: {type(exc).__name__}: {exc}")

    if installed_version != EXPECTED_VERSION:
        failures.append(f"installed semantica {installed_version!r} != {EXPECTED_VERSION!r}")

    capabilities, capability_failures = _record_capabilities()
    failures.extend(capability_failures)
    evidence = {
        "schema_version": 1,
        "python": {"version": sys.version, "executable": sys.executable, "prefix": sys.prefix},
        "platform": platform.platform(),
        "distribution": {
            "name": "semantica",
            "version": installed_version,
            "path": distribution_path,
            "package_file": package_file,
            "expected_wheel_sha256": EXPECTED_WHEEL_SHA256,
        },
        "lock_hash": lock_hash,
        "command": command,
        "exit_code": 0 if not failures else 1,
        "evidence_layer": "actual-runtime",
        "capabilities": capabilities,
        "limitations": [
            "No provider, model, persistent storage, Go authorization, or external service was invoked.",
            "The lock records the frozen wheel hash; installed files do not retain a wheel byte-for-byte hash.",
        ],
        "failures": failures,
    }
    return evidence


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    command = [str(Path(__file__).name), "--output", str(args.output)]
    evidence = build_evidence(command)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(evidence, indent=2, ensure_ascii=False) + "\n")
    if evidence["exit_code"]:
        print("V01 verification failed; see evidence output.", file=sys.stderr)
    return int(evidence["exit_code"])


if __name__ == "__main__":
    raise SystemExit(main())
