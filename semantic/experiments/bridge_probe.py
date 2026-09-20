"""Narrow, non-production V02 bridge between a fixture and isolated services."""

from __future__ import annotations

import json
import subprocess
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from hashlib import sha256
from importlib.metadata import version
from pathlib import Path
from typing import Any
from urllib.parse import urlparse
from uuid import uuid4

from semantica.graph_store import GraphStore
from semantica.reasoning import GraphReasoner, Reasoner
from semantica.semantic_extract.providers import BaseProvider
from semantica.semantic_extract.registry import provider_registry


DEFAULT_PASSWORD = "semantica-v02-experiment-only"
DEFAULT_GATEWAY = "http://127.0.0.1:18092"
RULE_PREFIX = "IF "
NEO4J_IMAGE = "neo4j@sha256:155c8aad10d5c838bc3bbc476c0418779086547822acb214ec5e3d49ba336907"
NEO4J_HOST_PORT = 17687


@dataclass(frozen=True)
class _OwnedNeo4j:
    """Run-unique, disposable storage identity; every mutation rechecks it."""

    nonce: str
    container_id: str
    volume: str
    host_port: int

    @property
    def namespace(self) -> str:
        return f"semantica-v02-{self.nonce}"

    @property
    def config(self) -> dict[str, str]:
        return {
            "uri": f"bolt://127.0.0.1:{self.host_port}",
            "user": "neo4j",
            "password": DEFAULT_PASSWORD,
            "database": "neo4j",
        }

    @classmethod
    def create(cls) -> "_OwnedNeo4j":
        nonce = uuid4().hex[:12]
        name = f"semantica-v02-{nonce}"
        volume = f"{name}-data"
        container_id: str | None = None
        try:
            subprocess.run(
                ["docker", "volume", "create", "--label", "semantica.experiment=v02", "--label", f"semantica.run_nonce={nonce}", volume],
                check=True, capture_output=True, text=True, timeout=30,
            )
            created = subprocess.run(
                [
                    "docker", "run", "-d", "--name", name,
                    "--label", "semantica.experiment=v02",
                    "--label", f"semantica.run_nonce={nonce}",
                    "-p", f"127.0.0.1:{NEO4J_HOST_PORT}:7687",
                    "--mount", f"type=volume,source={volume},target=/data",
                    "-e", f"NEO4J_AUTH=neo4j/{DEFAULT_PASSWORD}",
                    NEO4J_IMAGE,
                ], check=True, capture_output=True, text=True, timeout=30,
            )
            container_id = created.stdout.strip()
            metadata = _verify_owned_mutation(container_id, nonce)
            return cls(nonce, container_id, volume, _host_port(metadata))
        except BaseException as error:
            issues = _cleanup_owned_resources(container_id, volume, nonce)
            if issues:
                raise RuntimeError(f"V02 provisioning failed; cleanup_issues={json.dumps(issues, sort_keys=True)}") from error
            raise

    def verify(self) -> None:
        metadata = _verify_owned_mutation(self.container_id, self.nonce)
        if _host_port(metadata) != self.host_port:
            raise RuntimeError("owned Neo4j port mapping changed")

    def restart(self) -> None:
        self.verify()
        result = subprocess.run(["docker", "restart", self.container_id], check=False, capture_output=True, text=True, timeout=60)
        if result.returncode != 0:
            raise RuntimeError(f"owned Neo4j restart failed: {result.stderr.strip()}")
        self.verify()

    def cleanup(self) -> list[dict[str, str]]:
        return _cleanup_owned_resources(self.container_id, self.volume, self.nonce)


def _inspect_owned_identity(container_id: str, nonce: str) -> dict[str, Any]:
    inspected = subprocess.run(["docker", "inspect", container_id], check=True, capture_output=True, text=True, timeout=30)
    metadata = json.loads(inspected.stdout)[0]
    labels = metadata["Config"].get("Labels") or {}
    if (
        metadata.get("Id") != container_id
        or metadata["Config"].get("Image") != NEO4J_IMAGE
        or labels.get("semantica.experiment") != "v02"
        or labels.get("semantica.run_nonce") != nonce
    ):
        raise RuntimeError("refusing V02 operation: container identity check failed")
    return metadata


