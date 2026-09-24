# R5 Effect Task 2b — Task 1 report

**Checkpoint:** `craft107-r5-effect-task2b-task1-fix1`  
**Plan:** [R5 Effect Task 2b Transition Fence Plan](2026-09-24-craft-107-runview-r5-effect-task2b-plan.md)  
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Base HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34`  
**Commit:** none (task explicitly uses an uncommitted checkpoint).

## Scope and implementation

Added the typed `ErrCraftRunViewEffectUnresolved` result and one transaction-local guard that reads the durable pending/unknown RunView intent while the relevant Run lock is held. New claims and Run transitions remain serialized by the existing Run row lock. Admission uses the session row lock: it rejects a new generation if *any* Run for the same tenant/session still has a pending or unknown RunView effect. This is paired with `BeginEffect` taking the same session lock before creating an intent, so a claim and slot transfer cannot both commit based on stale reads.

The guards cover these writer/status/slot paths:

- `AgentRunStore.Admit`: idempotent replay is left read-only; only creation/reservation of a new Run checks unresolved effects from earlier Runs in the session, under the session lock.
- `CancelRun`, `CancelRunOwnedAtRevision`, and the per-Run path in `DeleteSessionRuns`: `cancelRunTx` takes the Run transition lock and checks before idempotent terminal handling, charge-start recovery, status/lease changes, cancellation events, or slot release. `DeleteSessionRuns` retains its pre-existing durable tombstone, but its Run cleanup transaction aborts if an effect is unresolved; no Run status, lease, or slot clear commits.
- `SetStatus`: checks before waiting-user pause or succeeded/failed transitions. `transitionCraftBudgetPauseTx` and `resolveCraftChargePendingTransition` also check before status/slot changes.
- `ClaimDriver`: checks under the Run lock before charge-start reconciliation, lease recovery, epoch increment, or ownership transfer.
- `Finalize`: checks before assistant message completion, succeeded status, `run_completed`, slot clear, and the R4 terminal capture trigger. After the effect is resolved, finalization remains idempotent and the capture enqueue occurs once.
- `ApplyDecision`: checks before a new decision changes a waiting Run, tool state, or lease. An exact previously stored decision replay remains read-only and is allowed.

The inspection also covered `Renew`, `SaveCheckpoint`, and `AppendEvent`: these renew the same lease or append checkpoint/event data without revoking writer ownership, changing Run status/epoch, or transferring the session slot, so they do not introduce a transition that can bypass the fence. Read, scan, event-retention, and cleanup-claim operations were also inspected; they do not mutate the protected Run writer/slot state. This task does not add physical effect calls or alter routing/DI.

## RED → GREEN evidence

The deterministic RED selector ran before wiring the transition guards:

```text
go test ./internal/application/repository -run 'TestCraftRunViewEffect(ClaimFirst|TransitionFirst|Resolution|UnresolvedSession)' -count=1
```

It failed as intended: claim-first pending/unknown transitions and admission slot transfer returned nil instead of the unresolved-effect error, and finalization was not delayed. Transition-first denial was already enforced by the existing lease/session fences.

After implementation, these commands passed on the same final source/test postimage:

```text
go test ./internal/application/repository -run '^$' -count=1
TRPC_TEST_POSTGRES_DSN='postgres://postgres:craft107_local_test_only@127.0.0.1:32771/craft107_r5?sslmode=disable' go test ./internal/application/repository -run 'TestCraftRunViewEffect(ClaimFirst|TransitionFirst|Resolution|UnresolvedSession|CannotBeBypassed|TransitionFencePostgres)' -count=1
TRPC_TEST_POSTGRES_DSN='postgres://postgres:craft107_local_test_only@127.0.0.1:32771/craft107_r5?sslmode=disable' go test -race ./internal/application/repository -run 'TestCraftRunViewEffect(ClaimFirst|TransitionFirst|Resolution|UnresolvedSession|CannotBeBypassed|TransitionFencePostgres)' -count=1
go test ./internal/application/repository -run 'TestTerminalCancellationEnqueuesCaptureAndFencesNextRun|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun|TestCraftChargePendingPauseFinalizesAfterIntentResolves|TestPauseForCraftBudgetInTxIsIdempotentAndCancelWins|TestCraftRunViewEffectResolutionAllowsFinalizeOnce' -count=1
git diff --check
```

Results respectively: PASS (compile only); PASS (SQLite plus isolated PostgreSQL 17); PASS (same selector under race); PASS (R4 terminal capture and existing charge-pending behavior); PASS. PostgreSQL used the parent-provided isolated `craft107_r5` database, with `app.skip_embedding=true` applied to its connection because the available PostgreSQL 17 instance does not provide the vector extension. No shared schema was used. The repository test window was released to the T19 owner after the final R5 commands; no Go command is currently running.

## Exact task checkpoint

- Preimage archive: `2026-09-24-craft-107-runview-r5-effect-task2b-task1-preimage.tar.gz`
- Preimage hash manifest: `2026-09-24-craft-107-runview-r5-effect-task2b-task1-pre.sha256`
- Task-local patch relative to that exact preimage: `2026-09-24-craft-107-runview-r5-effect-task2b-task1-task-local.patch`
- Postimage archive: `2026-09-24-craft-107-runview-r5-effect-task2b-task1-postimage.tar.gz`
- Postimage hash manifest: `2026-09-24-craft-107-runview-r5-effect-task2b-task1-post.sha256`
- Machine-readable checkpoint: `2026-09-24-craft-107-runview-r5-effect-task2b-task1-checkpoint.json`

Postimage archive SHA-256: `cefc83719599c2531ec25617458a165ccaae42368907c52a8bc7b5b9c34e2945`.  
Task-local patch SHA-256: `2dde61aa9f05138b7dbc73cce93d7bf1c1369f5dced8731fff41a605c61c39c9`.

The report and checkpoint artifacts are task records; the patch contains only the four owned source files and new focused test file relative to the captured preimage. The integration worktree has many concurrent work streams, so its aggregate `git status`/diff is not this task's change set.

## Remaining gate / risk

This is a database writer-transition fence only. It does not claim that R5 Task 2a's effect-claim seam is routed through production physical operations, nor does it provide external-operation TOCTOU protection by itself. Production physical routing remains gated on Task 3 integrating the fenced API and exact reconciliation, followed by independent review of this checkpoint. Session tombstoning can persist before the per-Run cancellation transaction detects an unresolved effect; it is a deletion barrier and does not clear the Run writer slot or commit a Run transition.
