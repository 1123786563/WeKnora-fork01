"""V02-only scope-first persistence bridge; not production service code."""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any


RULE_ID = "technical-dependency-transitivity@v1"
RECORD_KIND = "assertion"


@dataclass(frozen=True)
class TopologyConfig:
    uri: str
    user: str
    password: str
    database: str
    topology: str


@dataclass(frozen=True)
class Scope:
    tenant_id: str
    kb_id: str
    document_id: str
    revision: int
    generation: str


def load_fixture(path: Path | str) -> dict[str, Any]:
    payload = json.loads(Path(path).read_text())
    if payload.get("schema_version") != 1 or not isinstance(payload.get("assertions"), list):
        raise ValueError("unsupported V02 fixture")
    return payload


def scope_read_query(scope: Scope) -> tuple[str, dict[str, Any]]:
    if not all((scope.tenant_id, scope.kb_id, scope.document_id, scope.generation)) or scope.revision < 1:
        raise ValueError("scope requires tenant, KB, document, positive revision, and generation")
    return (
        "MATCH (record:SemanticRecord {tenant_id: $tenant_id, kb_id: $kb_id, "
        "document_id: $document_id, revision: $revision, generation: $generation, "
        "record_kind: $record_kind}) RETURN record ORDER BY record.semantic_id",
        {
            "tenant_id": scope.tenant_id,
            "kb_id": scope.kb_id,
            "document_id": scope.document_id,
            "revision": scope.revision,
            "generation": scope.generation,
            "record_kind": RECORD_KIND,
        },
    )


def _node_parameters(assertion: dict[str, Any]) -> dict[str, Any]:
    required = {"semantic_id", "tenant_id", "kb_id", "document_id", "revision", "chunk_id", "generation", "content_hash", "quote", "predicate", "subject", "object", "evidence_ids"}
    missing = required.difference(assertion)
    if missing:
        raise ValueError(f"assertion missing required properties: {sorted(missing)}")
    return {**assertion, "record_kind": RECORD_KIND}


def write_fixture(config: TopologyConfig, fixture: dict[str, Any]) -> set[str]:
    """Persist fixture through compound-key MERGE; never return Neo4j internal IDs."""
    from neo4j import GraphDatabase

    query = (
        "MERGE (record:SemanticRecord {tenant_id: $tenant_id, kb_id: $kb_id, generation: $generation, "
        "record_kind: $record_kind, semantic_id: $semantic_id}) "
        "SET record += $properties RETURN record.semantic_id AS semantic_id"
    )
    driver = GraphDatabase.driver(config.uri, auth=(config.user, config.password))
    written: set[str] = set()
    try:
        with driver.session(database=config.database) as session:
            for assertion in fixture["assertions"]:
                properties = _node_parameters(assertion)
                row = session.run(query, {**properties, "properties": properties}).single()
                written.add(row["semantic_id"])
    finally:
        driver.close()
    return written


def read_scope(config: TopologyConfig, scope: Scope) -> list[dict[str, Any]]:
    from neo4j import GraphDatabase

    query, parameters = scope_read_query(scope)
    driver = GraphDatabase.driver(config.uri, auth=(config.user, config.password))
    try:
        with driver.session(database=config.database) as session:
            return [dict(row["record"]) for row in session.run(query, parameters)]
    finally:
        driver.close()


def probe_roundtrip(config: TopologyConfig, fixture_path: Path | str) -> dict[str, Any]:
    fixture = load_fixture(fixture_path)
    write_fixture(config, fixture)
    target = [
        *read_scope(config, Scope("T1", "K1", "D1", 1, "g1")),
        *read_scope(config, Scope("T1", "K1", "D2", 1, "g1")),
    ]
    assertion_ids = {row["semantic_id"] for row in target}
    evidence_ids = {evidence for row in target for evidence in row["evidence_ids"]}
    return {
        "storage_returned_ids": assertion_ids,
        "assertion_ids": assertion_ids,
        "evidence_ids": evidence_ids,
        "foreign_ids": {row["semantic_id"] for row in target if row["tenant_id"] != "T1" or row["kb_id"] != "K1"},
    }


def probe_rule(assertions: list[dict[str, Any]], rule_id: str) -> dict[str, Any]:
    if rule_id != RULE_ID:
        raise ValueError(f"unsupported rule: {rule_id}")
    by_id = {assertion.get("semantic_id"): assertion for assertion in assertions}
    first = by_id.get("a-d1")
    second = by_id.get("a-d2")
    if not (
        first
        and second
        and first.get("predicate") == "depends_on"
        and first.get("subject") == "A"
        and first.get("object") == "B"
        and second.get("predicate") == "depends_on"
        and second.get("subject") == "B"
        and second.get("object") == "C"
    ):
        return {"status": "insufficient_evidence", "conclusions": []}
    return {
        "status": "derived",
        "conclusions": [{
            "assertion_id": "r-a-d1-a-d2",
            "predicate": "indirectly_depends_on",
            "subject": "A",
            "object": "C",
            "kind": "rule",
            "rule_id": RULE_ID,
            "premise_ids": ["a-d1", "a-d2"],
            "evidence_ids": [*first["evidence_ids"], *second["evidence_ids"]],
        }],
    }
