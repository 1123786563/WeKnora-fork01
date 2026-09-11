"""Persistent operations: idempotent acceptance, worker leases, CAS
transitions (I01).

Backed by the service's own PostgreSQL schema (semantic.operations). The
lease discipline follows the plan's constraint: every stage transition is a
single UPDATE guarded by state + lease_token + lease_until; rowcount 0 means
the worker lost the lease or lost a race and MUST NOT continue publishing.
Claims use FOR UPDATE SKIP LOCKED so concurrent workers hand out each
operation exactly once.
"""

from __future__ import annotations

import json
import uuid
from pathlib import Path

import psycopg
from psycopg.rows import dict_row

from .contracts import (
    ApplyRequest,
    Operation,
    ScopeKey,
    apply_request_from_json,
    apply_request_to_json,
)

TERMINAL_STATES = ("succeeded", "failed", "cancelled", "superseded")

ALLOWED_TRANSITIONS = {
    ("accepted", "running"),
    ("accepted", "failed"),
    ("accepted", "cancelled"),
    ("accepted", "superseded"),
    ("running", "staged"),
    ("running", "failed"),
    ("running", "cancelled"),
    ("running", "superseded"),
    ("staged", "publishing"),
    ("staged", "failed"),
    ("staged", "cancelled"),
    ("staged", "superseded"),
    ("publishing", "succeeded"),
    ("publishing", "failed"),
    ("publishing", "cancelled"),
    ("publishing", "superseded"),
}

MIGRATIONS_DIR = Path(__file__).resolve().parents[1] / "migrations"


class OperationConflictError(Exception):
    """Same (scope, idempotency key) presented with a different payload."""


class OperationNotFoundError(Exception):
    """The referenced operation does not exist in the requested scope."""


class RequestTooLargeError(ValueError):
    """Inline request exceeds MAX_REQUEST_BYTES; large documents must use
    manifest_ref (maps to INVALID_ARGUMENT at the RPC layer)."""


# Total serialized request budget: the request column carries chunk payloads
# for inline (small) documents; bigger documents must use manifest_ref.
MAX_REQUEST_BYTES = 4 * 1024 * 1024


class InvalidTransitionError(Exception):
    """The requested transition is not in the state machine's table."""


class OperationStateError(Exception):
    """The operation is in a state that refuses the request (e.g. cancel
    after a successful publish -> FAILED_PRECONDITION)."""


def _connect(dsn: str):
    """Connection with dict rows (every accessor uses column names)."""
    return psycopg.connect(dsn, row_factory=dict_row)


def apply_migrations(dsn: str) -> None:
    """Apply pending migrations in order, exactly once each.

    An advisory lock serializes concurrent migrators (multi-process starts).
    001 executes as the idempotent ledger bootstrap (IF NOT EXISTS) and is
    then recorded like every other migration.
    """
    with _connect(dsn) as connection:
        connection.execute("SELECT pg_advisory_lock(835471001)")
        try:
            # Idempotent bootstrap: creates schema + ledger; 001 is then
            # then tracked like every other migration.
            ledger = Path(MIGRATIONS_DIR / "001_operations.sql").read_text(encoding="utf-8")
            connection.execute(ledger)
            for path in sorted(MIGRATIONS_DIR.glob("*.sql")):
                version = path.stem
                applied = connection.execute(
                    "SELECT 1 FROM semantic.schema_migrations WHERE version = %s", (version,)
                ).fetchone()
                if applied:
                    continue
                connection.execute(path.read_text(encoding="utf-8"))
                connection.execute(
                    "INSERT INTO semantic.schema_migrations (version) VALUES (%s)", (version,)
                )
            connection.commit()
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.execute("SELECT pg_advisory_unlock(835471001)")


def _row_to_operation(row) -> Operation:
    return Operation(
        operation_id=row["operation_id"],
        scope=ScopeKey(tenant_id=row["tenant_id"], kb_id=row["kb_id"]),
        document_id=row["document_id"],
        revision=row["revision"],
        state=row["state"],
        stage=row["stage"],
        lease_token=row["lease_token"],
        result_generation=row["result_generation"],
        error_code=row["error_code"],
    )


