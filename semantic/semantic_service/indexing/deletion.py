"""Deletion barriers, support revocation and cleanup receipts (I04).

Deleting a document revision writes a MONOTONE tombstone overlaid on
every generation query (deny-first, even before physical cleanup).
Revoking one source of a multi-source fact keeps the fact while any
visible support remains; the last support's removal recursively
invalidates derived assertions through the reverse-premise fixpoint.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime
from typing import Optional

import psycopg
from psycopg.rows import dict_row

from ..contracts import DocumentRevision, Operation, ScopeKey
from ..operations import OperationStore


@dataclass(frozen=True)
class DeletionReceipt:
    operation_id: str
    tombstone_revision: int
    graph_state: str
    vector_state: str
    object_state: str
    cache_state: str
    backup_state: str
    completed_at: Optional[datetime]


def _connect(dsn: str):
    return psycopg.connect(dsn, row_factory=dict_row)


class DeletionService:
    """Applies deletion barriers and tracks per-store cleanup."""

    def __init__(self, dsn: str, operations: OperationStore):
        self._dsn = dsn
        self._operations = operations

    def close(self) -> None:
        return None

    def delete_revision(self, scope: ScopeKey, document_id: str, revision: int) -> None:
        """Persist a monotone tombstone and revoke the revision's support."""
        with _connect(self._dsn) as conn:
            conn.execute(
                "INSERT INTO semantic.tombstones (tenant_id, kb_id, document_id, revision) VALUES (%s, %s, %s, %s) ON CONFLICT DO NOTHING",
                (scope.tenant_id, scope.kb_id, document_id, revision),
            )
            self._revoke_supports(conn, scope, document_id, revision)
            conn.commit()

    def denied(self, scope: ScopeKey, document_id: str, revision: int) -> bool:
        with _connect(self._dsn) as conn:
            row = conn.execute(
                "SELECT 1 FROM semantic.tombstones WHERE tenant_id = %s AND kb_id = %s AND document_id = %s AND revision >= %s LIMIT 1",
                (scope.tenant_id, scope.kb_id, document_id, revision),
            ).fetchone()
        return row is not None

    def visible_assertion(self, scope: ScopeKey, assertion_id: str) -> bool:
        """Visible iff marked visible AND at least one support row survives
        the tombstones (single query, no N+1)."""
        with _connect(self._dsn) as conn:
            row = conn.execute(
                """
                SELECT 1 FROM semantic.assertions a
                WHERE a.tenant_id = %s AND a.kb_id = %s AND a.assertion_id = %s
                  AND a.visible = TRUE
                  AND NOT EXISTS (
                    SELECT 1 FROM semantic.tombstones t
                    WHERE t.tenant_id = a.tenant_id AND t.kb_id = a.kb_id
                      AND t.document_id = a.support_document_id
                      AND t.revision >= a.support_revision)
                LIMIT 1
                """,
                (scope.tenant_id, scope.kb_id, assertion_id),
            ).fetchone()
        return row is not None

    def add_assertion_support(self, scope: ScopeKey, assertion_id: str, subject_id: str,
                              predicate: str, *, object_id=None, value=None, kind="source",
                              support_document_id: str = "", support_revision: int = 1,
                              evidence_ids=(), premise_ids=()) -> None:
        """Store one support row. Replay-aware: a row whose (document,
        revision) is already tombstoned is stored INVISIBLE so a replayed
        event after delete cannot resurrect deleted data."""
        with _connect(self._dsn) as conn:
            conn.execute(
                """
                INSERT INTO semantic.assertions
                  (tenant_id, kb_id, assertion_id, subject_id, predicate, object_id, value, kind,
                   support_document_id, support_revision, evidence_ids, premise_ids, visible)
                VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,
                        NOT EXISTS (SELECT 1 FROM semantic.tombstones t
                                    WHERE t.tenant_id = %s AND t.kb_id = %s
                                      AND t.document_id = %s AND t.revision >= %s))
                ON CONFLICT DO NOTHING
                """,
                (scope.tenant_id, scope.kb_id, assertion_id, subject_id, predicate,
                 object_id, value, kind, support_document_id, support_revision,
                 json.dumps(list(evidence_ids)), json.dumps(list(premise_ids)),
                 scope.tenant_id, scope.kb_id, support_document_id, support_revision),
            )
            # A derived row inserted AFTER its premise died must not be born
            # visible: run the conjunctive fixpoint in the same transaction.
            self._invalidate_derived(conn, scope)
            conn.commit()

    def _revoke_supports(self, conn, scope: ScopeKey, document_id: str, revision: int) -> None:
        conn.execute(
            "UPDATE semantic.assertions SET visible = FALSE WHERE tenant_id = %s AND kb_id = %s"
            " AND support_document_id = %s AND support_revision <= %s",
            (scope.tenant_id, scope.kb_id, document_id, revision),
        )
        self._invalidate_derived(conn, scope)

    def _invalidate_derived(self, conn, scope: ScopeKey) -> None:
        """Fixpoint with CONJUNCTIVE premise semantics: a derived assertion
        dies when ANY premise id has no visible support left. Idempotent
        (visible=FALSE rows are skipped)."""
        while True:
            invalidated = conn.execute(
                """
                UPDATE semantic.assertions a SET visible = FALSE
                WHERE a.tenant_id = %s AND a.kb_id = %s AND a.visible = TRUE
                  AND a.kind IN ('rule','model')
                  AND a.premise_ids != '[]'
                  AND EXISTS (
                    SELECT 1
                    FROM jsonb_array_elements_text(a.premise_ids::jsonb) AS pid
                    WHERE NOT EXISTS (
                      SELECT 1 FROM semantic.assertions p
                      WHERE p.tenant_id = a.tenant_id AND p.kb_id = a.kb_id
                        AND p.assertion_id = pid
                        AND p.visible = TRUE))
                """,
                (scope.tenant_id, scope.kb_id),
            ).rowcount
            if not invalidated:
                break

    def start_receipt(self, scope: ScopeKey, operation_id: str, document_id: str, tombstone_revision: int) -> None:
        with _connect(self._dsn) as conn:
            conn.execute(
                "INSERT INTO semantic.deletion_receipts"
                " (operation_id, tenant_id, kb_id, document_id, tombstone_revision)"
                " VALUES (%s, %s, %s, %s, %s) ON CONFLICT (operation_id) DO NOTHING",
                (operation_id, scope.tenant_id, scope.kb_id, document_id, tombstone_revision),
            )
            conn.commit()

    _STORES = ("graph", "vector", "object", "cache")

    def mark_store_cleaned(self, operation_id: str, store: str) -> bool:
        if store not in self._STORES:
            raise ValueError(f"unknown store {store!r}")
        with _connect(self._dsn) as conn:
            changed = conn.execute(
                "UPDATE semantic.deletion_receipts SET " + store + "_state = 'cleaned', updated_at = now()"
                " WHERE operation_id = %s AND " + store + "_state = 'pending'",
                (operation_id,),
            ).rowcount
            row = conn.execute(
                "SELECT graph_state, vector_state, object_state, cache_state FROM semantic.deletion_receipts WHERE operation_id = %s",
                (operation_id,),
            ).fetchone()
            if row is not None and all(row[s + "_state"] == "cleaned" for s in self._STORES):
                conn.execute(
                    "UPDATE semantic.deletion_receipts SET completed_at = now() WHERE operation_id = %s AND completed_at IS NULL",
                    (operation_id,),
                )
            conn.commit()
        return changed == 1

    def receipt(self, operation_id: str) -> Optional[DeletionReceipt]:
        with _connect(self._dsn) as conn:
            row = conn.execute(
                "SELECT * FROM semantic.deletion_receipts WHERE operation_id = %s",
                (operation_id,),
            ).fetchone()
        if row is None:
            return None
        return DeletionReceipt(
            operation_id=row["operation_id"],
            tombstone_revision=row["tombstone_revision"],
            graph_state=row["graph_state"],
            vector_state=row["vector_state"],
            object_state=row["object_state"],
            cache_state=row["cache_state"],
            backup_state=row["backup_state"],
            completed_at=row["completed_at"],
        )

    def apply(self, document: DocumentRevision) -> Operation:
        """Service-side delete admission (idempotent per document+revision)."""
        if not document.deleted:
            raise ValueError("DeletionService.apply requires deleted=True")
        from ..contracts import ApplyRequest
        request = ApplyRequest(document=document, idempotency_key="del-" + document.document_id + "-" + str(document.revision),
                                payload_hash=document.content_hash)
        operation = self._operations.accept(request)
        self.delete_revision(document.scope, document.document_id, document.revision)
        self.start_receipt(document.scope, operation.operation_id, document.document_id, document.revision)
        return operation

    def cleanup(self, operation_id: str) -> DeletionReceipt:
        receipt = self.receipt(operation_id)
        if receipt is None:
            raise ValueError("no deletion receipt for " + repr(operation_id))
        return receipt
