import dataclasses
import threading
from concurrent.futures import ThreadPoolExecutor

import psycopg
import pytest
from psycopg import sql

from semantic_service.operations import (
    OperationFailedPrecondition,
    OperationNotFound,
    OperationPayloadConflict,
    OperationPhase,
)
from semantic_service.contracts import UINT64_MAX, apply_request_to_wire


def expire_lease_at_database_clock(store, operation_id):
    with psycopg.connect(store.dsn) as connection:
        with connection.cursor() as cursor:
            cursor.execute(
                sql.SQL("UPDATE {}.operations SET lease_until = clock_timestamp() - interval '1 second' WHERE operation_id = %s").format(sql.Identifier(store.schema)),
                (operation_id,),
            )


def request_bytes_for_test(store, operation_id):
    with psycopg.connect(store.dsn) as connection:
        return connection.execute(
            sql.SQL("SELECT request_bytes FROM {}.operations WHERE operation_id = %s").format(sql.Identifier(store.schema)),
            (operation_id,),
        ).fetchone()[0]


def set_lease_token_for_test(store, operation_id, lease_token):
    with psycopg.connect(store.dsn) as connection:
        connection.execute(
            sql.SQL("UPDATE {}.operations SET lease_token = %s WHERE operation_id = %s").format(sql.Identifier(store.schema)),
            (lease_token, operation_id),
        )


def test_repeated_apply_returns_same_operation(operation_store, apply_request):
    first = operation_store.accept(apply_request)
    second = operation_store.accept(apply_request)
    assert second.operation_id == first.operation_id
    assert operation_store.get(apply_request.document.scope, first.operation_id) == first


def test_reused_idempotency_identity_with_different_hash_conflicts(operation_store, apply_request):
    operation_store.accept(apply_request)
    with pytest.raises(OperationPayloadConflict):
        operation_store.accept(dataclasses.replace(apply_request, payload_hash="different-hash"))


def test_same_key_with_different_document_identity_conflicts(operation_store, apply_request):
    operation_store.accept(apply_request)
    changed = dataclasses.replace(apply_request, document=dataclasses.replace(apply_request.document, document_id="other-document"))
    with pytest.raises(OperationPayloadConflict):
        operation_store.accept(changed)


def test_same_document_revision_and_config_deduplicates_across_request_retry_key(operation_store, apply_request):
    first = operation_store.accept(apply_request)
    second = operation_store.accept(dataclasses.replace(apply_request, idempotency_key="retry-key"))
    assert second.operation_id == first.operation_id


def test_document_identity_unique_key_with_different_payload_hash_conflicts(operation_store, apply_request):
    first = dataclasses.replace(apply_request, idempotency_key="identity-a", payload_hash="hash-a")
    operation_store.accept(first)
    with pytest.raises(OperationPayloadConflict):
        operation_store.accept(dataclasses.replace(first, idempotency_key="identity-b", payload_hash="hash-b"))


def test_collision_between_idempotency_and_document_identities_conflicts(operation_store, apply_request):
    first = dataclasses.replace(apply_request, idempotency_key="key-a", document=dataclasses.replace(apply_request.document, document_id="document-a"))
    second = dataclasses.replace(apply_request, idempotency_key="key-b", document=dataclasses.replace(apply_request.document, document_id="document-b"))
    operation_store.accept(first)
    operation_store.accept(second)
    with pytest.raises(OperationPayloadConflict):
        operation_store.accept(dataclasses.replace(second, idempotency_key="key-a"))


def test_get_and_cancel_do_not_cross_scope(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    wrong_scopes = (
        dataclasses.replace(apply_request.document.scope, tenant_id=apply_request.document.scope.tenant_id + 1),
        dataclasses.replace(apply_request.document.scope, kb_id="other-kb"),
    )
    for wrong_scope in wrong_scopes:
        with pytest.raises(OperationNotFound):
            operation_store.get(wrong_scope, operation.operation_id)
        with pytest.raises(OperationNotFound):
            operation_store.cancel(wrong_scope, operation.operation_id)
    assert operation_store.get(apply_request.document.scope, operation.operation_id) == operation


def test_two_postgres_connections_cannot_claim_same_live_lease(operation_store, apply_request):
    operation_store.accept(apply_request)
    with ThreadPoolExecutor(max_workers=2) as pool:
        claims = list(pool.map(lambda worker: operation_store.claim(worker, 30), ("w1", "w2")))
    assert sum(claim is not None for claim in claims) == 1


def test_expired_claim_gets_new_fence_and_stale_or_wrong_workers_are_fenced(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    first = operation_store.claim("worker-old", 30)
    assert first is not None
    expire_lease_at_database_clock(operation_store, operation.operation_id)
    second = operation_store.claim("worker-new", 30)
    assert second is not None and second.lease_token == first.lease_token + 1
    assert not operation_store.renew(operation.operation_id, "worker-old", first.lease_token, 30)
    assert not operation_store.transition(operation.operation_id, "worker-old", first.lease_token, OperationPhase.RUNNING, OperationPhase.STAGED, "staged")
    assert not operation_store.renew(operation.operation_id, "wrong-worker", second.lease_token, 30)
    assert not operation_store.renew(operation.operation_id, "worker-new", second.lease_token - 1, 30)
    assert not operation_store.transition(operation.operation_id, "wrong-worker", second.lease_token, OperationPhase.RUNNING, OperationPhase.STAGED, "staged")
    assert not operation_store.transition(operation.operation_id, "worker-new", second.lease_token - 1, OperationPhase.RUNNING, OperationPhase.STAGED, "staged")


def test_cancel_and_publish_race_has_one_terminal_winner(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.RUNNING, OperationPhase.STAGED, "staged")
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.STAGED, OperationPhase.PUBLISHING, "publishing")
    barrier = threading.Barrier(2)
    def cancel():
        barrier.wait()
        try:
            operation_store.cancel(apply_request.document.scope, operation.operation_id)
            return "cancelled"
        except OperationFailedPrecondition:
            return "already-succeeded"
    def publish():
        barrier.wait()
        return "succeeded" if operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.PUBLISHING, OperationPhase.SUCCEEDED, "succeeded", result_generation="gen-1") else "lost"
    with ThreadPoolExecutor(max_workers=2) as pool:
        cancel_future = pool.submit(cancel)
        publish_future = pool.submit(publish)
        outcomes = {cancel_future.result(), publish_future.result()}
    final = operation_store.get(apply_request.document.scope, operation.operation_id)
    assert final.state in {"succeeded", "cancelled"}
    assert len(outcomes & {"cancelled", "succeeded"}) == 1
    if final.state == "cancelled":
        assert not operation_store.renew(operation.operation_id, "worker", claim.lease_token, 30)
        assert not operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.PUBLISHING, OperationPhase.SUCCEEDED, "succeeded")