def _verify_owned_mutation(container_id: str, nonce: str) -> dict[str, Any]:
    metadata = _inspect_owned_identity(container_id, nonce)
    ports = metadata.get("NetworkSettings", {}).get("Ports") or {}
    mapping = ports.get("7687/tcp")
    if (
        metadata.get("State", {}).get("Running") is not True
        or not isinstance(mapping, list)
        or len(mapping) != 1
        or mapping[0].get("HostIp") != "127.0.0.1"
        or mapping[0].get("HostPort") != str(NEO4J_HOST_PORT)
    ):
        raise RuntimeError("refusing V02 storage mutation: container ownership/isolation check failed")
    return metadata


def _inspect_owned_volume(volume: str, nonce: str) -> dict[str, Any]:
    inspected = subprocess.run(["docker", "volume", "inspect", volume], check=True, capture_output=True, text=True, timeout=30)
    metadata = json.loads(inspected.stdout)[0]
    labels = metadata.get("Labels") or {}
    if (
        metadata.get("Name") != volume
        or labels.get("semantica.experiment") != "v02"
        or labels.get("semantica.run_nonce") != nonce
    ):
        raise RuntimeError("refusing V02 cleanup: volume identity check failed")
    return metadata


def _cleanup_owned_resources(container_id: str | None, volume: str, nonce: str) -> list[dict[str, str]]:
    """Best-effort cleanup uses immutable ownership, never runtime readiness."""
    issues: list[dict[str, str]] = []
    if container_id:
        try:
            _inspect_owned_identity(container_id, nonce)
            subprocess.run(["docker", "rm", "-f", container_id], check=True, capture_output=True, text=True, timeout=30)
        except Exception as exc:
            issues.append({"resource": "container", "error": str(exc)})
    try:
        _inspect_owned_volume(volume, nonce)
        subprocess.run(["docker", "volume", "rm", volume], check=True, capture_output=True, text=True, timeout=30)
    except Exception as exc:
        issues.append({"resource": "volume", "error": str(exc)})
    return issues


def _host_port(metadata: dict[str, Any]) -> int:
    return int(metadata["NetworkSettings"]["Ports"]["7687/tcp"][0]["HostPort"])


def _wait_for_neo4j(store: GraphStore) -> None:
    deadline = time.monotonic() + 75
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


def _relationship_properties(row: dict[str, Any], namespace: str) -> dict[str, Any]:
    return {
        "namespace": namespace,
        "assertion_id": row["assertion_id"],
        "document_id": row["document_id"],
        "revision": row["revision"],
        "evidence_ids": row["evidence_ids"],
        "quote": row["quote"],
    }


def _persist_fixture(store: GraphStore, fixture: dict[str, Any], owned: _OwnedNeo4j) -> None:
    owned.verify()
    store.execute_query("MATCH (n:SemanticaV02 {namespace: $namespace}) DETACH DELETE n", {"namespace": owned.namespace})
    node_ids: dict[str, int] = {}
    for row in fixture["relationships"]:
        for name in (row["source"], row["target"]):
            if name not in node_ids:
                created = store.create_node(
                    labels=["SemanticaV02"], properties={"namespace": owned.namespace, "name": name}
                )
                node_ids[name] = created["id"]
        store.create_relationship(
            node_ids[row["source"]], node_ids[row["target"]], "DEPENDS_ON", _relationship_properties(row, owned.namespace)
        )


