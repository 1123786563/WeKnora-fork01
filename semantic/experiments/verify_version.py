"""Emit installed-distribution evidence for the frozen Semantica V01 contract."""

from __future__ import annotations

import argparse
import json
import platform
import sys
import tomllib
from hashlib import sha256
from importlib import import_module
from importlib.metadata import PackageNotFoundError, distribution, version
from inspect import signature
from pathlib import Path
from typing import Any


EXPERIMENTS_DIR = Path(__file__).resolve().parent
REPO_ROOT = EXPERIMENTS_DIR.parents[1]
EVIDENCE_ROOT = REPO_ROOT / "docs" / "plans" / "semantica" / "evidence"
LOCK_PATH = EXPERIMENTS_DIR / "uv.lock"
EXPECTED_VERSION = "0.6.8"
EXPECTED_WHEEL_SHA256 = "0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7"
CAPABILITIES = {
    "NERExtractor": ("semantica.semantic_extract", "NERExtractor", "(method: Union[str, List[str]] = 'ml', entity_types: Optional[List[str]] = None, **config)"),
    "RelationExtractor": ("semantica.semantic_extract", "RelationExtractor", "(method: Union[str, List[str]] = 'pattern', relation_types: Optional[List[str]] = None, bidirectional: bool = False, confidence_threshold: float = 0.6, max_distance: int = 50, **config)"),
    "ContextGraph": ("semantica.context", "ContextGraph", "(config: Optional[Dict[str, Any]] = None, **kwargs)"),
    "ContextRetriever": ("semantica.context", "ContextRetriever", "(config: Optional[Dict[str, Any]] = None, **kwargs)"),
    "Reasoner": ("semantica.reasoning", "Reasoner", "(**kwargs)"),
    "GraphReasoner": ("semantica.reasoning", "GraphReasoner", "(core=None, config=None, **kwargs)"),
    "GraphStore": ("semantica.graph_store", "GraphStore", "(backend: Optional[str] = None, **config)"),
}


def _inside(path: Path, parent: Path) -> bool:
    try:
        path.resolve().relative_to(parent.resolve())
    except ValueError:
        return False
    return True


def _locked_semantica(lock_path: Path = LOCK_PATH) -> dict[str, Any]:
    packages = tomllib.loads(lock_path.read_text()).get("package", [])
    matching = [package for package in packages if package.get("name") == "semantica"]
    if len(matching) != 1:
        raise ValueError("lock must contain exactly one semantica package entry")
    package = matching[0]
    if package.get("version") != EXPECTED_VERSION:
        raise ValueError(f"lock semantica version is not {EXPECTED_VERSION}")
    hashes = {wheel.get("hash") for wheel in package.get("wheels", [])}
    if f"sha256:{EXPECTED_WHEEL_SHA256}" not in hashes:
        raise ValueError("lock semantica entry lacks the frozen wheel hash")
    return package


def evidence_output_path(output: Path | str) -> Path:
    candidate = Path(output)
    resolved = candidate.resolve() if candidate.is_absolute() else (REPO_ROOT / candidate).resolve()
    if not _inside(resolved, EVIDENCE_ROOT):
        raise ValueError(f"output must be under evidence root: {EVIDENCE_ROOT}")
    return resolved


def _record_capabilities(
    contract: dict[str, tuple[str, str, str]] = CAPABILITIES,
    importer: Any = import_module,
) -> tuple[dict[str, dict[str, str]], list[str]]:
    records: dict[str, dict[str, str]] = {}
    failures: list[str] = []
    for name, (module_name, attribute_name, expected_signature) in contract.items():
        try:
            value = getattr(importer(module_name), attribute_name)
            if not callable(value):
                raise TypeError("not callable")
            observed_signature = str(signature(value))
            if observed_signature != expected_signature:
                raise TypeError(f"expected {expected_signature}, observed {observed_signature}")
            records[name] = {
                "status": "available",
                "signature": observed_signature,
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
    else:
        try:
            _locked_semantica()
        except (OSError, ValueError, tomllib.TOMLDecodeError) as exc:
            failures.append(f"lock provenance failed: {exc}")

    installed_version: str | None
    distribution_path = ""
    package_file = ""
    try:
        installed_version = version("semantica")
        installed_distribution = distribution("semantica")
        distribution_root = Path(installed_distribution.locate_file(""))
        distribution_path = str(distribution_root)
        package_file = str(Path(import_module("semantica").__file__).resolve())
        declared_init = next((item for item in (installed_distribution.files or []) if str(item) == "semantica/__init__.py"), None)
        if declared_init is None or Path(package_file) != Path(installed_distribution.locate_file(declared_init)).resolve():
            failures.append("imported semantica package does not match distribution metadata")
        if not _inside(Path(package_file), distribution_root) or not _inside(Path(package_file), Path(sys.prefix)):
            failures.append(f"semantica is not installed under sys.prefix: {package_file}")
    except (PackageNotFoundError, ImportError, AttributeError) as exc:
        installed_version = None
        failures.append(f"installed distribution unavailable: {type(exc).__name__}: {exc}")

    if installed_version != EXPECTED_VERSION:
        failures.append(f"installed semantica {installed_version!r} != {EXPECTED_VERSION!r}")

    capabilities, capability_failures = _record_capabilities()
    failures.extend(capability_failures)
    if failures:
        for capability in capabilities.values():
            if capability["status"] == "available":
                capability.update({"status": "unavailable", "reason": "distribution provenance verification failed"})
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
    output_path = evidence_output_path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    evidence["artifact_path"] = str(output_path.relative_to(REPO_ROOT))
    output_path.write_text(json.dumps(evidence, indent=2, ensure_ascii=False) + "\n")
    if evidence["exit_code"]:
        print("V01 verification failed; see evidence output.", file=sys.stderr)
    return int(evidence["exit_code"])


if __name__ == "__main__":
    raise SystemExit(main())