def test_accept_survives_store_recreation(operation_store_factory, apply_request):
    first = operation_store_factory().accept(apply_request)
    second = operation_store_factory().accept(apply_request)
    assert second.operation_id == first.operation_id


def test_succeeded_operation_cannot_be_cancelled_and_terminal_clears_request(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None and claim.request == apply_request
    assert request_bytes_for_test(operation_store, operation.operation_id) == apply_request_to_wire(apply_request).SerializeToString(deterministic=True)
    for expected, next_phase in ((OperationPhase.RUNNING, OperationPhase.STAGED), (OperationPhase.STAGED, OperationPhase.PUBLISHING), (OperationPhase.PUBLISHING, OperationPhase.SUCCEEDED)):
        assert operation_store.transition(operation.operation_id, "worker", claim.lease_token, expected, next_phase, next_phase.value, result_generation="generation-1" if next_phase == OperationPhase.SUCCEEDED else None)
    with pytest.raises(OperationFailedPrecondition):
        operation_store.cancel(apply_request.document.scope, operation.operation_id)
    assert operation_store.claim("other-worker", 30) is None
    assert operation_store.get(apply_request.document.scope, operation.operation_id).result_generation == "generation-1"
    assert request_bytes_for_test(operation_store, operation.operation_id) is None


@pytest.mark.parametrize("terminal_phase", (OperationPhase.FAILED, OperationPhase.SUPERSEDED, OperationPhase.CANCELLED))
def test_every_terminal_phase_clears_durable_request_bytes(operation_store, apply_request, terminal_phase):
    operation = operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert request_bytes_for_test(operation_store, operation.operation_id) is not None
    if terminal_phase == OperationPhase.CANCELLED:
        operation_store.cancel(apply_request.document.scope, operation.operation_id)
        assert not operation_store.renew(operation.operation_id, "worker", claim.lease_token, 30)
        assert not operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.RUNNING, OperationPhase.FAILED, "failed")
    else:
        assert operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.RUNNING, terminal_phase, terminal_phase.value, error_code="build-failed" if terminal_phase == OperationPhase.FAILED else None)
    assert request_bytes_for_test(operation_store, operation.operation_id) is None


def test_failed_transition_persists_error_and_stage(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.RUNNING, OperationPhase.FAILED, "build", error_code="provider-unavailable")
    failed = operation_store.get(apply_request.document.scope, operation.operation_id)
    assert failed.state == "failed" and failed.stage == "build" and failed.error_code == "provider-unavailable"


def test_store_migration_is_idempotent_across_restarts(operation_store_factory):
    operation_store_factory().migrate()
    operation_store_factory().migrate()


def test_store_migration_serializes_concurrent_startup(operation_store_factory):
    with ThreadPoolExecutor(max_workers=2) as pool:
        futures = [pool.submit(operation_store_factory().migrate) for _ in range(2)]
        for future in futures:
            future.result()


def test_exhausted_lease_fence_fails_without_exceeding_uint64(operation_store_factory, apply_request):
    store = operation_store_factory()
    operation = store.accept(apply_request)
    set_lease_token_for_test(store, operation.operation_id, UINT64_MAX)
    with pytest.raises(OperationFailedPrecondition):
        store.claim("worker", 30)
    assert store.get(apply_request.document.scope, operation.operation_id).lease_token == UINT64_MAX


def test_c01_projection_preserves_internal_stage(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    assert operation.state == "pending" and operation.stage == "accepted"
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token, OperationPhase.RUNNING, OperationPhase.STAGED, "custom-stage")
    staged = operation_store.get(apply_request.document.scope, operation.operation_id)
    assert staged.state == "running" and staged.stage == "custom-stage"
