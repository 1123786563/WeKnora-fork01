"""Fixture-backed AccessGraph over the real PG fact store (A02)."""

from __future__ import annotations

import json

import psycopg
from psycopg.rows import dict_row

from .contracts import ScopeKey
from .access import AccessGraph


class PgAccessGraph(AccessGraph):
    def __init__(self, dsn: str, scope: ScopeKey, deletion):
        super().__init__(deletion)
        self._dsn = dsn
        self._scope = scope

    def _support_rows(self, assertion_id: str) -> list[dict]:
        with psycopg.connect(self._dsn, row_factory=dict_row) as conn:
            rows = conn.execute(
                "SELECT support_document_id, support_revision, visible, premise_ids"
                " FROM semantic.assertions WHERE tenant_id = %s AND kb_id = %s AND assertion_id = %s",
                (self._scope.tenant_id, self._scope.kb_id, assertion_id),
            ).fetchall()
        out = []
        for row in rows:
            out.append({
                "support_document_id": row["support_document_id"],
                "support_revision": row["support_revision"],
                "visible": row["visible"],
                "_premises": json.loads(row["premise_ids"] or "[]"),
                "_scope": self._scope,
            })
        return out

    def neighborhood(self, assertion_id: str, allowed_documents: set[str]) -> list[str]:
        """Subject/object nodes of the assertion's VISIBLE support rows."""
        with psycopg.connect(self._dsn, row_factory=dict_row) as conn:
            rows = conn.execute(
                "SELECT subject_id, object_id, visible, support_document_id FROM semantic.assertions"
                " WHERE tenant_id = %s AND kb_id = %s AND assertion_id = %s",
                (self._scope.tenant_id, self._scope.kb_id, assertion_id),
            ).fetchall()
        nodes: list[str] = []
        for row in rows:
            if row["support_document_id"] not in allowed_documents or not row["visible"]:
                continue
            nodes.append(row["subject_id"])
            if row["object_id"]:
                nodes.append(row["object_id"])
        return nodes

    def assertions_on(self, node: str) -> list[str]:
        """Assertion ids touching a node (visibility is re-checked by the
        walker - this is only the structural adjacency list)."""
        with psycopg.connect(self._dsn, row_factory=dict_row) as conn:
            rows = conn.execute(
                "SELECT assertion_id FROM semantic.assertions"
                " WHERE tenant_id = %s AND kb_id = %s AND (subject_id = %s OR object_id = %s)",
                (self._scope.tenant_id, self._scope.kb_id, node, node),
            ).fetchall()
        return [row["assertion_id"] for row in rows]

    def _alias_index(self) -> dict[str, list[str]]:
        with psycopg.connect(self._dsn, row_factory=dict_row) as conn:
            rows = conn.execute(
                "SELECT assertion_id, value FROM semantic.assertions"
                " WHERE tenant_id = %s AND kb_id = %s AND predicate = 'alias'",
                (self._scope.tenant_id, self._scope.kb_id),
            ).fetchall()
        index: dict[str, list[str]] = {}
        for row in rows:
            index.setdefault(row["value"] or "", []).append(row["assertion_id"])
        return index
