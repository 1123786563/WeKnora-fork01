"""Narrow, non-production V02 bridge between a fixture and isolated services."""

from __future__ import annotations

import json
import os
import subprocess
import time
import urllib.error
import urllib.request
from importlib.metadata import version
from pathlib import Path
from typing import Any

from semantica.graph_store import GraphStore
from semantica.reasoning import GraphReasoner, Reasoner
from semantica.semantic_extract.providers import BaseProvider
from semantica.semantic_extract.registry import provider_registry


NAMESPACE = "semantica-v02"
DEFAULT_URI = "bolt://127.0.0.1:17687"
DEFAULT_CONTAINER = "semantica-v02-bridge"
DEFAULT_PASSWORD = "semantica-v02-experiment-only"
DEFAULT_GATEWAY = "http://127.0.0.1:18092"
RULE_PREFIX = "IF "


def _neo4j_config() -> dict[str, str]:
    return {
        "uri": os.environ.get("V02_NEO4J_URI", DEFAULT_URI),
        "user": "neo4j",
        "password": os.environ.get("V02_NEO4J_PASSWORD", DEFAULT_PASSWORD),
        "database": "neo4j",
    }


def _docker_restart_owned_container() -> None:
    """Restart only the explicitly named V02 container after closing the driver."""
    container = os.environ.get("V02_NEO4J_CONTAINER", DEFAULT_CONTAINER)
    if not container.startswith("semantica-v02-"):
        raise ValueError("V02_NEO4J_CONTAINER must use the semantica-v02- prefix")
    result = subprocess.run(
        ["docker", "restart", container], check=False, capture_output=True, text=True, timeout=60
    )
    if result.returncode != 0:
        raise RuntimeError(f"owned Neo4j restart failed: {result.stderr.strip()}")


def _wait_for_neo4j(store: GraphStore) -> None:
    deadline = time.monotonic() + 45
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            if store.connect():
                return
        except Exception as exc:  # Neo4j is expected to be down briefly after the owned restart.
            last_error = exc
            store.close()
        time.sleep(1)
    raise RuntimeError(f"isolated Neo4j did not accept a connection: {last_error}")


def _relationship_properties(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "namespace": NAMESPACE,
        "assertion_id": row["assertion_id"],
        "document_id": row["document_id"],
        "revision": row["revision"],
        "evidence_ids": row["evidence_ids"],
        "quote": row["quote"],
    }


def _persist_fixture(store: GraphStore, fixture: dict[str, Any]) -> None:
    store.execute_query("MATCH (n:SemanticaV02 {namespace: $namespace}) DETACH DELETE n", {"namespace": NAMESPACE})
    node_ids: dict[str, int] = {}
    for row in fixture["relationships"]:
        for name in (row["source"], row["target"]):
            if name not in node_ids:
                created = store.create_node(
                    labels=["SemanticaV02"], properties={"namespace": NAMESPACE, "name": name}
                )
                node_ids[name] = created["id"]
        store.create_relationship(
            node_ids[row["source"]], node_ids[row["target"]], "DEPENDS_ON", _relationship_properties(row)
        )


def _read_fixture_from_neo4j(store: GraphStore) -> list[dict[str, Any]]:
    response = store.execute_query(
        """
        MATCH (source:SemanticaV02 {namespace: $namespace})-[rel:DEPENDS_ON {namespace: $namespace}]->
              (target:SemanticaV02 {namespace: $namespace})
        RETURN source.name AS source, target.name AS target, rel.assertion_id AS assertion_id,
               rel.document_id AS document_id, rel.revision AS revision, rel.evidence_ids AS evidence_ids,
               rel.quote AS quote
        ORDER BY rel.assertion_id
        """,
        {"namespace": NAMESPACE},
    )
    records = response["records"] if isinstance(response, dict) else response
    return [
        {"source": record["source"], "target": record["target"], "type": "depends_on", **{
            key: record[key] for key in ("assertion_id", "document_id", "revision", "evidence_ids", "quote")
        }}
        for record in records
    ]


def build_probe_graph(rows: list[dict[str, Any]]) -> dict[str, Any]:
    """Project pre-authorized persisted relationships into a provenance-preserving graph."""
    for row in rows:
        if "evidence_ids" not in row:
            raise ValueError("persistent relationship is missing evidence_ids")
    names = sorted({row["source"] for row in rows} | {row["target"] for row in rows})
    return {
        "entities": [{"id": name, "type": "步骤", "properties": {}} for name in names],
        "relationships": [
            {
                "source": row["source"],
                "target": row["target"],
                "type": row["type"],
                "properties": {
                    key: row[key]
                    for key in ("assertion_id", "document_id", "revision", "evidence_ids", "quote")
                },
            }
            for row in rows
        ],
    }


