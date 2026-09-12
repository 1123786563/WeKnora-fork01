"""Integration fixtures for recovery drills (O02).

RecoveryHarness drives REAL services over the isolated PG store: no
in-memory substitutes for recovery state. Fault points are TEST-ONLY
(pause phases, snapshot/restore) - never reachable through production
user APIs.
"""

from __future__ import annotations

import json
import uuid

import psycopg
import pytest

from semantic_service.contracts import ScopeKey
from semantic_service.indexing.deletion import DeletionService
from semantic_service.indexing.manifest import ArtifactRef, DocumentEntry, IndexManifest
from semantic_service.pg_access import PgAccessGraph
from semantic_service.query.search import SearchService
from semantic_service.query.subgraph import QueryLimits, build_authorized_subgraph
from semantic_service.query.adapter import FrozenGraphRAGAdapter

SCOPE = ScopeKey(tenant_id=1, kb_id="kb-o02")


class RecoveryHarness:
    """Drives the real service stack for fault-injection drills."""

    def __init__(self, dsn: str, deletion: DeletionService, store):
        self._dsn = dsn
        self._deletion = deletion
        self._store = store

    # --- snapshot / restore (test-only fault points) ---

    def snapshot(self) -> dict:
        """Snapshot the SERVICE-side state: assertion support rows (the
        visible fact base). Tombstones (deny authority) are NOT
        snapshotted - a restore must never resurrect deleted data."""
        with psycopg.connect(self._dsn) as conn:
            assertions = conn.execute(
                "SELECT assertion_id, subject_id, predicate, support_document_id, support_revision, visible, kind"
                " FROM semantic.assertions WHERE tenant_id = %s AND kb_id = %s",
                (SCOPE.tenant_id, SCOPE.kb_id)).fetchall()
        return {"assertions": [list(r) for r in assertions]}

    def restore_snapshot(self, snapshot: dict) -> None:
        """Restore assertions AND enter maintenance (readiness closes
        until denials replay). Tombstones survive by design."""
        with psycopg.connect(self._dsn) as conn:
            conn.execute("DELETE FROM semantic.assertions WHERE tenant_id = %s AND kb_id = %s",
                         (SCOPE.tenant_id, SCOPE.kb_id))
            for row in snapshot["assertions"]:
                conn.execute(
                    "INSERT INTO semantic.assertions (assertion_id, subject_id, predicate,"
                    " support_document_id, support_revision, visible, kind, tenant_id, kb_id)"
                    " VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)",
                    (row[0], row[1], row[2], row[3], row[4], row[5], row[6], SCOPE.tenant_id, SCOPE.kb_id))
            conn.execute("INSERT INTO semantic.recovery_state (key, value) VALUES ('denials_replayed', 'false')"
                         " ON CONFLICT (key) DO UPDATE SET value = 'false'")
            conn.commit()

    # --- readiness / deny replay (O02 core) ---

    def ready(self) -> bool:
        """Restore-mode readiness: deny state must be REPLAYED from the
        business authority before queries open."""
        with psycopg.connect(self._dsn) as conn:
            row = conn.execute("SELECT value FROM semantic.recovery_state WHERE key = 'denials_replayed'").fetchone()
        return bool(row and row[0] == "true")

    def replay_current_denials(self) -> None:
        """Replay the CURRENT deny set from the business authority (the
        tombstones ARE the authoritative record after Go replay writes
        them) and mark readiness open."""
        with psycopg.connect(self._dsn) as conn:
            conn.execute(
                "INSERT INTO semantic.recovery_state (key, value) VALUES ('denials_replayed', 'true')"
                " ON CONFLICT (key) DO UPDATE SET value = 'true'")
            conn.commit()

    def set_maintenance(self, maintenance: bool) -> None:
        with psycopg.connect(self._dsn) as conn:
            conn.execute("INSERT INTO semantic.recovery_state (key, value) VALUES ('denials_replayed', %s)"
                         " ON CONFLICT (key) DO UPDATE SET value = %s",
                         ("false" if maintenance else "true", "false" if maintenance else "true"))
            conn.commit()

    # --- business actions ---

    def delete_document(self, document_id: str, revision: int) -> None:
        self._deletion.delete_revision(SCOPE, document_id, revision)

    def add_document(self, document_id: str, revision: int = 1, text_entity: str = "甲公司") -> None:
        self._deletion.add_assertion_support(SCOPE, f"fact-{document_id}", text_entity, "mentions",
            value=document_id, support_document_id=document_id, support_revision=revision)

    def query(self, query_text: str) -> "QueryOutcome":
        graph = PgAccessGraph(self._dsn, SCOPE, self._deletion)
        allowed = self._visible_documents()
        # Seed by alias matches AND assertions whose subject entity name
        # matches the query (the Q01 seeding semantics).
        seeds = list(graph.search_seeds(query_text, allowed))
        for assertion_id in graph.assertions_on(query_text):
            if assertion_id not in seeds and graph.visible(assertion_id, allowed):
                seeds.append(assertion_id)
        result = build_authorized_subgraph(graph, SCOPE, None, seeds, allowed, QueryLimits())
        return QueryOutcome(assertions=result.assertions, document_ids=self._documents_of(result, graph, allowed))

    def _visible_documents(self) -> set:
        """Documents with a visible assertion row surviving tombstones."""
        docs = set()
        with psycopg.connect(self._dsn) as conn:
            rows = conn.execute(
                "SELECT DISTINCT support_document_id, support_revision FROM semantic.assertions"
                " WHERE tenant_id = %s AND kb_id = %s AND visible = TRUE",
                (SCOPE.tenant_id, SCOPE.kb_id)).fetchall()
        for doc, rev in rows:
            if not self._deletion.denied(SCOPE, doc, rev):
                docs.add(doc)
        return docs

    def _documents_of(self, graph_result, graph, allowed) -> set:
        out = set()
        for assertion_id in graph_result.assertions:
            for row in graph._support_rows(assertion_id):
                if row["support_document_id"] in allowed:
                    out.add(row["support_document_id"])
        return out


class QueryOutcome:
    def __init__(self, assertions: list, document_ids: set):
        self.assertions = assertions
        self.document_ids = document_ids


@pytest.fixture()
def recovery(pg_dsn, deletion_service, generation_index_store):
    """Recovery harness over the isolated PG with a published generation."""
    import psycopg

    with psycopg.connect(pg_dsn) as conn:
        conn.execute("""
            CREATE TABLE IF NOT EXISTS semantic.recovery_state (
                key VARCHAR(64) PRIMARY KEY,
                value TEXT NOT NULL
            )""")
        conn.execute("DELETE FROM semantic.recovery_state")
        conn.commit()
    harness = RecoveryHarness(pg_dsn, deletion_service, generation_index_store)
    harness.add_document("d1", revision=2, text_entity="甲公司")
    harness.add_document("d2", revision=2, text_entity="甲公司")
    harness.replay_current_denials()
    return harness
