# T63 Task 5 R1 Fix Report

- Task: T63 Task 5 review repair, authoritative lifecycle gate at `AgentRunStore.Admit`.
- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t63`
- Branch: `codex/issue30-t63`
- Base HEAD: `cded1932c8fc8ffc8401d5ab883052f33c0d102b` (Task 5 checkpoint).
- Scope: the runtime contract, AgentRun repository/store test, workbench admission/test, and command handler/error test files assigned by `plan-t63-task5-review-fix.md`, plus this report.

## Changes

- Added explicit `Admission.AgentID` and shared `agentruntime.ErrAgentUseDenied`. `AgentRunStore.Admit` now checks an existing request before the lifecycle check; for a new run it performs a no-op UPDATE over matching tenant/local-Agent variant rows inside its transaction, then reads retirement state before reserving the session slot or creating rows. Retirement and admission therefore contend on the same variant row. The gate uses explicit `Admission.AgentID`, not snapshot JSON.
- `AdmissionCoordinator.Start` resolves same-hash admitted/dispatching requests before the fast Agent-use gate, preserving replay after retirement. Pending retries remain gated. New attempts still use the early gate before creating a request row.
- A final transaction denial transitions that pending intent to rejected and releases its unstarted reservation only when the pending-to-rejected update succeeds. It creates no run, message, or session slot.
- Workbench's `ErrAgentUseDenied` aliases the runtime sentinel, and `queue_next` command errors map it to HTTP 409.
- Added deterministic SQLite tests for the gate/retirement race, the opposite lock ordering, idempotent replay, mismatched hash, pending retry denial, reservation release, no run/slot on denial, and queue_next status mapping.

## Evidence

1. RED:
   - Repository command: `go test ./internal/application/repository/ -run 'TestAgentRunAdmit(RejectsRetiredVariant|SerializesWithVariantRetirement)' -count=1`
   - Result before implementation: compile failure for missing runtime `Admission.AgentID` and `agentruntime.ErrAgentUseDenied`.
   - Workbench/handler behavior RED: initial run returned foreign-key fixture errors (missing Listing/Adoption rows) and the queue_next mapping test got 500 instead of expected 409. The fixture was corrected with real parent rows; production change added the missing mapping.
2. Repeated focused tests:
   - `go test ./internal/application/repository/ -run 'TestAgentRunAdmit(RejectsRetiredVariant|SerializesWithVariantRetirement)' -count=10` — PASS (`ok .../repository 17.051s`).
   - `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission(RetirementBetweenFastGate|ReplaysCommittedRun)' -count=5` — PASS (`ok .../service/workbench 11.894s`).
   - `go test ./internal/handler/session/ -run TestQueueNextAgentUseDenied -count=1` — PASS (`ok .../handler/session 1.221s`).
3. Adjacent behavior:
   - `go test ./internal/application/repository/ -run 'TestAgentRunAdmit|TestRetiredVariantAgentExists' -count=1` — PASS (`3.664s`).
   - `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission|TestQueueNext' -count=1` — PASS (`8.741s`).
   - `go test ./internal/handler/session/ -run 'TestQueueNextAgentUseDenied|TestWorkbench(StartHTTPIntegrationAndIdentityIsolation|StopThenRestartHTTPIntegration|QueueNextStaleRevisionIsAConflict)' -count=1` — PASS (`2.751s`).
   - `go test ./internal/container/ -run 'TestWorkbenchAdmission|TestAgentMarketplaceLifecycle' -count=1` — PASS (`2.380s`; linker warned about duplicate `-lc++`).
4. Build:
   - `go build ./...` — PASS, exit 0. The desktop and server link steps emitted the existing duplicate `-lc++` warning.
5. Diff:
   - `git diff --check` — PASS, no output.

## Limits

- The concurrency proof uses file-backed SQLite, a multi-connection pool, and channel/GORM callbacks. It demonstrates SQLite serialization and does not claim a live PostgreSQL execution.
- Task 6 and all T64 files were left untouched.

## Commit

- Local implementation commit: pending at report creation; final HEAD is recorded by Git after commit.
