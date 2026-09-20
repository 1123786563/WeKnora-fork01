# Semantica I01 Persistent Operations Implementation Plan

> **For agentic workers:** REQUIRED SKILL: Use superpowers:dispatching-parallel-agents for the independent I01/I02 workstreams; this task's files are owned only by the I01 worker. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist idempotent semantic operations in a service-owned PostgreSQL schema and provide restart-safe worker claiming, lease renewal, fencing, and cancellation primitives.

**Architecture:** Python owns semantic operation records and worker leases in its own PostgreSQL schema; Go remains authoritative for tenant/KB/document identity and passes the existing C01 `ApplyRequest`. The store durably retains the deterministic C01 request protobuf while work is nonterminal, exposes a lease-fenced worker seam, and clears request bytes at terminal state. I01 does not run graph/model work or wire partially implemented RPC methods into the public server.

**Tech Stack:** Python 3.12, Psycopg 3.3.6 (`psycopg[binary]`), PostgreSQL 17 test service, C01 frozen contracts/protobuf. Psycopg's stable 3.3.6 release is recorded by [PyPI](https://pypi.org/project/psycopg/3.3.6/); the sync connection API matches the existing synchronous gRPC thread-pool server.

**Spec:** `docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md` §§3, 6–7; `docs/adr/0002-semantica-independent-service.md`; `docs/plans/2026-09-20-semantica-rebaseline.md`; I01 in `docs/plans/2026-09-11-semantica-02-indexing.md`.

## Global Constraints

- The semantic service owns semantic `operation` state; Go retains business resource, tenant, KB, document, ACL, and revision authority.
- Operation identity is scoped to `(tenant_id, kb_id)`; repeated delivery returns the existing operation; same idempotency identity with a different supplied payload hash is a conflict.
- The service's persisted phases are `accepted`, `running`, `staged`, `publishing`, `succeeded`, `failed`, `cancelled`, and `superseded`; terminal state is irreversible.
- Every claim/reclaim increments a monotonic fencing token. A worker with stale token, wrong owner, or expired lease cannot renew, advance stage, publish, or finish.
- Existing C01 wire `OperationState` is frozen to `pending/running/succeeded/failed/cancelled`; no proto/C01 contract change is allowed in I01. Projection: `accepted → pending`; `running/staged/publishing → running` while preserving exact phase in `stage`; `superseded → failed` with `stage="superseded"` and `error_code="operation_superseded"`.
- Persist the deterministic serialized C01 `ApplyRequest` only while an operation is nonterminal, so another worker/process can resume; clear request bytes atomically on terminal transition. Never log request bytes, chunks, or credentials.
- PostgreSQL tests use `SEMANTIC_TEST_POSTGRES_DSN` and a per-test random schema. Missing DSN is a test failure, never a skip or a successful mock substitute.
- This task does not write semantic graph/vector data, call a model/provider, change authorization, serve successful business RPCs, change C01 protobuf/contracts, or claim production readiness. The gRPC health status remains `NOT_SERVING` while downstream capability work is incomplete.

## Review Focus

- Same scope/idempotency key/same hash, duplicate delivery after store recreation: return one operation, not two.
- Same key or document-revision-config identity with a different payload hash: fail explicitly; do not return another payload's operation.
- Simultaneous claims over real independent PostgreSQL connections: exactly one current lease; expired lease takeover increments the fence.
- Stale worker actions after reclaim/cancel: every renew/transition fails, even if the old worker resumes.
- Cancel vs `publishing → succeeded`: one durable terminal state wins; cancelling a succeeded operation returns a failed-precondition error; terminal operation cannot be changed.

---

### Task 1: Implement the PostgreSQL operation store and worker lease seam

**Files:**
- Create: `semantic/migrations/001_operations.sql`
- Create: `semantic/semantic_service/operations.py`
- Create: `semantic/semantic_service/worker.py`
- Modify: `semantic/pyproject.toml` (pin `psycopg[binary]==3.3.6`)
- Modify: `semantic/uv.lock`
- Modify: `semantic/tests/conftest.py` (add mandatory isolated PostgreSQL `operation_store` and `operation_store_factory` fixtures plus a complete `apply_request` fixture)
- Create: `semantic/tests/test_operations.py`