def probe_roundtrip(fixture_path: str | Path) -> dict[str, Any]:
    """Persist, close, restart the owned service, reconnect, then authorize-project."""
    fixture = json.loads(Path(fixture_path).read_text())
    config = _neo4j_config()
    writer = GraphStore(backend="neo4j", **config)
    try:
        _wait_for_neo4j(writer)
        _persist_fixture(writer, fixture)
    finally:
        writer.close()

    _docker_restart_owned_container()

    reader = GraphStore(backend="neo4j", **config)
    try:
        _wait_for_neo4j(reader)
        persistent_rows = _read_fixture_from_neo4j(reader)
    finally:
        reader.close()

    allowed = set(fixture["allowed_document_ids"])
    allowed_rows = [row for row in persistent_rows if row["document_id"] in allowed]
    subgraph = build_probe_graph(allowed_rows)
    evidence_ids = sorted({evidence for row in allowed_rows for evidence in row["evidence_ids"]})
    return {
        "restart_verified": True,
        "client_reopened": True,
        "storage_rows": len(persistent_rows),
        "evidence_ids": evidence_ids,
        "result": subgraph,
        "engine_version": version("neo4j"),
        "actual_backend": "neo4j-dedicated-instance",
    }


def _normalize_rule(rule: str) -> str:
    if "&" not in rule or "->" not in rule:
        raise ValueError("only the V02 two-premise depends_on rule grammar is supported")
    premises, conclusion = rule.split("->", 1)
    first, second = premises.split("&", 1)
    def with_variables(value: str) -> str:
        return value.replace("(x,y)", "(?x,?y)").replace("(y,z)", "(?y,?z)").replace("(x,z)", "(?x,?z)")
    return f"{RULE_PREFIX}{with_variables(first)} AND {with_variables(second)} THEN {with_variables(conclusion)}"


def probe_rule(facts: list[str], rules: list[str]) -> dict[str, Any]:
    """Run only supplied registered V02 rules through Semantica's real Reasoner."""
    reasoner = Reasoner()
    for fact in facts:
        reasoner.add_fact(fact)
    for rule in rules:
        reasoner.add_rule(_normalize_rule(rule))
    inferred = reasoner.forward_chain()
    conclusions = sorted(item.conclusion for item in inferred)
    conflicts = sorted(fact for fact in facts if fact.startswith("not_depends_on("))
    return {
        "evidence_ids": [],
        "result": {"inferences": [{"conclusion": item.conclusion, "premises": item.premises} for item in inferred]},
        "conclusions": conclusions,
        "conflicts": conflicts,
        "status": "conflicting_evidence" if conflicts else "supported",
        "engine_version": version("semantica"),
        "actual_backend": "semantica-reasoner-0.6.8",
    }


class _LoopbackGoProvider(BaseProvider):
    """Registered provider which can call only the bounded experiment gateway."""

    def __init__(self, **kwargs: Any) -> None:
        super().__init__(**kwargs)
        self.gateway_url = os.environ.get("V02_MODEL_GATEWAY_URL", DEFAULT_GATEWAY).rstrip("/")
        if not self.gateway_url.startswith("http://127.0.0.1:"):
            raise ValueError("V02 model gateway must bind to 127.0.0.1")
        self.last_usage: dict[str, int] = {}

    def generate(self, prompt: str, **kwargs: Any) -> str:
        if len(prompt) > 8_000:
            raise ValueError("V02 prompt exceeds fixed cap")
        request = urllib.request.Request(
            f"{self.gateway_url}/v1/semantica-v02/generate",
            data=json.dumps({"prompt": prompt}, ensure_ascii=False).encode(),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=25) as response:
                payload = json.loads(response.read())
        except urllib.error.URLError as exc:
            raise RuntimeError(f"V02 loopback Go gateway unavailable: {exc}") from exc
        self.last_usage = payload["raw_usage"]
        if self.last_usage["total_tokens"] != self.last_usage["prompt_tokens"] + self.last_usage["completion_tokens"]:
            raise RuntimeError("gateway returned inconsistent raw token counts")
        return payload["text"]


def probe_model(graph: dict[str, Any], query: str) -> dict[str, Any]:
    """Use registered GraphReasoner with the only permitted V02 model transport."""
    provider_name = "semantica-v02-go-loopback"
    provider_registry.register(provider_name, _LoopbackGoProvider)
    reasoner = GraphReasoner(provider=provider_name, model="qwen2.5:0.5b")
    answer = reasoner.reason(graph, query, max_tokens=96, temperature=0)
    provider = reasoner.provider
    evidence_ids = sorted({
        evidence
        for relation in graph.get("relationships", [])
        for evidence in relation.get("properties", {}).get("evidence_ids", [])
    })
    return {
        "availability": "live-local-provider",
        "evidence_ids": evidence_ids,
        "result": answer,
        "raw_usage": provider.last_usage,
        "engine_version": version("semantica"),
        "actual_backend": "semantica-graphreasoner-via-go-loopback",
    }
