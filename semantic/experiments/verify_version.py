"""Write V01's hash-checked Semantica public-import evidence.

The script is deliberately strict: every failed precondition writes structured
evidence and exits non-zero.  It validates the installed distribution rather
than loading source directly from the release wheel.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import traceback
from hashlib import sha256
from importlib.metadata import PackageNotFoundError, version
from inspect import signature
from pathlib import Path
from typing import Any


ROOT = Path(__file__).parent
EXPECTED_VERSION = "0.6.8"
EXPECTED_WHEEL_SHA256 = "0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7"
# Provenance captured from the official v0.6.8 GitHub tag API in the retained
# bounded-probe evidence; it identifies the release source, not this worktree.
SOURCE_REVISION = {
    "tag": "v0.6.8",
    "tag_object_sha": "29f3c3230cb72e6b84201a0f5f4e310929fefb47",
    "commit_sha": "f73f599a22c320676a45f247247038ca1fcf40f0",
    "url": "https://github.com/semantica-agi/semantica/tree/v0.6.8",
}
EVIDENCE_PATH = "docs/superpowers/plans/semantica/capability-evidence.json"


def _write(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def _base_payload(wheel: Path | None) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "python_version": sys.version,
        "python_executable": sys.executable,
        "semantica_version": None,
        "source_revision": SOURCE_REVISION,
        "lock_hash": None,
        "lock_wheel_sha256": None,
        "wheel_path": str(wheel) if wheel else None,
        "wheel_sha256": None,
        "expected_wheel_sha256": EXPECTED_WHEEL_SHA256,
        "capabilities": {},
        "command": {"argv": sys.argv, "exit_code": None},
    }


def _failure(payload: dict[str, Any], kind: str, detail: str) -> int:
    payload["status"] = "failed"
    payload["failure"] = {"kind": kind, "detail": detail}
    payload["command"]["exit_code"] = 1
    return 1


def _imports() -> dict[str, object]:
    from semantica.context import ContextGraph, ContextRetriever
    from semantica.graph_store import GraphStore
    from semantica.reasoning import GraphReasoner, Reasoner
    from semantica.semantic_extract import NERExtractor, RelationExtractor

    return {
        "NERExtractor": NERExtractor,
        "RelationExtractor": RelationExtractor,
        "ContextGraph": ContextGraph,
        "ContextRetriever": ContextRetriever,
        "Reasoner": Reasoner,
        "GraphReasoner": GraphReasoner,
        "GraphStore": GraphStore,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--wheel", type=Path, default=os.environ.get("SEMANTICA_WHEEL"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    wheel = args.wheel
    payload = _base_payload(wheel)
    try:
        lock_bytes = (ROOT / "uv.lock").read_bytes()
        payload["lock_hash"] = sha256(lock_bytes).hexdigest()
        if f"sha256:{EXPECTED_WHEEL_SHA256}".encode() not in lock_bytes:
            result = _failure(payload, "lock-wheel-hash-missing", "uv.lock does not contain the frozen Semantica wheel hash")
        elif wheel is None:
            result = _failure(payload, "wheel-not-specified", "pass --wheel or set SEMANTICA_WHEEL; the verifier never downloads a wheel")
        elif not wheel.is_file():
            result = _failure(payload, "wheel-not-found", f"wheel does not exist: {wheel}")
        else:
            payload["wheel_path"] = str(wheel)
            payload["wheel_sha256"] = sha256(wheel.read_bytes()).hexdigest()
            payload["lock_wheel_sha256"] = EXPECTED_WHEEL_SHA256
            if payload["wheel_sha256"] != EXPECTED_WHEEL_SHA256:
                result = _failure(payload, "wheel-sha256-mismatch", "wheel digest differs from frozen Semantica 0.6.8 release")
            else:
                installed_version = version("semantica")
                payload["semantica_version"] = installed_version
                if installed_version != EXPECTED_VERSION:
                    result = _failure(payload, "distribution-version-mismatch", f"installed {installed_version}, expected {EXPECTED_VERSION}")
                else:
                    exports = _imports()
                    if not all(callable(export) for export in exports.values()):
                        result = _failure(payload, "non-callable-public-export", "one or more required public exports are not callable")
                    else:
                        payload["capabilities"] = {
                            name: {"status": "passed", "signature": str(signature(export)), "evidence_path": EVIDENCE_PATH}
                            for name, export in exports.items()
                        }
                        payload["status"] = "passed"
                        payload["command"]["exit_code"] = 0
                        result = 0
    except FileNotFoundError:
        result = _failure(payload, "lockfile-not-found", f"missing lockfile: {ROOT / 'uv.lock'}")
    except PackageNotFoundError:
        result = _failure(payload, "distribution-not-installed", "Semantica distribution metadata is unavailable")
    except Exception as exc:  # imports must leave a replayable, structured failure
        result = _failure(payload, "public-import-contract-failed", f"{type(exc).__name__}: {exc}")
        payload["failure"]["traceback"] = traceback.format_exc(limit=8)
    _write(args.output, payload)
    return result


if __name__ == "__main__":
    raise SystemExit(main())
