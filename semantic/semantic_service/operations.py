"""PostgreSQL-backed, lease-fenced semantic operation persistence."""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
from pathlib import Path
import re
from uuid import uuid4

import psycopg
from psycopg import sql
from psycopg.rows import dict_row

from semantic_service.contracts import (
    ApplyRequest,
    Operation,
    ScopeKey,
    apply_request_from_wire,
    apply_request_to_wire,
)
from semantic_service.proto import semantic_pb2


class OperationError(RuntimeError):
    """Base error for durable operation operations."""


class OperationPayloadConflict(OperationError):
    """An existing idempotency identity has a different payload."""


class OperationNotFound(OperationError):
    """An operation does not exist in the supplied scope."""


class OperationFailedPrecondition(OperationError):
    """A requested change is not valid for the durable operation state."""


class OperationPhase(str, Enum):
    ACCEPTED = "accepted"
    RUNNING = "running"
    STAGED = "staged"
    PUBLISHING = "publishing"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELLED = "cancelled"
    SUPERSEDED = "superseded"


TERMINAL_PHASES = frozenset({
    OperationPhase.SUCCEEDED, OperationPhase.FAILED, OperationPhase.CANCELLED, OperationPhase.SUPERSEDED,
})
_ALLOWED_TRANSITIONS = {
    (OperationPhase.ACCEPTED, OperationPhase.RUNNING),
    (OperationPhase.RUNNING, OperationPhase.STAGED),
    (OperationPhase.STAGED, OperationPhase.PUBLISHING),
    (OperationPhase.PUBLISHING, OperationPhase.SUCCEEDED),
    *((phase, terminal) for phase in (OperationPhase.ACCEPTED, OperationPhase.RUNNING, OperationPhase.STAGED, OperationPhase.PUBLISHING)
      for terminal in (OperationPhase.FAILED, OperationPhase.CANCELLED, OperationPhase.SUPERSEDED)),
}
_SCHEMA_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]*\Z")


@dataclass(frozen=True)
class LeasedOperation:
    operation: Operation
    request: ApplyRequest
    worker_id: str
    lease_token: int


