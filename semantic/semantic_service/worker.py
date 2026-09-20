"""Small lease-fenced worker seam for a later semantic processor."""

from semantic_service.operations import LeasedOperation, OperationPhase, PostgresOperationStore


class OperationWorker:
    def __init__(self, store: PostgresOperationStore) -> None:
        self._store = store

    def claim_next(self, worker_id: str, lease_seconds: int) -> LeasedOperation | None:
        return self._store.claim(worker_id, lease_seconds)

    def renew(self, operation_id: str, worker_id: str, lease_token: int, lease_seconds: int) -> bool:
        return self._store.renew(operation_id, worker_id, lease_token, lease_seconds)

    def transition(self, operation_id: str, worker_id: str, lease_token: int, expected: OperationPhase, next: OperationPhase) -> bool:
        return self._store.transition(operation_id, worker_id, lease_token, expected, next)
