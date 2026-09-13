"""Worker lease helpers over OperationStore (I01).

Deliberately thin: the durable logic lives in the store's SQL. The worker
wrapper exists so call sites never touch lease bookkeeping directly and so
dead-worker takeover is an explicit, observable operation.
"""

from __future__ import annotations

from .contracts import Operation
from .operations import OperationStore


class Worker:
    """A single worker identity claiming operations from the store."""

    def __init__(self, store: OperationStore, worker_id: str, lease_seconds: int = 30):
        self.store = store
        self.worker_id = worker_id
        self.lease_seconds = lease_seconds

    def claim(self) -> Operation | None:
        """Claim the next recoverable operation (increments the fencing
        token; expired leases from dead workers are take-over-able)."""
        return self.store.claim(self.worker_id, self.lease_seconds)

    def renew(self, operation: Operation) -> bool:
        return self.store.renew(operation.operation_id, operation.lease_token, self.lease_seconds)

    def transition(self, operation: Operation, expected: str, next_state: str) -> bool:
        return self.store.transition(
            operation.operation_id, operation.lease_token, expected, next_state
        )

    def recoverable(self, limit: int = 100) -> list[str]:
        """Proxy the store's recovery scan (expired/unstarted operations)."""
        return self.store.recoverable(limit)