class PostgresOperationStore:
    def __init__(self, dsn: str, schema: str = "semantic_service") -> None:
        if not dsn:
            raise ValueError("PostgreSQL DSN is required")
        if not _SCHEMA_RE.fullmatch(schema):
            raise ValueError("schema must be a PostgreSQL identifier")
        self.dsn = dsn
        self.schema = schema
        self._schema = sql.Identifier(schema)

    def _connect(self):
        return psycopg.connect(self.dsn, row_factory=dict_row)

    def _query(self, template: str) -> sql.Composed:
        return sql.SQL(template).format(schema=self._schema)

    def migrate(self) -> None:
        migration = (Path(__file__).parent.parent / "migrations" / "001_operations.sql").read_text()
        with self._connect() as connection:
            with connection.cursor() as cursor:
                cursor.execute(sql.SQL("CREATE SCHEMA IF NOT EXISTS {}").format(self._schema))
                cursor.execute(migration.replace("{{schema}}", self._schema.as_string(connection)))

    def accept(self, request: ApplyRequest) -> Operation:
        request_bytes = apply_request_to_wire(request).SerializeToString(deterministic=True)
        scope = request.document.scope
        for attempt in range(2):
            try:
                with self._connect() as connection:
                    with connection.cursor() as cursor:
                        cursor.execute(self._query("""
                            SELECT * FROM {schema}.operations
                            WHERE tenant_id = %s AND kb_id = %s
                              AND (idempotency_key = %s OR (document_id = %s AND revision = %s AND config_digest = %s))
                            FOR UPDATE
                        """), (scope.tenant_id, scope.kb_id, request.idempotency_key, request.document.document_id, request.document.revision, request.config.config_digest))
                        row = cursor.fetchone()
                        if row is not None:
                            same_document_identity = (
                                row["document_id"] == request.document.document_id
                                and int(row["revision"]) == request.document.revision
                                and row["config_digest"] == request.config.config_digest
                            )
                            if row["payload_hash"] != request.payload_hash or (row["idempotency_key"] == request.idempotency_key and not same_document_identity):
                                raise OperationPayloadConflict("operation identity has a different payload hash")
                            return self._operation(row)
                        operation_id = str(uuid4())
                        cursor.execute(self._query("""
                            INSERT INTO {schema}.operations
                              (operation_id, tenant_id, kb_id, document_id, revision, config_digest, idempotency_key, payload_hash, phase, request_bytes)
                            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'accepted', %s)
                            RETURNING *
                        """), (operation_id, scope.tenant_id, scope.kb_id, request.document.document_id, request.document.revision,
                                request.config.config_digest, request.idempotency_key, request.payload_hash, request_bytes))
                        return self._operation(cursor.fetchone())
            except psycopg.errors.UniqueViolation:
                if attempt:
                    raise
        raise AssertionError("unreachable")

    def get(self, scope: ScopeKey, operation_id: str) -> Operation:
        with self._connect() as connection:
            with connection.cursor() as cursor:
                cursor.execute(self._query("SELECT * FROM {schema}.operations WHERE tenant_id = %s AND kb_id = %s AND operation_id = %s"),
                               (scope.tenant_id, scope.kb_id, operation_id))
                row = cursor.fetchone()
        if row is None:
            raise OperationNotFound("operation not found")
        return self._operation(row)

    def claim(self, worker_id: str, lease_seconds: int) -> LeasedOperation | None:
        self._validate_lease(worker_id, lease_seconds)
        with self._connect() as connection:
            with connection.cursor() as cursor:
                cursor.execute(self._query("""
                    SELECT * FROM {schema}.operations
                    WHERE phase NOT IN ('succeeded', 'failed', 'cancelled', 'superseded')
                      AND (phase = 'accepted' OR lease_until IS NULL OR lease_until <= clock_timestamp())
                    ORDER BY created_at
                    FOR UPDATE SKIP LOCKED
                    LIMIT 1
                """))
                row = cursor.fetchone()
                if row is None:
                    return None
                cursor.execute(self._query("""
                    UPDATE {schema}.operations
                    SET phase = CASE WHEN phase = 'accepted' THEN 'running' ELSE phase END,
                        lease_owner = %s,
                        lease_until = clock_timestamp() + (%s * interval '1 second'),
                        lease_token = lease_token + 1,
                        updated_at = clock_timestamp()
                    WHERE operation_id = %s
                    RETURNING *
                """), (worker_id, lease_seconds, row["operation_id"]))
                claimed = cursor.fetchone()
        request_bytes = claimed["request_bytes"]
        if request_bytes is None:
            raise OperationFailedPrecondition("nonterminal operation has no durable request")
        wire = semantic_pb2.ApplyRequest()
        wire.ParseFromString(request_bytes)
        return LeasedOperation(self._operation(claimed), apply_request_from_wire(wire), worker_id, int(claimed["lease_token"]))

    def renew(self, operation_id: str, worker_id: str, lease_token: int, lease_seconds: int) -> bool:
        self._validate_lease(worker_id, lease_seconds)
        with self._connect() as connection:
            with connection.cursor() as cursor:
                cursor.execute(self._query("""
                    UPDATE {schema}.operations
                    SET lease_until = clock_timestamp() + (%s * interval '1 second'), updated_at = clock_timestamp()
                    WHERE operation_id = %s AND lease_owner = %s AND lease_token = %s
                      AND lease_until > clock_timestamp()
                      AND phase NOT IN ('succeeded', 'failed', 'cancelled', 'superseded')
                """), (lease_seconds, operation_id, worker_id, lease_token))
                return cursor.rowcount == 1

    def transition(self, operation_id: str, worker_id: str, lease_token: int, expected: OperationPhase, next: OperationPhase) -> bool:
        if (expected, next) not in _ALLOWED_TRANSITIONS:
            raise OperationFailedPrecondition("operation transition is not allowed")
        terminal = next in TERMINAL_PHASES
        with self._connect() as connection:
            with connection.cursor() as cursor:
                cursor.execute(self._query("""
                    UPDATE {schema}.operations
                    SET phase = %s,
                        request_bytes = CASE WHEN %s THEN NULL ELSE request_bytes END,
                        lease_owner = CASE WHEN %s THEN NULL ELSE lease_owner END,
                        lease_until = CASE WHEN %s THEN NULL ELSE lease_until END,
                        error_code = CASE WHEN %s = 'superseded' THEN 'operation_superseded' ELSE error_code END,
                        updated_at = clock_timestamp()
                    WHERE operation_id = %s AND phase = %s AND lease_owner = %s AND lease_token = %s
                      AND lease_until > clock_timestamp()
                """), (next.value, terminal, terminal, terminal, next.value, operation_id, expected.value, worker_id, lease_token))
                return cursor.rowcount == 1

    def cancel(self, scope: ScopeKey, operation_id: str) -> Operation:
        with self._connect() as connection:
            with connection.cursor() as cursor:
                cursor.execute(self._query("SELECT * FROM {schema}.operations WHERE tenant_id = %s AND kb_id = %s AND operation_id = %s FOR UPDATE"),
                               (scope.tenant_id, scope.kb_id, operation_id))
                row = cursor.fetchone()
                if row is None:
                    raise OperationNotFound("operation not found")
                phase = OperationPhase(row["phase"])
                if phase in TERMINAL_PHASES:
                    raise OperationFailedPrecondition("terminal operation cannot be cancelled")
                cursor.execute(self._query("""
                    UPDATE {schema}.operations
                    SET phase = 'cancelled', request_bytes = NULL, lease_owner = NULL, lease_until = NULL,
                        lease_token = lease_token + 1, updated_at = clock_timestamp()
                    WHERE operation_id = %s
                    RETURNING *
                """), (operation_id,))
                return self._operation(cursor.fetchone())

    @staticmethod
    def _validate_lease(worker_id: str, lease_seconds: int) -> None:
        if not worker_id or type(lease_seconds) is not int or lease_seconds < 1:
            raise ValueError("worker id and positive lease seconds are required")

    @staticmethod
    def _operation(row: dict) -> Operation:
        phase = OperationPhase(row["phase"])
        state, stage, error_code = {
            OperationPhase.ACCEPTED: ("pending", "accepted", row["error_code"]),
            OperationPhase.RUNNING: ("running", "running", row["error_code"]),
            OperationPhase.STAGED: ("running", "staged", row["error_code"]),
            OperationPhase.PUBLISHING: ("running", "publishing", row["error_code"]),
            OperationPhase.SUCCEEDED: ("succeeded", "succeeded", row["error_code"]),
            OperationPhase.FAILED: ("failed", "failed", row["error_code"]),
            OperationPhase.CANCELLED: ("cancelled", "cancelled", row["error_code"]),
            OperationPhase.SUPERSEDED: ("failed", "superseded", "operation_superseded"),
        }[phase]
        return Operation(row["operation_id"], ScopeKey(int(row["tenant_id"]), row["kb_id"]), row["document_id"], int(row["revision"]),
                         state, stage, int(row["lease_token"]), row["result_generation"], error_code)
