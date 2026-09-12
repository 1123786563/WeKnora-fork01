# tRPC recovery acceptance record

This document is the release gate for the durable tRPC run path. A passing unit
or checkpoint test does not prove process recovery: the crash cases below must
start a real graph provider, kill that process at a provider-owned barrier, and
reopen the same durable database in a new process.

## Current evidence

Recorded 2026-09-12 on macOS (go1.26.3 darwin/arm64), branch
`codex/trpc-recovery-r2`. The production chain is now wired end to end: the
session service registers the real graph executor
(`ExecuteDurableRun`), the container injects it into the durable worker with
a boot-time gate, the recovery hook reconciles sandbox-bound runs through the
provider sandbox-list query (`SessionBoundManager.ObserveInstance`), and the
graph persists `run_started`, `attempt_replaced`, `tool_dispatched`,
`tool_result`, `run_failed` and `run_completed` events through the
fenced event store.

The SIGKILL matrix runs against `internal/agent/recoverytest/provider`, a
provider binary that owns the production stack (migrated database, durable
worker with fenced leases, SDK graph, repository checkpoint saver, tool
journal, durable decision parking, finalize transaction); only the chat model
and the external side-effect endpoint are deterministic doubles.

| Check | Command / setup | Result | Evidence |
|---|---|---|---|
| admission gate | `GOWORK=off go test ./internal/agent/recoverytest -run TestRecoveryAdmissionGate -count=1` | PASS | disabled admission is false; worker-only mode is drain-only; enabled admission is true |
| inconsistent config | `GOWORK=off go test ./internal/agent/recoverytest -run TestRecoveryAdmissionGateRejectsInconsistentConfig -count=1` | PASS | `AdmissionEnabled=true` with `Enabled=false` is rejected before runtime construction |
| SIGKILL matrix (SQLite) | `GOWORK=off go test ./internal/agent/recoverytest -run TestCrashMatrixSQLite -count=1 -v` | PASS 8/8 | after_admission, after_plan_before_dispatch, after_result_before_checkpoint, after_finalize each: exactly 1 external call, final status succeeded, 1 completed assistant row, 0 lost events; after_side_effect_before_result and waiting_user: 1 external call, parked at waiting_user; unknown_result_user_retry: explicit retry alone raises the external count to 2; idempotent_redelivery: redelivery deduplicated, count stays 1 |
| SIGKILL matrix (PostgreSQL) | `docker run postgres:16-alpine` on 55432; `TRPC_RECOVERY_PG_DSN=postgres://... go test ./internal/agent/recoverytest -run TestCrashMatrixPostgreSQL -count=1 -v` | PASS 7/7 (2026-09-12) | per-case schemas with hashed names, versioned migrations, real fenced leases/claims/epochs on PostgreSQL; after_admission/after_plan/after_result/after_finalize succeed with 1 external call and 0 lost events; after_side_effect parks at waiting_user; unknown_result_user_retry reaches 2 calls only after the explicit retry decision; idempotent_redelivery stays at 1. Durable JSON columns are TEXT (byte-exact round-trip; empty retry results must not trip JSON parsing) |
| PostgreSQL repository suite | `TRPC_TEST_POSTGRES_DSN=... go test ./internal/application/repository -count=1` | PASS | full suite incl. TestAgentRunPostgres (admission idempotency, guards, rollback, lease/checkpoint, concurrent claim, reopen+migrations) on PostgreSQL 16 |
| two-worker contention | `go test ./internal/agent/recoverytest -run TestTwoWorkerContentionSQLite -count=1` (SQLite) and `...PostgreSQL` with `TRPC_RECOVERY_PG_DSN` | PASS both (2026-09-12) | stale worker claims, lets its lease expire, then keeps attempting fenced writes while a takeover process claims with a higher epoch and completes: every post-expiry write rejected, external side effect exactly once, no durable-state pollution |
| deadline budget | `go test ./internal/application/service -run TestWorkerFailsRunPastPersistedDeadline -count=1` | PASS | a run past its persisted deadline fails with deadline_exceeded before executing; the execution context is capped at the deadline (second test), so the budget survives restarts and a slow graph cannot outlive it |
| cancel lifecycle | `go test ./internal/application/service -run TestCancelReleasesSessionSlot -count=1` | PASS | cancel marks canceled, releases the session slot (next run admits immediately), canceled run never claimable — real migrated store |
| schema incompatibility | `go test ./internal/application/service -run TestExecuteDurableRunRejectsIncompatibleCheckpoint -count=1` | PASS | foreign graph_version envelope in the current namespace fails resume with the explicit error before any execution; foreign namespaces stay isolated |
| permission revocation | `go test ./internal/application/repository -run TestAgentRunToolRejectsRevokedSessionAndCanceledRun -count=1` | PASS | dispatch is rejected for revoked sessions and canceled runs at the journal boundary |
| crash after tool result | `GOWORK=off go test ./internal/agent/recoverytest -run TestCrashAfterToolResult -count=1` | SKIPPED | superseded by the matrix subtest above when run without `TRPC_RECOVERY_GRAPH_PROVIDER`; the env-gated variant remains for CI |
| executor end to end | `GOWORK=off go test ./internal/application/service -run TestExecuteDurableRun -count=1` | PASS | fresh run completes through admission snapshot → capability rebuild → graph → finalize transaction; superseded fence rejected with ErrLeaseLost |
| worker wait mapping | `GOWORK=off go test ./internal/application/service -run TestWorkerParksWaitClass -count=1` | PASS | unknown tool outcomes park durably at waiting_user/tool_outcome_unknown instead of terminating |
| race | `GOWORK=off go test -race ./internal/agent/trpc ./internal/agent/runtime ./internal/application/repository -count=1` | PASS | recorded 2026-09-12 |

