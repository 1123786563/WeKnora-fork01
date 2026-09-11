"""I01 persistent operations, idempotency and worker leases (real PG).

Every test runs against an isolated PostgreSQL instance with the real
migrations applied - no in-memory substitutes. Duplicate deliveries return
the same operation; payload conflicts fail loudly; stale leases cannot
publish; cancel/publish races resolve to exactly one terminal state; and a
"restarted process" (new store instance) still gets idempotent accepts.
"""

import threading

import pytest

from semantic_service.contracts import (
    ApplyRequest,
    ChunkSnapshot,
    DocumentRevision,
    IndexConfig,
    Operation,
    ScopeKey,
)
from semantic_service.operations import (
    InvalidTransitionError,
    OperationConflictError,
    OperationNotFoundError,
    OperationStore,
    RequestTooLargeError,
)

SCOPE = ScopeKey(tenant_id=1, kb_id="kb-ops")


def make_apply_request(idempotency_key="idem-1", payload_hash="hash-1", revision=1):
    return ApplyRequest(
        document=DocumentRevision(
            scope=SCOPE, document_id="doc-1", revision=revision,
            content_hash="content-hash", deleted=False),
        chunks=(ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="ch1"),),
        config=IndexConfig(
            config_digest="digest", engine_version="semantica-0.6.8",
            model_profile_ref="profile", prompt_version="p1",
            rule_set_version="r1", schema_version="s1"),
        idempotency_key=idempotency_key,
        payload_hash=payload_hash,
    )


def test_repeated_apply_returns_same_operation(operation_store, apply_request):
    first = operation_store.accept(apply_request)
    second = operation_store.accept(apply_request)
    assert first.operation_id == second.operation_id


def test_new_store_instance_keeps_idempotency(operation_store, apply_request, pg_dsn):
    # "Process restart": a fresh store over the same database returns the
    # same operation for the same idempotency key.
    first = operation_store.accept(apply_request)
    restarted = OperationStore(pg_dsn)
    second = restarted.accept(apply_request)
    assert first.operation_id == second.operation_id
    assert second.state == first.state


def test_same_key_different_payload_is_conflict(operation_store, apply_request):
    operation_store.accept(apply_request)
    tampered = make_apply_request(payload_hash="hash-OTHER")
    with pytest.raises(OperationConflictError):
        operation_store.accept(tampered)


def test_claim_assigns_lease_and_moves_to_running(operation_store, apply_request):
    operation_store.accept(apply_request)
    claimed = operation_store.claim(worker_id="worker-1", lease_seconds=30)
    assert claimed is not None
    assert claimed.state == "running"
    assert claimed.lease_token >= 1


