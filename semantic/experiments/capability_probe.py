"""Bounded, non-production probe for the published Semantica wheel.

It deliberately does not synthesize Semantica modules or call a remote model.
When the wheel cannot satisfy a public import, the corresponding result remains
an actual-runtime import failure; API findings read from that exact wheel are
labelled source-only.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path


WHEEL_SHA256 = "0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7"
TAG_OBJECT_SHA = "29f3c3230cb72e6b84201a0f5f4e310929fefb47"
SOURCE_COMMIT_SHA = "f73f599a22c320676a45f247247038ca1fcf40f0"


def _source(wheel: Path, member: str) -> str:
    with zipfile.ZipFile(wheel) as archive:
        return archive.read(member).decode("utf-8")


def _public_import(wheel: Path, interpreter: str) -> dict[str, object]:
    with tempfile.TemporaryDirectory(prefix="semantica-wheel-") as directory:
        with zipfile.ZipFile(wheel) as archive:
            archive.extractall(directory)
        environment = dict(os.environ, PYTHONPATH=directory)
        command = [
            interpreter,
            "-c",
            "from semantica.reasoning import Reasoner, GraphReasoner; "
            "from semantica.context import ContextGraph, ContextRetriever; print('ok')",
        ]
        try:
            result = subprocess.run(command, capture_output=True, text=True, env=environment, check=False, timeout=30)
        except subprocess.TimeoutExpired as exc:
            return {"classification": "actual-runtime", "command": command, "exit_code": None,
                    "timeout_seconds": 30, "stdout": (exc.stdout or "").strip(),
                    "stderr": (exc.stderr or "").strip(), "error": "timeout"}
    return {
        "classification": "actual-runtime",
        "command": command,
        "exit_code": result.returncode,
        "stdout": result.stdout.strip(),
        "stderr": result.stderr.strip(),
    }


def _capabilities(wheel: Path | None, public_import: dict[str, object] | None) -> dict[str, object]:
    if wheel is None:
        unavailable = {"classification": "not-tested", "status": "not-tested", "reason": "wheel path was not supplied"}
        return {name: dict(unavailable) for name in ("rule_reasoner", "context_authorized_subgraph_bridge", "graph_reasoner_provider_injection")}

    reasoner = _source(wheel, "semantica/reasoning/reasoner.py")
    graph_reasoner = _source(wheel, "semantica/reasoning/graph_reasoner.py")
    context_graph = _source(wheel, "semantica/context/context_graph.py")
    import_ok = public_import is not None and public_import["exit_code"] == 0
    import_reason = "public import succeeded" if import_ok else "public import failed; see public_import"
    return {
        "rule_reasoner": {
            "classification": "source-only",
            "status": "blocked-runtime-import" if not import_ok else "not-executed",
            "reason": import_reason,
            "source_findings": {
                "add_fact": "def add_fact" in reasoner,
                "add_rule": "def add_rule" in reasoner,
                "forward_chain": "def forward_chain" in reasoner,
                "premises_api": "premises" in reasoner,
                "requested_chain": "A DependsOn B + B DependsOn C => IndirectDependsOn(A,C) was not executed",
            },
        },
        "context_authorized_subgraph_bridge": {
            "classification": "source-only",
            "status": "not-an-acl-acceptance",
            "reason": "ContextGraph is an in-memory API candidate; persistent storage, Go authorization prefilter, and deletion/revocation checks were not exercised",
            "source_findings": {
                "context_graph_add_nodes": "def add_nodes" in context_graph,
                "context_graph_add_edges": "def add_edges" in context_graph,
                "fixture_parsed": True,
            },
        },
        "graph_reasoner_provider_injection": {
            "classification": "source-only",
            "status": "not-executed",
            "reason": import_reason,
            "source_findings": {
                "reason_signature": "def reason(self, graph: Dict[str, Any], query: str, **options) -> str:" in graph_reasoner,
                "returns_plain_string": "return response" in graph_reasoner,
                "constructor_provider_argument": "self.provider_name = kwargs.get(\"provider\")" in graph_reasoner,
                "factory_creation": "create_provider(self.provider_name" in graph_reasoner,
                "direct_provider_object_injection": False,
                "entity_properties_enter_prompt": "props = ent.get(\"properties\", {})" in graph_reasoner,
                "relationship_properties_enter_prompt": "props = rel.get(\"properties\", {})" in graph_reasoner,
                "controlled_provider_return": "not-tested: no mock is presented as a real provider",
            },
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixture", required=True, type=Path)
    parser.add_argument("--wheel", type=Path, default=os.environ.get("SEMANTICA_WHEEL"))
    parser.add_argument("--runtime-python", default=sys.executable)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    fixture = json.loads(args.fixture.read_text())
    if args.wheel is not None and not args.wheel.exists():
        parser.error(f"wheel does not exist: {args.wheel}")
    wheel = args.wheel
    digest = hashlib.sha256(wheel.read_bytes()).hexdigest() if wheel else None
    if wheel and digest != WHEEL_SHA256:
        parser.error(f"wheel SHA-256 mismatch: got {digest}, expected {WHEEL_SHA256}")
    public_import = _public_import(wheel, args.runtime_python) if wheel else None
    payload = {
        "schema_version": 1,
        "classification_legend": ["actual-runtime", "mock", "source-only", "not-tested"],
        "python": {"version": sys.version, "executable": sys.executable},
        "package": {
            "name": "semantica", "requested_version": "0.6.8", "wheel_sha256": digest,
            "expected_wheel_sha256": WHEEL_SHA256, "official_tag_object_sha": TAG_OBJECT_SHA,
            "official_source_commit_sha": SOURCE_COMMIT_SHA,
        },
        "fixture_summary": {"fact_count": len(fixture["facts"]), "authorized_node_count": len(fixture["authorized_subgraph"]["nodes"])},
        "public_import": public_import,
        "capabilities": _capabilities(wheel, public_import),
        "limits": ["No persistent Neo4j, Go ACL, deletion/revocation, performance, quality, budget, or real provider call was tested."],
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