Defects found and fixed by the matrix (recorded for audit):

1. A mid-node SIGKILL leaves a pending branch write in the latest checkpoint.
   The SDK executor only plans the resume frontier from `StateKeyNextNodes`
   when no pending writes remain, so every resume returned nil without
   executing a node or finalizing, and the worker marked the run succeeded
   with an empty answer. The repository checkpoint saver now materializes
   branch-marker pending writes at load time as frontier re-execution (nodes
   are idempotent against the tool journal); value writes keep round-tripping
   and the stored record keeps everything for audit.
2. The graph schema seeds the state channel with a zero-valued state; the
   first SDK checkpoint therefore carries a version-0 seed that strict
   decoding rejected. Seeds are now accepted only when no execution-owned key
   is present in the JSON.

## Remaining matrix rows (not yet passed — do not treat as done)

- API reconnect through the run-events SSE endpoint against a live
  recovery worker, decision-conflict HTTP envelope, and session deletion
  racing an active run (unit and repository layers cover the pieces);
- sandbox alive/lost/destroyed fixtures (the hook queries the provider
  sandbox list, but the three fixture states are not yet asserted end to end);
- the external-action outbox and event retention watermark (Task 11
  leftovers), and the after-mode steering follow-up admission path;
- frontend browser-level verification of the two engine types (unit,
  type-check and build pass; no live browser session in this environment).

Known migration limitation: a SQLite database that applied the intermediate
branch revision of migration 000014 cannot upgrade through 000016 (the rebuild
references columns the intermediate 000014 did not create). Upgrade from the
pre-feature baseline (000013) is verified data-preserving; the feature was
never released, so no released database can hold the intermediate state.

## Provider protocol

Set `TRPC_RECOVERY_GRAPH_PROVIDER` to an executable that owns the real graph,
saver, and migrated database. The harness invokes it once with:

```text
--recovery-case <barrier> --recovery-db <file> --recovery-barrier <file> --recovery-report <file>
```

The provider must create only test-namespace resources, touch the barrier after
the requested durable point, and keep running. The parent sends SIGKILL, then
invokes the same executable with `--recovery-resume <barrier>` and the same
arguments. Resume must print or write a JSON `CrashReport` with
`external_calls`, `final_status`, `assistant_rows`, and `lost_events`.

## Gate semantics

`AgentRecoveryAdmissionEnabled` is true only when both the worker and admission
flags are enabled. `Enabled=true, AdmissionEnabled=false` is the controlled
drain mode: existing queued or leased runs may finish or be reclaimed, while
new tRPC work is closed. `AdmissionEnabled=true, Enabled=false` is rejected at
startup because it would admit work without a recovery worker. A disabled
worker must never silently fall back to builtin execution for a persisted tRPC
run.

Until every required matrix row has fresh command output and a provider report,
the feature remains unavailable for rollout. The single-process SQLite matrix
above is now green; the remaining rows keep the feature closed.
