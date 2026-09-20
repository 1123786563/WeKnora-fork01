"""Dedicated V02 writer/reader process runner; never calls a model/provider."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from hashlib import sha256
from importlib.metadata import version
from pathlib import Path
from typing import Any

from bridge_probe import Scope, TopologyConfig, load_fixture, probe_rule, read_scope, write_fixture


ROOT = Path(__file__).resolve().parents[2]
LOCK_PATH = ROOT / "semantic" / "experiments" / "uv.lock"
V01_LOCK_SHA256 = "c643ce123490c93f56ed95e9d6501bbd90daf8b2cdc74b183067dccd63864c54"


def from_environment() -> TopologyConfig:
    password = os.environ.get("SEMANTICA_V02_NEO4J_PASSWORD")
    if not password:
        raise RuntimeError("SEMANTICA_V02_NEO4J_PASSWORD is required for V02 dedicated topology")
    return TopologyConfig(
        uri=os.environ.get("SEMANTICA_V02_NEO4J_URI", "bolt://127.0.0.1:17687"),
        user=os.environ.get("SEMANTICA_V02_NEO4J_USER", "neo4j"),
        password=password,
        database=os.environ.get("SEMANTICA_V02_NEO4J_DATABASE", "neo4j"),
        topology="dedicated",
    )


def _reader_result(config: TopologyConfig, fixture_path: Path) -> dict[str, Any]:
    target = [
        *read_scope(config, Scope("T1", "K1", "D1", 1, "g1")),
        *read_scope(config, Scope("T1", "K1", "D2", 1, "g1")),
    ]
    assertion_ids = {row["semantic_id"] for row in target}
    evidence_ids = {evidence for row in target for evidence in row["evidence_ids"]}
    return {
        "reader_pid": os.getpid(),
        "assertion_ids": sorted(assertion_ids),
        "evidence_ids": sorted(evidence_ids),
        "storage_returned_ids": sorted(assertion_ids),
        "foreign_ids": sorted(row["semantic_id"] for row in target if row["tenant_id"] != "T1" or row["kb_id"] != "K1"),
        "provenance": target,
        "rule": probe_rule(sorted(assertion_ids), "technical-dependency-transitivity@v1"),
        "fixture": str(fixture_path),
    }


def _run_child(mode: str, fixture_path: Path) -> dict[str, Any]:
    process = subprocess.run(
        [sys.executable, str(Path(__file__)), mode, "--fixture", str(fixture_path)],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(process.stdout)


def run_writer_then_reader(config: TopologyConfig, fixture_path: Path | str) -> dict[str, Any]:
    if config.topology != "dedicated":
        raise ValueError("V02 local runner accepts dedicated topology only")
    fixture = Path(fixture_path).resolve()
    writer = _run_child("writer", fixture)
    reader = _run_child("reader", fixture)
    for key in ("assertion_ids", "evidence_ids", "storage_returned_ids", "foreign_ids"):
        reader[key] = set(reader[key])
    return {
        **reader,
        "writer_pid": writer["writer_pid"],
        "restart_verified": writer["writer_pid"] != reader["reader_pid"],
        "backend": "neo4j",
        "topology": config.topology,
    }


def evidence_record(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: evidence_record(item) for key, item in value.items()}
    if isinstance(value, set):
        return sorted(evidence_record(item) for item in value)
    if isinstance(value, list):
        return [evidence_record(item) for item in value]
    return value


def evidence_metadata(config: TopologyConfig) -> dict[str, Any]:
    return {
        "topology": config.topology,
        "backend": "neo4j",
        "endpoint_alias": "loopback:17687" if config.uri == "bolt://127.0.0.1:17687" else "operator-supplied",
        "database": config.database,
        "account_identity": config.user,
        "volume_identity": "semantica-v02_semantica-v02-neo4j-data" if config.topology == "dedicated" else "operator-supplied",
        "semantica_version": version("semantica"),
        "v01_lock_sha256": V01_LOCK_SHA256,
        "v02_lock_sha256": sha256(LOCK_PATH.read_bytes()).hexdigest(),
    }


def capture_dedicated_run(config: TopologyConfig, fixture_path: Path, output: Path, phase: str) -> dict[str, Any]:
    existing = json.loads(output.read_text()) if output.exists() else {
        "schema_version": 1,
        "topology": "dedicated",
        "backend": "neo4j",
        "evidence_layer": "real-storage",
        "runs": {},
        "limitations": [
            "Dedicated local topology only; shared-isolated comparison remains required for V02 verification.",
            "No model/provider, production ACL, or Go service integration was invoked.",
        ],
    }
    existing["metadata"] = evidence_metadata(config)
    existing["runs"][phase] = evidence_record(run_writer_then_reader(config, fixture_path))
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(existing, ensure_ascii=False, indent=2) + "\n")
    return existing


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("writer", "reader", "capture"))
    parser.add_argument("--fixture", required=True, type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--phase")
    args = parser.parse_args()
    config = from_environment()
    if args.mode == "writer":
        written = write_fixture(config, load_fixture(args.fixture))
        print(json.dumps({"writer_pid": os.getpid(), "written_ids": sorted(written)}))
    elif args.mode == "reader":
        print(json.dumps(_reader_result(config, args.fixture), ensure_ascii=False))
    else:
        if args.output is None or not args.phase:
            parser.error("capture requires --output and --phase")
        print(json.dumps(capture_dedicated_run(config, args.fixture, args.output, args.phase), ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