**Interfaces:**
- `PostgresOperationStore(dsn: str, schema: str = "semantic_service")` owns a connection per call and the service schema; `migrate()` is safe on every startup and serializes concurrent migration attempts by advisory lock plus a service `schema_migrations` version row.
- `accept(request: ApplyRequest) -> Operation` deduplicates by both `(scope, idempotency_key)` and `(scope, document_id, revision, config_digest)`; a matching payload hash returns the existing operation, mismatched hash/identity raises `OperationPayloadConflict`.
- `get(scope: ScopeKey, operation_id: str) -> Operation` is scope-bound; missing/wrong-scope IDs raise the same `OperationNotFound` result. `cancel` has the same scope-bound not-found behavior.
- Internal `OperationPhase` contains the eight persisted phases from Global Constraints; C01 DTO conversion applies the projection described above.
- `LeasedOperation` contains `operation: Operation`, the decoded durable `request: ApplyRequest`, `worker_id`, and `lease_token`.
- `claim(worker_id: str, lease_seconds: int) -> LeasedOperation | None` atomically selects one accepted or expired nonterminal row with `FOR UPDATE SKIP LOCKED`, updates owner/deadline, and increments the fence before returning.
- `renew(operation_id: str, worker_id: str, lease_token: int, lease_seconds: int) -> bool` and `transition(operation_id: str, worker_id: str, lease_token: int, expected: OperationPhase, next: OperationPhase, stage: str, error_code: str | None = None, result_generation: str | None = None) -> bool` require a current unexpired lease and allowed state edge; transition persists stage, error, and result metadata.
- `cancel(scope: ScopeKey, operation_id: str) -> Operation` is compare-and-set against current nonterminal state, releases/invalidate the lease, clears request bytes, and rejects succeeded operations with `OperationFailedPrecondition`.
- `OperationWorker(store: PostgresOperationStore)` exposes `claim_next`, `renew`, and `transition` for a later I03 processor; it does not invent an indexing handler in I01.
- Allowed internal transitions are `accepted→running`, `running→staged`, `staged→publishing`, `publishing→succeeded`; `accepted/running/staged/publishing` may transition to `failed/cancelled/superseded`; no transition leaves a terminal phase. Reclaim renews a lease without changing the phase.

- [ ] **Step 1: Write failing real-PostgreSQL behavior tests**

