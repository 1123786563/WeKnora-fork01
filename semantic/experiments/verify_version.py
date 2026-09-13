"""V01 evidence generator for the frozen Semantica release.

Produces docs/superpowers/plans/semantica/capability-evidence.json recording
the pinned semantica version, interpreter, lock hash, upstream source
revision and the importable capability signatures. Every recorded outcome is
actually executed here; nothing is written as a bare boolean. Exits 0 only
when every required capability is verified and the contract test passes;
otherwise exits non-zero and still writes the failure reasons.

Usage (from the worktree root):
  uv run --project semantic/experiments python semantic/experiments/verify_version.py \
      --output docs/superpowers/plans/semantica/capability-evidence.json
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tomllib
import urllib.request
from datetime import datetime, timezone
from hashlib import sha256
from importlib import import_module
from importlib.metadata import PackageNotFoundError, version
from inspect import signature
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
EXPERIMENTS_DIR = REPO_ROOT / "semantic" / "experiments"
PYPROJECT_PATH = EXPERIMENTS_DIR / "pyproject.toml"
LOCK_PATH = EXPERIMENTS_DIR / "uv.lock"
CONTRACT_TEST = "semantic/experiments/test_import_contract.py"
PYTEST_COMMAND = [
    "uv", "run", "--project", "semantic/experiments", "python", "-m",
    "pytest", CONTRACT_TEST, "-q",
]

# Frozen upstream contract: module path -> callables downstream tasks may use.
CAPABILITY_CONTRACT = {
    "semantica.semantic_extract": ["NERExtractor", "RelationExtractor"],
    "semantica.context": ["ContextGraph", "ContextRetriever"],
    "semantica.reasoning": ["Reasoner", "GraphReasoner"],
    "semantica.graph_store": ["GraphStore"],
}

SEMANTICA_GITHUB = "semantica-agi/semantica"


def _load_pin() -> dict:
    """Read the pin from pyproject + uv.lock so evidence never drifts from
    the actual project definition (review V01: no hardcoded pin constants)."""
    pyproject = tomllib.loads(PYPROJECT_PATH.read_text())
    requirements = pyproject["project"]["dependencies"]
    pin_requirement = next(r for r in requirements if r.startswith("semantica"))
    match = re.search(r"semantica\[[^\]]*\]==([0-9][^ ;]*)", pin_requirement)
    pinned_version = match.group(1) if match else None

    lock_text = LOCK_PATH.read_text()
    package_block = re.search(
        r'name = "semantica"\nversion = "' + re.escape(str(pinned_version)) + r'"(.*?)(?=\n\n\[\[package\]\])',
        lock_text,
        re.S,
    )
    wheel = None
    if package_block:
        wheel = re.search(
            r'url = "([^"]+semantica-[^"]+\.whl)", hash = "sha256:([0-9a-f]{64})"',
            package_block.group(1),
        )
    return {
        "requirement": pin_requirement,
        "version": pinned_version,
        "wheel_url": wheel.group(1) if wheel else None,
        "wheel_sha256": wheel.group(2) if wheel else None,
    }


def _utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def _run(command: list[str]) -> dict:
    proc = subprocess.run(command, cwd=REPO_ROOT, capture_output=True, text=True)
    return {
        "command": " ".join(command),
        "exit_code": proc.returncode,
        "stdout_tail": proc.stdout.strip().splitlines()[-3:],
        "stderr_tail": proc.stderr.strip().splitlines()[-3:],
    }


def _resolve_source_revision(pinned_version: str) -> dict:
    """Resolve the release tag to its commit; never guess from the version."""
    base = f"https://api.github.com/repos/{SEMANTICA_GITHUB}"
    tag = f"v{pinned_version}"
    try:
        with urllib.request.urlopen(f"{base}/git/ref/tags/{tag}", timeout=30) as resp:
            ref = json.load(resp)
        obj = ref["object"]
        tag_url = obj["url"]
        tag_sha = obj["sha"]
        if obj["type"] == "tag":  # annotated tag -> dereference to commit
            with urllib.request.urlopen(tag_url, timeout=30) as resp:
                tag_obj = json.load(resp)
            commit_sha = tag_obj["object"]["sha"]
        else:
            commit_sha = tag_sha
        return {
            "status": "verified",
            "tag": tag,
            "tag_sha": tag_sha,
            "commit_sha": commit_sha,
            "source_url": f"https://github.com/{SEMANTICA_GITHUB}/tree/{commit_sha}",
            "release_url": f"https://github.com/{SEMANTICA_GITHUB}/releases/tag/{tag}",
        }
    except Exception as exc:  # noqa: BLE001 - record the concrete failure
        return {"status": "unavailable", "reason": f"{type(exc).__name__}: {exc}"}


def _verify_capabilities() -> dict:
    capabilities: dict[str, dict] = {}
    for module_path, names in CAPABILITY_CONTRACT.items():
        try:
            module = import_module(module_path)
        except Exception as exc:  # noqa: BLE001 - import failure is a finding
            for name in names:
                capabilities[f"{module_path}.{name}"] = {
                    "status": "unavailable",
                    "signature": None,
                    "evidence_path": CONTRACT_TEST,
                    "reason": f"import failed: {type(exc).__name__}: {exc}",
                }
            continue
        for name in names:
            attr = getattr(module, name, None)
            if attr is None or not callable(attr):
                capabilities[f"{module_path}.{name}"] = {
                    "status": "unavailable",
                    "signature": None,
                    "evidence_path": CONTRACT_TEST,
                    "reason": "missing or not callable",
                }
                continue
            capabilities[f"{module_path}.{name}"] = {
                "status": "verified",
                "signature": str(signature(attr)),
                "evidence_path": CONTRACT_TEST,
            }
    return capabilities


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, help="evidence JSON destination path")
    args = parser.parse_args()

    failures: list[str] = []

    pin = _load_pin()
    pinned_version = pin["version"]
    if pinned_version is None:
        failures.append("pyproject.toml does not pin semantica with '==<version>'")

    try:
        semantica_version = version("semantica")
    except PackageNotFoundError:
        semantica_version = None
        failures.append("semantica package not installed")

    # Review V01: evidence must fail closed when the installed release is not
    # the pinned one; uv.lock enforces it, this cross-check keeps the record
    # honest even if the environment drifts.
    if semantica_version is not None and pinned_version is not None and semantica_version != pinned_version:
        failures.append(
            f"installed semantica {semantica_version} != pinned {pinned_version}"
        )

    lock_hash = sha256(LOCK_PATH.read_bytes()).hexdigest() if LOCK_PATH.exists() else None
    if lock_hash is None:
        failures.append(f"lock file missing: {LOCK_PATH}")

    capabilities = _verify_capabilities()
    failures.extend(
        f"capability {name}: {cap['reason']}"
        for name, cap in capabilities.items()
        if cap["status"] != "verified"
    )

    source_revision = _resolve_source_revision(pinned_version or "unknown")
    if source_revision["status"] != "verified":
        failures.append(f"source_revision: {source_revision['reason']}")

    pytest_result = _run(PYTEST_COMMAND)
    if pytest_result["exit_code"] != 0:
        failures.append(f"contract test exit code {pytest_result['exit_code']}")

    evidence = {
        "evidence.schema_version": 1,
        "generated_at": _utc_now(),
        "python_version": sys.version.replace("\n", " "),
        "semantica_version": semantica_version,
        "lock_hash": lock_hash,
        "pin": {
            "python": (EXPERIMENTS_DIR / ".python-version").read_text().strip(),
            "requirement": pin["requirement"],
            "pinned_version": pinned_version,
            "wheel_url": pin["wheel_url"],
            "wheel_sha256": pin["wheel_sha256"],
        },
        "source_revision": source_revision,
        "capabilities": capabilities,
        "contract_test": pytest_result,
        "failures": failures,
    }

    output_path = (REPO_ROOT / args.output) if not Path(args.output).is_absolute() else Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(evidence, ensure_ascii=False, indent=2) + "\n")

    if failures:
        print(f"FAILED: {len(failures)} problem(s) recorded in {output_path}", file=sys.stderr)
        for failure in failures:
            print(f"  - {failure}", file=sys.stderr)
        return 1
    print(f"OK: evidence written to {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