class OperationStore:
    """Repository over semantic.operations. One connection per call keeps
    concurrent stores independent; pooling arrives with deployment (O01)."""

    def __init__(self, dsn: str):
        self._dsn = dsn

    @property
    def dsn(self) -> str:
        return self._dsn

    def close(self) -> None:
        # Connections are per-call; nothing persistent to close.
        return None

    # -- acceptance ---------------------------------------------------------

    def accept(self, request: ApplyRequest) -> Operation:
        """Idempotent acceptance keyed by (tenant, kb, idempotency_key).

        A repeated delivery with the SAME payload hash returns the existing
        operation; a DIFFERENT payload hash under the same key is a conflict.
        """
        operation_id = str(uuid.uuid4())
        request_json = json.dumps(apply_request_to_json(request), ensure_ascii=False, sort_keys=True)
        if len(request_json.encode("utf-8")) > MAX_REQUEST_BYTES:
            raise RequestTooLargeError(
                f"inline request too large ({len(request_json.encode('utf-8'))} bytes > "
                f"{MAX_REQUEST_BYTES}); large documents must use manifest_ref"
            )
        with _connect(self._dsn) as connection:
            inserted = connection.execute(
                """
                INSERT INTO semantic.operations
                    (operation_id, tenant_id, kb_id, document_id, revision,
                     idempotency_key, payload_hash, state, request)
                VALUES (%s, %s, %s, %s, %s, %s, %s, 'accepted', %s)
                ON CONFLICT (tenant_id, kb_id, idempotency_key) DO NOTHING
                RETURNING operation_id, tenant_id, kb_id, document_id, revision,
                          state, stage, lease_token, result_generation, error_code
                """,
                (
                    operation_id,
                    request.document.scope.tenant_id,
                    request.document.scope.kb_id,
                    request.document.document_id,
                    request.document.revision,
                    request.idempotency_key,
                    request.payload_hash,
                    request_json,
                ),
            ).fetchone()
            connection.commit()
            if inserted is not None:
                return _row_to_operation(inserted)

        existing = self._fetch_by_idempotency(request.document.scope, request.idempotency_key)
        if existing is None:  # pragma: no cover - defensive
            raise OperationStateError("idempotent insert raced unexpectedly")
        if existing["payload_hash"] != request.payload_hash:
            raise OperationConflictError(
                f"idempotency key {request.idempotency_key!r} already used with a "
                f"different payload hash ({existing['payload_hash']!r} != {request.payload_hash!r})"
            )
        return _row_to_operation(existing)

    # -- leases -------------------------------------------------------------

    def claim(self, worker_id: str, lease_seconds: int) -> Operation | None:
        """Claim the oldest claimable operation FOR UPDATE SKIP LOCKED.

        Claimable = accepted-but-unstarted or running with an expired lease
        (dead-worker takeover). Claiming increments the fencing token.
        """
        with _connect(self._dsn) as connection:
            row = connection.execute(
                """
                WITH candidate AS (
                    SELECT operation_id FROM semantic.operations
                    WHERE state IN ('accepted', 'running')
                      AND (lease_until IS NULL OR lease_until <= now())
                    ORDER BY created_at
                    FOR UPDATE SKIP LOCKED
                    LIMIT 1
                )
                UPDATE semantic.operations o
                SET state = 'running',
                    lease_token = o.lease_token + 1,
                    lease_owner = %s,
                    lease_until = now() + (%s * interval '1 second'),
                    updated_at = now()
                FROM candidate
                WHERE o.operation_id = candidate.operation_id
                RETURNING o.operation_id, o.tenant_id, o.kb_id, o.document_id,
                          o.revision, o.state, o.stage, o.lease_token,
                          o.result_generation, o.error_code
                """,
                (worker_id, lease_seconds),
            ).fetchone()
            connection.commit()
        if row is None:
            return None
        return _row_to_operation(row)

    def renew(self, operation_id: str, lease_token: int, lease_seconds: int) -> bool:
        with _connect(self._dsn) as connection:
            rowcount = connection.execute(
                """
                UPDATE semantic.operations
                SET lease_until = now() + (%s * interval '1 second'), updated_at = now()
                WHERE operation_id = %s AND lease_token = %s
                  AND state NOT IN ('succeeded', 'failed', 'cancelled', 'superseded')
                  AND lease_until IS NOT NULL
                """,
                (lease_seconds, operation_id, lease_token),
            ).rowcount
            connection.commit()
            return rowcount == 1

    # -- state machine ------------------------------------------------------

    def transition(self, operation_id: str, lease_token: int, expected: str, next_state: str) -> bool:
        """CAS transition guarded by state, lease token AND live lease.

        Returns False when the worker lost the lease, the state raced, or the
        source state is terminal (irreversible); raises InvalidTransitionError
        for transitions outside the state machine's table.
        """
        if expected in TERMINAL_STATES:
            return False
        if (expected, next_state) not in ALLOWED_TRANSITIONS:
            raise InvalidTransitionError(f"transition {expected!r} -> {next_state!r} is not allowed")
        with _connect(self._dsn) as connection:
            rowcount = connection.execute(
                """
                UPDATE semantic.operations
                SET state = %s, updated_at = now()
                WHERE operation_id = %s AND lease_token = %s AND state = %s
                  AND (
                        (lease_until > now())
                     OR (state = 'accepted' AND lease_until IS NULL)
                  )
                """,
                (next_state, operation_id, lease_token, expected),
            ).rowcount
            connection.commit()
            return rowcount == 1

    def mark_result(self, operation_id: str, lease_token: int, expected: str,
                    next_state: str, *, error_code: str | None = None,
                    result_generation: str | None = None) -> bool:
        """Terminal transition carrying an error code or generation result.

        Deliberately ONE statement: the terminal state, its error/result and
        the lease release commit atomically, so a crash can never leave a
        terminal operation without its result or with a dangling lease.
        """
        if expected in TERMINAL_STATES:
            return False
        if (expected, next_state) not in ALLOWED_TRANSITIONS:
            raise InvalidTransitionError(f"transition {expected!r} -> {next_state!r} is not allowed")
        if next_state not in TERMINAL_STATES:
            raise InvalidTransitionError(f"mark_result targets a terminal state, got {next_state!r}")
        with _connect(self._dsn) as connection:
            rowcount = connection.execute(
                """
                UPDATE semantic.operations
                SET state = %s,
                    error_code = COALESCE(%s, error_code),
                    result_generation = COALESCE(%s, result_generation),
                    lease_owner = NULL, lease_until = NULL,
                    updated_at = now()
                WHERE operation_id = %s AND lease_token = %s AND state = %s
                  AND (
                        (lease_until > now())
                     OR (state = 'accepted' AND lease_until IS NULL)
                  )
                """,
                (next_state, error_code, result_generation, operation_id, lease_token, expected),
            ).rowcount
            connection.commit()
            return rowcount == 1

    def cancel(self, scope: ScopeKey, operation_id: str) -> Operation:
        """Cancel before terminal; release the lease. Cancelling an already
        succeeded operation is FAILED_PRECONDITION (a published operation
        cannot be rolled back by Cancel)."""
        for _ in range(5):  # bounded retry on terminal races
            current = self.get(scope, operation_id)
            if current.state == "succeeded":
                raise OperationStateError(
                    f"FAILED_PRECONDITION: operation {operation_id} already succeeded; "
                    "published operations cannot be cancelled"
                )
            if current.state in TERMINAL_STATES:
                return current
            with _connect(self._dsn) as connection:
                row = connection.execute(
                    """
                    UPDATE semantic.operations
                    SET state = 'cancelled', lease_owner = NULL, lease_until = NULL,
                        updated_at = now()
                    WHERE tenant_id = %s AND kb_id = %s AND operation_id = %s
                      AND state NOT IN ('succeeded', 'failed', 'cancelled', 'superseded')
                    RETURNING operation_id, tenant_id, kb_id, document_id, revision,
                              state, stage, lease_token, result_generation, error_code
                    """,
                    (scope.tenant_id, scope.kb_id, operation_id),
                ).fetchone()
                connection.commit()
            if row is not None:
                return _row_to_operation(row)
        raise OperationStateError(f"cancel of {operation_id} lost every race; retry")

    # -- reads ----------------------------------------------------------

    def get(self, scope: ScopeKey, operation_id: str) -> Operation:
        row = self._fetch(scope, operation_id)
        if row is None:
            raise OperationNotFoundError(f"operation {operation_id!r} not found in scope")
        return _row_to_operation(row)

    def load_request(self, operation_id: str) -> ApplyRequest:
        with _connect(self._dsn) as connection:
            row = connection.execute(
                "SELECT request FROM semantic.operations WHERE operation_id = %s",
                (operation_id,),
            ).fetchone()
        if row is None:
            raise OperationNotFoundError(f"operation {operation_id!r} not found")
        return apply_request_from_json(json.loads(row["request"]))

    def _fetch(self, scope: ScopeKey, operation_id: str):
        with _connect(self._dsn) as connection:
            return connection.execute(
                """
                SELECT operation_id, tenant_id, kb_id, document_id, revision, state,
                       stage, lease_token, result_generation, error_code
                FROM semantic.operations
                WHERE tenant_id = %s AND kb_id = %s AND operation_id = %s
                """,
                (scope.tenant_id, scope.kb_id, operation_id),
            ).fetchone()

    def _fetch_by_idempotency(self, scope: ScopeKey, idempotency_key: str):
        with _connect(self._dsn) as connection:
            return connection.execute(
                """
                SELECT operation_id, tenant_id, kb_id, document_id, revision, state,
                       stage, lease_token, result_generation, error_code, payload_hash
                FROM semantic.operations
                WHERE tenant_id = %s AND kb_id = %s AND idempotency_key = %s
                """,
                (scope.tenant_id, scope.kb_id, idempotency_key),
            ).fetchone()

    def recoverable(self, limit: int = 100) -> list[str]:
        """Operations whose lease expired while running (dead workers) or
        that were never started - the recovery scan a starting worker runs.
        The predicate intentionally mirrors claim's candidate set."""
        with _connect(self._dsn) as connection:
            rows = connection.execute(
                """
                SELECT operation_id FROM semantic.operations
                WHERE state IN ('accepted', 'running')
                  AND (lease_until IS NULL OR lease_until <= now())
                ORDER BY created_at
                LIMIT %s
                """,
                (limit,),
            ).fetchall()
        return [row["operation_id"] for row in rows]

    # -- test helpers (never used in production paths) ---------------------

    def clear_for_test(self) -> None:
        with _connect(self._dsn) as connection:
            connection.execute("DELETE FROM semantic.operations")
            connection.commit()

    def lease_owner_for_test(self, operation_id: str):
        with _connect(self._dsn) as connection:
            row = connection.execute(
                "SELECT lease_owner FROM semantic.operations WHERE operation_id = %s",
                (operation_id,),
            ).fetchone()
        return row["lease_owner"] if row else None

    def lease_until_for_test(self, operation_id: str):
        with _connect(self._dsn) as connection:
            row = connection.execute(
                "SELECT lease_until FROM semantic.operations WHERE operation_id = %s",
                (operation_id,),
            ).fetchone()
        return row["lease_until"] if row else None

    def expire_lease_for_test(self, operation_id: str) -> None:
        with _connect(self._dsn) as connection:
            connection.execute(
                "UPDATE semantic.operations SET lease_until = now() - interval '1 second' "
                "WHERE operation_id = %s",
                (operation_id,),
            )
            connection.commit()