```python
def test_repeated_apply_returns_same_operation(operation_store, apply_request):
    first = operation_store.accept(apply_request)
    second = operation_store.accept(apply_request)
    assert second.operation_id == first.operation_id
    assert operation_store.get(apply_request.document.scope, first.operation_id) == first

def test_reused_idempotency_identity_with_different_hash_conflicts(operation_store, apply_request):
    operation_store.accept(apply_request)
    changed = dataclasses.replace(apply_request, payload_hash="different-hash")
    with pytest.raises(OperationPayloadConflict):
        operation_store.accept(changed)

def test_same_key_with_different_document_identity_conflicts(operation_store, apply_request):
    operation_store.accept(apply_request)
    changed = dataclasses.replace(
        apply_request,
        document=dataclasses.replace(apply_request.document, document_id="other-document"),
    )
    with pytest.raises(OperationPayloadConflict):
        operation_store.accept(changed)

def test_same_document_revision_and_config_deduplicates_across_request_retry_key(operation_store, apply_request):
    first = operation_store.accept(apply_request)
    retry = dataclasses.replace(apply_request, idempotency_key="retry-key")
    second = operation_store.accept(retry)
    assert second.operation_id == first.operation_id

def test_document_identity_unique_key_with_different_payload_hash_conflicts(operation_store, apply_request):
    first = dataclasses.replace(apply_request, idempotency_key="identity-a", payload_hash="hash-a")
    operation_store.accept(first)
    changed = dataclasses.replace(first, idempotency_key="identity-b", payload_hash="hash-b")
    with pytest.raises(OperationPayloadConflict):
        operation_store.accept(changed)

def test_get_and_cancel_do_not_cross_scope(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    wrong_scopes = (
        ScopeKey(apply_request.document.scope.tenant_id + 1, apply_request.document.scope.kb_id),
        ScopeKey(apply_request.document.scope.tenant_id, "other-kb"),
    )
    for wrong_scope in wrong_scopes:
        with pytest.raises(OperationNotFound):
            operation_store.get(wrong_scope, operation.operation_id)
        with pytest.raises(OperationNotFound):
            operation_store.cancel(wrong_scope, operation.operation_id)
    assert operation_store.get(apply_request.document.scope, operation.operation_id) == operation

def test_two_postgres_connections_cannot_claim_the_same_live_lease(operation_store, apply_request):
    operation_store.accept(apply_request)
    with ThreadPoolExecutor(max_workers=2) as pool:
        claims = list(pool.map(lambda worker: operation_store.claim(worker, 30), ("w1", "w2")))
    assert sum(claim is not None for claim in claims) == 1

def test_expired_claim_gets_new_fence_and_old_worker_cannot_transition(operation_store, apply_request):
    operation_store.accept(apply_request)
    first = operation_store.claim("worker-old", 30)
    assert first is not None
    expire_lease_at_database_clock(operation_store, first.operation.operation_id)
    second = operation_store.claim("worker-new", 30)
    assert second is not None and second.lease_token == first.lease_token + 1
    assert not operation_store.renew(first.operation.operation_id, "worker-old", first.lease_token, 30)
    assert not operation_store.transition(first.operation.operation_id, "worker-old", first.lease_token,
                                          OperationPhase.RUNNING, OperationPhase.STAGED, "staged")
    assert not operation_store.renew(second.operation.operation_id, "wrong-worker", second.lease_token, 30)
    assert not operation_store.renew(second.operation.operation_id, "worker-new", second.lease_token - 1, 30)
    assert not operation_store.transition(second.operation.operation_id, "wrong-worker", second.lease_token,
                                          OperationPhase.RUNNING, OperationPhase.STAGED, "staged")
    assert not operation_store.transition(second.operation.operation_id, "worker-new", second.lease_token - 1,
                                          OperationPhase.RUNNING, OperationPhase.STAGED, "staged")

def test_cancel_and_publish_race_has_one_terminal_winner(operation_store, apply_request):
    operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert operation_store.transition(claim.operation.operation_id, "worker", claim.lease_token,
                                      OperationPhase.RUNNING, OperationPhase.STAGED, "staged")
    assert operation_store.transition(claim.operation.operation_id, "worker", claim.lease_token,
                                      OperationPhase.STAGED, OperationPhase.PUBLISHING, "publishing")
    barrier = threading.Barrier(2)
    def cancel():
        barrier.wait()
        try:
            operation_store.cancel(apply_request.document.scope, claim.operation.operation_id)
            return "cancelled"
        except OperationFailedPrecondition:
            return "already-succeeded"
    def publish():
        barrier.wait()
        won = operation_store.transition(claim.operation.operation_id, "worker", claim.lease_token,
                                        OperationPhase.PUBLISHING, OperationPhase.SUCCEEDED, "succeeded",
                                        result_generation="gen-1")
        return "succeeded" if won else "lost"
    with ThreadPoolExecutor(max_workers=2) as pool:
        cancel_future = pool.submit(cancel)
        publish_future = pool.submit(publish)
        outcomes = {cancel_future.result(), publish_future.result()}
    final = operation_store.get(apply_request.document.scope, claim.operation.operation_id)
    assert final.state in {"succeeded", "cancelled"}
    assert len(outcomes & {"cancelled", "succeeded"}) == 1
    if final.state == "cancelled":
        assert not operation_store.renew(claim.operation.operation_id, "worker", claim.lease_token, 30)
        assert not operation_store.transition(claim.operation.operation_id, "worker", claim.lease_token,
                                              OperationPhase.PUBLISHING, OperationPhase.SUCCEEDED, "succeeded")

def test_accept_survives_store_recreation(operation_store_factory, apply_request):
    first_store = operation_store_factory()
    first = first_store.accept(apply_request)
    second_store = operation_store_factory()  # fresh object and new database connections
    second = second_store.accept(apply_request)
    assert second.operation_id == first.operation_id

def test_succeeded_operation_cannot_be_cancelled(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert claim.request == apply_request
    assert request_bytes_for_test(operation_store, operation.operation_id) == apply_request_to_wire(apply_request).SerializeToString(deterministic=True)
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token,
                                      OperationPhase.RUNNING, OperationPhase.STAGED, "staged")
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token,
                                      OperationPhase.STAGED, OperationPhase.PUBLISHING, "publishing")
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token,
                                      OperationPhase.PUBLISHING, OperationPhase.SUCCEEDED, "succeeded",
                                      result_generation="generation-1")
    with pytest.raises(OperationFailedPrecondition):
        operation_store.cancel(apply_request.document.scope, operation.operation_id)
    operation = operation_store.get(apply_request.document.scope, operation.operation_id)
    assert operation.result_generation == "generation-1"
    assert operation.error_code is None
    assert request_bytes_for_test(operation_store, operation.operation_id) is None

@pytest.mark.parametrize("terminal_phase", (OperationPhase.FAILED, OperationPhase.SUPERSEDED, OperationPhase.CANCELLED))
def test_terminal_operation_clears_durable_request_bytes(operation_store, apply_request, terminal_phase):
    operation = operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert request_bytes_for_test(operation_store, operation.operation_id) is not None
    if terminal_phase == OperationPhase.CANCELLED:
        operation_store.cancel(apply_request.document.scope, operation.operation_id)
    else:
        assert operation_store.transition(operation.operation_id, "worker", claim.lease_token,
                                          OperationPhase.RUNNING, terminal_phase, terminal_phase.value,
                                          error_code="build-failed" if terminal_phase == OperationPhase.FAILED else None)
    assert request_bytes_for_test(operation_store, operation.operation_id) is None

def test_failed_transition_persists_error_and_stage(operation_store, apply_request):
    operation = operation_store.accept(apply_request)
    claim = operation_store.claim("worker", 30)
    assert claim is not None
    assert operation_store.transition(operation.operation_id, "worker", claim.lease_token,
                                      OperationPhase.RUNNING, OperationPhase.FAILED, "build", error_code="provider-unavailable")
    failed = operation_store.get(apply_request.document.scope, operation.operation_id)
    assert failed.state == "failed" and failed.stage == "build"
    assert failed.error_code == "provider-unavailable"

def test_store_migration_is_idempotent_across_restarts(operation_store_factory):
    first_store = operation_store_factory()
    first_store.migrate()
    operation_store_factory().migrate()

def test_exhausted_lease_fence_fails_without_exceeding_uint64(operation_store_factory, apply_request):
    store = operation_store_factory()
    store.accept(apply_request)
    set_lease_token_for_test(store, apply_request, UINT64_MAX)
    with pytest.raises(OperationFailedPrecondition):
        store.claim("worker", 30)
```