def _read_fixture_from_neo4j(store: GraphStore, namespace: str) -> list[dict[str, Any]]:
    response = store.execute_query(
        """
        MATCH (source:SemanticaV02 {namespace: $namespace})-[rel:DEPENDS_ON {namespace: $namespace}]->
              (target:SemanticaV02 {namespace: $namespace})
        RETURN source.name AS source, target.name AS target, rel.assertion_id AS assertion_id,
               rel.document_id AS document_id, rel.revision AS revision, rel.evidence_ids AS evidence_ids,
               rel.quote AS quote
        ORDER BY rel.assertion_id
        """,
        {"namespace": namespace},
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
    owned = _OwnedNeo4j.create()
    primary_error: BaseException | None = None
    try:
        writer = GraphStore(backend="neo4j", **owned.config)
        try:
            _wait_for_neo4j(writer)
            _persist_fixture(writer, fixture, owned)
        finally:
            writer.close()

        owned.restart()

        reader = GraphStore(backend="neo4j", **owned.config)
        try:
            _wait_for_neo4j(reader)
            owned.verify()
            persistent_rows = _read_fixture_from_neo4j(reader, owned.namespace)
        finally:
            reader.close()
    except BaseException as error:
        primary_error = error

    cleanup_issues = owned.cleanup()
    if primary_error is not None:
        if cleanup_issues:
            raise RuntimeError(f"V02 roundtrip failed; cleanup_issues={json.dumps(cleanup_issues, sort_keys=True)}") from primary_error
        raise primary_error
    if cleanup_issues:
        raise RuntimeError(f"V02 roundtrip cleanup failed; cleanup_issues={json.dumps(cleanup_issues, sort_keys=True)}")

    allowed = set(fixture["allowed_document_ids"])
    allowed_rows = [row for row in persistent_rows if row["document_id"] in allowed]
    subgraph = build_probe_graph(allowed_rows)
    evidence_ids = sorted({evidence for row in allowed_rows for evidence in row["evidence_ids"]})
    return {
        "restart_verified": True,
        "client_reopened": True,
        "storage_rows": len(persistent_rows),
        "run_nonce": owned.nonce,
        "resource_cleanup_verified": True,
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
    has_negative_fact = any(fact.startswith("not_depends_on(") for fact in facts)
    conflict_capability = {
        "classification": "actual-runtime",
        "status": "unavailable",
        "reason": (
            "Semantica 0.6.8 Reasoner.forward_chain exposes inferred facts only; "
            "the frozen public Reasoner API has no registered contradiction/conflict operation."
        ),
    }
    return {
        "evidence_ids": [],
        "result": {"inferences": [{"conclusion": item.conclusion, "premises": item.premises} for item in inferred]},
        "conclusions": conclusions,
        "conflict_capability": conflict_capability if has_negative_fact else None,
        "status": "unavailable" if has_negative_fact else "supported",
        "engine_version": version("semantica"),
        "actual_backend": "semantica-reasoner-0.6.8",
    }


def validate_gateway_url(value: str) -> str:
    """Accept exactly the fixed loopback base, never a redirectable lookalike."""
    parsed = urlparse(value)
    try:
        port = parsed.port
    except ValueError as exc:
        raise ValueError("V02 model gateway has an invalid port") from exc
    if (
        parsed.scheme != "http"
        or parsed.hostname != "127.0.0.1"
        or port != 18092
        or parsed.username is not None
        or parsed.password is not None
        or parsed.path not in ("", "/")
        or parsed.params
        or parsed.query
        or parsed.fragment
    ):
        raise ValueError("V02 model gateway must be exactly http://127.0.0.1:18092")
    return DEFAULT_GATEWAY


class _LoopbackGoProvider(BaseProvider):
    """Registered provider which can call only the bounded experiment gateway."""

    def __init__(self, **kwargs: Any) -> None:
        super().__init__(**kwargs)
        self.gateway_url = validate_gateway_url(DEFAULT_GATEWAY)
        self.last_usage: dict[str, int] = {}
        self.last_prompt = ""

    def generate(self, prompt: str, **kwargs: Any) -> str:
        if len(prompt) > 8_000:
            raise ValueError("V02 prompt exceeds fixed cap")
        self.last_prompt = prompt
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
        if payload.get("completed") is not True or payload.get("truncated") is not False:
            raise RuntimeError("gateway did not return a complete bounded result")
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
    properties = [relation.get("properties", {}) for relation in graph.get("relationships", [])]
    def present_values(key: str) -> list[str]:
        values = {
            value
            for properties_item in properties
            for value in (properties_item.get(key) if isinstance(properties_item.get(key), list) else [properties_item.get(key)])
            if value is not None and str(value) in provider.last_prompt
        }
        return sorted(values)
    return {
        "availability": "live-local-provider",
        "evidence_ids": evidence_ids,
        "result": answer,
        "raw_usage": provider.last_usage,
        "prompt_sha256": sha256(provider.last_prompt.encode()).hexdigest(),
        "prompt_provenance": {
            "allowed_assertion_ids": present_values("assertion_id"),
            "allowed_revisions": present_values("revision"),
            "allowed_evidence_ids": present_values("evidence_ids"),
            "allowed_quotes": present_values("quote"),
            "hidden_absent": all(hidden not in provider.last_prompt for hidden in ("隐藏组件", "e-hidden", "a-hidden-deploy")),
        },
        "engine_version": version("semantica"),
        "actual_backend": "semantica-graphreasoner-via-go-loopback",
    }