def test_concurrent_claim_single_winner(operation_store, apply_request, pg_dsn):
    operation_store.accept(apply_request)
    winners = []
    barrier = threading.Barrier(2)

    def contender(worker_id):
        store = OperationStore(pg_dsn)
        barrier.wait()
        op = store.claim(worker_id=worker_id, lease_seconds=30)
        if op is not None:
            winners.append(worker_id)

    threads = [threading.Thread(target=contender, args=(f"worker-{i}",)) for i in range(2)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
    assert len(winners) == 1, f"exactly one claimer must win, got {winners}"


def test_stale_lease_cannot_publish(operation_store, claimed_operation):
    op = claimed_operation
    assert not operation_store.transition(op.operation_id, op.lease_token - 1,
                                          "staged", "publishing")


def test_expired_lease_taken_over_and_old_worker_blocked(operation_store, apply_request):
    operation_store.accept(apply_request)
    first = operation_store.claim(worker_id="worker-1", lease_seconds=30)
    # Simulate worker death: force the lease to expire behind the store's back.
    operation_store.expire_lease_for_test(first.operation_id)
    second = operation_store.claim(worker_id="worker-2", lease_seconds=30)
    assert second is not None
    assert second.lease_token == first.lease_token + 1
    # The dead worker's stale token can no longer move the operation.
    assert not operation_store.transition(first.operation_id, first.lease_token,
                                          "running", "staged")
    # The new owner can.
    assert operation_store.transition(second.operation_id, second.lease_token,
                                      "running", "staged")


def test_renewal_requires_matching_token(operation_store, claimed_operation):
    op = claimed_operation
    assert operation_store.renew(op.operation_id, op.lease_token, lease_seconds=30)
    assert not operation_store.renew(op.operation_id, op.lease_token + 5, lease_seconds=30)


def test_full_lifecycle_publishes_once(operation_store, apply_request):
    operation_store.accept(apply_request)
    op = operation_store.claim(worker_id="worker-1", lease_seconds=30)
    assert operation_store.transition(op.operation_id, op.lease_token, "running", "staged")
    assert operation_store.transition(op.operation_id, op.lease_token, "staged", "publishing")
    assert operation_store.transition(op.operation_id, op.lease_token, "publishing", "succeeded")
    final = operation_store.get(SCOPE, op.operation_id)
    assert final.state == "succeeded"
    # Terminal states are irreversible.
    assert not operation_store.transition(op.operation_id, op.lease_token, "succeeded", "running")
    assert not operation_store.transition(op.operation_id, op.lease_token, "succeeded", "cancelled")


def test_cancel_before_terminal_releases_lease(operation_store, claimed_operation):
    op = claimed_operation
    cancelled = operation_store.cancel(SCOPE, op.operation_id)
    assert cancelled.state == "cancelled"
    assert not operation_store.transition(op.operation_id, op.lease_token,
                                          "running", "staged")
    # Cancel is idempotent for already-cancelled operations.
    again = operation_store.cancel(SCOPE, op.operation_id)
    assert again.state == "cancelled"


def test_cancel_after_success_is_failed_precondition(operation_store, apply_request):
    operation_store.accept(apply_request)
    op = operation_store.claim(worker_id="worker-1", lease_seconds=30)
    operation_store.transition(op.operation_id, op.lease_token, "running", "staged")
    operation_store.transition(op.operation_id, op.lease_token, "staged", "publishing")
    operation_store.transition(op.operation_id, op.lease_token, "publishing", "succeeded")
    with pytest.raises(Exception, match="FAILED_PRECONDITION"):
        operation_store.cancel(SCOPE, op.operation_id)


def test_cancel_publish_race_exactly_one_terminal(operation_store, apply_request):
    """Cancel vs publish on a staged operation: whichever CAS wins first
    decides the terminal state; the loser observes a failed transition or a
    FAILED_PRECONDITION cancel - never two terminal states."""
    operation_store.accept(apply_request)
    op = operation_store.claim(worker_id="worker-1", lease_seconds=30)
    operation_store.transition(op.operation_id, op.lease_token, "running", "staged")

    # Path A: cancel wins first, publish path loses.
    cancelled = operation_store.cancel(SCOPE, op.operation_id)
    assert cancelled.state == "cancelled"
    assert not operation_store.transition(op.operation_id, op.lease_token,
                                          "staged", "publishing")

    # Path B (fresh operation): publish wins first, cancel raises.
    request_b = make_apply_request(idempotency_key="idem-race", payload_hash="hash-race")
    operation_store.accept(request_b)
    op_b = operation_store.claim(worker_id="worker-1", lease_seconds=30)
    operation_store.transition(op_b.operation_id, op_b.lease_token, "running", "staged")
    operation_store.transition(op_b.operation_id, op_b.lease_token, "staged", "publishing")
    operation_store.transition(op_b.operation_id, op_b.lease_token, "publishing", "succeeded")
    with pytest.raises(Exception, match="FAILED_PRECONDITION"):
        operation_store.cancel(SCOPE, op_b.operation_id)


def test_renew_on_unclaimed_operation_rejected(operation_store, apply_request):
    # An accepted-but-unclaimed row must not be "renewed" into a phantom
    # lease that blocks real workers from claiming it.
    created = operation_store.accept(apply_request)
    assert not operation_store.renew(created.operation_id, 0, lease_seconds=30)
    claimed = operation_store.claim(worker_id="w", lease_seconds=30)
    assert claimed is not None


def test_publishing_to_superseded_is_allowed(operation_store, apply_request):
    # Spec section 6: losing the active-pointer CAS during publish must be
    # expressible as superseded (the I03 publisher path).
    operation_store.accept(apply_request)
    op = operation_store.claim(worker_id="w", lease_seconds=30)
    operation_store.transition(op.operation_id, op.lease_token, "running", "staged")
    operation_store.transition(op.operation_id, op.lease_token, "staged", "publishing")
    assert operation_store.transition(op.operation_id, op.lease_token,
                                      "publishing", "superseded")


def test_unclaimed_accepted_operation_can_be_superseded(operation_store, apply_request):
    # accepted -> superseded bypasses claims (lease_token stays 0 until the
    # first claim); the CAS must honor the NULL-lease unclaimed state.
    created = operation_store.accept(apply_request)
    assert operation_store.transition(created.operation_id, 0,
                                      "accepted", "superseded")


def test_mark_result_is_atomic_single_step(operation_store, apply_request):
    # Terminal failure records its error code AND releases the lease in the
    # same statement - no window where a terminal op keeps a dangling lease.
    operation_store.accept(apply_request)
    op = operation_store.claim(worker_id="w", lease_seconds=30)
    assert operation_store.mark_result(op.operation_id, op.lease_token,
                                       "running", "failed", error_code="E_EXTRACT")
    final = operation_store.get(SCOPE, op.operation_id)
    assert final.state == "failed"
    assert final.error_code == "E_EXTRACT"
    assert operation_store.lease_owner_for_test(op.operation_id) is None
    assert operation_store.lease_until_for_test(op.operation_id) is None


def test_unknown_operation_raises_not_found(operation_store):
    with pytest.raises(OperationNotFoundError):
        operation_store.get(SCOPE, "op-missing")
    with pytest.raises(OperationNotFoundError):
        operation_store.cancel(SCOPE, "op-missing")
    with pytest.raises(OperationNotFoundError):
        operation_store.load_request("op-missing")


def test_claim_on_empty_store_returns_none(operation_store):
    assert operation_store.claim(worker_id="w", lease_seconds=30) is None


def test_cancel_scope_mismatch_is_not_found(operation_store, apply_request):
    created = operation_store.accept(apply_request)
    other_scope = ScopeKey(tenant_id=99, kb_id="kb-other")
    with pytest.raises(OperationNotFoundError):
        operation_store.cancel(other_scope, created.operation_id)


def test_illegal_transition_raises_precisely(operation_store, claimed_operation):
    with pytest.raises(InvalidTransitionError):
        operation_store.transition(claimed_operation.operation_id,
                                   claimed_operation.lease_token,
                                   "running", "succeeded")


def test_oversized_request_rejected(operation_store):
    big = make_apply_request(idempotency_key="idem-big", payload_hash="hash-big")
    huge_chunk = ChunkSnapshot(chunk_id="c1", text="甲" * (5 * 1024 * 1024), content_hash="ch1")
    big = type(big)(document=big.document, chunks=(huge_chunk,), manifest_ref=big.manifest_ref,
                    config=big.config, idempotency_key=big.idempotency_key,
                    payload_hash=big.payload_hash)
    with pytest.raises(RequestTooLargeError):
        operation_store.accept(big)


def test_get_returns_scope_bound_operation(operation_store, apply_request):
    created = operation_store.accept(apply_request)
    fetched = operation_store.get(SCOPE, created.operation_id)
    assert fetched.operation_id == created.operation_id
    assert fetched.document_id == "doc-1"
    assert fetched.revision == 1