The test module imports `dataclasses`, `threading`, `ThreadPoolExecutor`, `pytest`, `psycopg`, `psycopg.sql`, C01 `ApplyRequest`/`ScopeKey`/`apply_request_to_wire`/`UINT64_MAX`, and I01 store/phase/error types. `expire_lease_at_database_clock`, `request_bytes_for_test`, and `set_lease_token_for_test` are test-only helpers using a new psycopg connection plus `psycopg.sql.Identifier(store.schema)`; they never add test-only production APIs. `operation_store_factory` creates a new store object using the same test DSN/schema. The store fixture opens real PostgreSQL, creates a unique schema, applies the production I01 migration, and drops only that schema in cleanup. It fails with a clear `SEMANTIC_TEST_POSTGRES_DSN is required` message if the DSN is absent.

```python
def request_bytes_for_test(store, operation_id):
    with psycopg.connect(store.dsn) as connection:
        row = connection.execute(
            sql.SQL("SELECT request_bytes FROM {}.operations WHERE operation_id = %s").format(sql.Identifier(store.schema)),
            (operation_id,),
        ).fetchone()
    return row[0]

def expire_lease_at_database_clock(store, operation_id):
    with psycopg.connect(store.dsn) as connection:
        connection.execute(
            sql.SQL("UPDATE {}.operations SET lease_until = clock_timestamp() - interval '1 second' WHERE operation_id = %s").format(sql.Identifier(store.schema)),
            (operation_id,),
        )

def set_lease_token_for_test(store, request, lease_token):
    operation = store.accept(request)
    with psycopg.connect(store.dsn) as connection:
        connection.execute(
            sql.SQL("UPDATE {}.operations SET lease_token = %s WHERE operation_id = %s").format(sql.Identifier(store.schema)),
            (lease_token, operation.operation_id),
        )
```

- [ ] **Step 2: Run RED**

Run: `uv run --locked --project semantic python -m pytest semantic/tests/test_operations.py -q`

Expected: fail because `semantic_service.operations` and the store fixture do not exist.

- [ ] **Step 3: Implement migration, store, and C01 projection**

Use one transaction per accept/claim/renew/transition/cancel; create the service schema under the migration account; record migration version 1; keep the database row's internal `phase` and mutable `stage` separate from C01 `Operation.state`; store exact deterministic protobuf request bytes only while nonterminal. Use `NUMERIC(20,0)` for `tenant_id`, revision, and lease token with an upper-bound check at `18446744073709551615` so PostgreSQL can represent but never exceed the full uint64 contract.

- [ ] **Step 4: Run GREEN and all service tests against the disposable PostgreSQL DSN**

Run: `uv run --locked --project semantic python -m pytest semantic/tests/test_operations.py -q`

Expected: all real-PostgreSQL operation tests pass, including concurrency, fencing, cancel/publish, and store recreation/reclaim. Then run `uv run --locked --project semantic python -m pytest semantic/tests -q` with the same DSN; existing auth/contract tests continue to pass and no test silently skips the persistence fixture.

- [ ] **Step 5: Commit Task I01**

Commit `feat(semantic): i01 persistent operations and worker leases` with only I01 code/tests/dependency changes.

---

## Final I01 Gate

- [ ] Request an independent read-only review of the complete I01 range; fix Critical/Important findings with RED→GREEN behavior tests.
- [ ] Record test DSN evidence layer as actual-runtime PostgreSQL; do not claim graph storage, model, or production readiness.
