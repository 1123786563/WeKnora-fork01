# T19 Task 2 — Journal-aware Run transitions

Date: 2026-09-23. Task scope: Task 2 of `2026-09-23-craft-107-t19-durable-start-plan.md`. This checkpoint is not a claim that T19/#138 is complete. Production gateway/sandbox call sites and Task 3–5 validation remain open.

## Implemented behavior

- `ClaimDriver` serializes against the Run row, refuses unresolved `intent`/`unknown` journal states, and finalizes a pending pause/cancel only after the journal resolves. `Scan` now excludes queued as well as expired/recovering Runs with unresolved starts. A regression test first failed because SQL operator precedence let queued Runs through, then passed after grouping the predicate.
- Fenced `SetStatus(waiting_user)` records `reconciling/craft_charge_start_pending` and a durable pause-request event while a charge start is unresolved. It retains the active Run slot and does not expose a resumable waiting state until reconciliation.
- Decision application, including retry/resume, is fenced by the Run row and refuses a new decision transition while a start is unresolved.
- `Finalize` retains journal evidence while recording terminal completion; unresolved hold/journal rows are not rewritten by finalization.
- `CancelRun`, `DeleteSessionRuns`, and the owner/revision-scoped Workbench cancel adapter route through the same journal-aware transition. Unresolved cancellation records a pending request, retains the session's active Run slot and journal evidence, and only releases the slot once cancellation is finalized after reconciliation.
- Pending transitions explicitly update `updated_at`.

## Changed files

- `internal/application/repository/agent_run.go` (also contains concurrent T01 input-admission work; it was preserved and not attributed to this Task 2 delta)
- `internal/application/repository/agent_run_decisions.go`
- `internal/application/repository/agent_run_lifecycle.go`
- `internal/application/repository/agent_run_craft_charge_test.go` (new)
- `internal/modules/workbench/service/workbench/interaction.go`
- `internal/modules/workbench/service/workbench/interaction_test.go`

`agent_run_events.go` was reviewed as part of the owned seam but not changed in this checkpoint. The Workbench test file already included unrelated preexisting tests; only the new `TestGormCancelPortDefersUnresolvedCraftStart` was added here.

## Verification

- RED: `go test ./internal/application/repository -run '^TestCraftChargeIntentIsNotScheduledFromQueuedRun$' -count=1` failed before the SQL predicate fix: queued Run with unresolved intent was returned by `Scan`.
- GREEN: `go test ./internal/application/repository -run 'TestCraftCharge|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1` passed.
- GREEN: `go test ./internal/modules/workbench/service/workbench -run '^TestGormCancelPortDefersUnresolvedCraftStart$' -count=1` passed.
- GREEN: `go test ./internal/modules/workbench/service/workbench -count=1` passed (`22.094s`). Log: `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-run-transition-checkpoint-01/workbench-full.log`.
- `git diff --check` passed for the Task 2-owned tracked files.
- Full package attempt: `go test ./internal/application/repository -count=1` (exit 1 after explicit SIGINT at 145.487s). It produced no package summary before interruption while a second repository test process from concurrent RunView validation was also running; `repository-full.log` ends with `signal: interrupt`. It is **not** a pass or a recorded test failure. Parent/validator should rerun once shared test activity is quiet. PostgreSQL DSN `TRPC_TEST_POSTGRES_DSN` is unset; no PG run was attempted.

## Checkpoint

- Integration HEAD: recorded in `t19-run-transition-checkpoint-01/HEAD`; no commit was created.
- Full saved source bytes and SHA-256 list: `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-run-transition-checkpoint-01/source/` and `source.sha256`.
- `tracked.patch` and complete `git-status.txt` are in that checkpoint. `agent_run.go` includes unrelated T01 shared changes; use the full bytes/hash as the integrated checkpoint, not as an assertion that every line is owned by Task 2.
- Targeted test and package logs are saved alongside this report.
- Log SHA-256: `repository-full.log` `f5b37dcfcfc5bdd73f406fd0319b03dbf3caca89966b0cac13f2ab16a9892ec4`; `workbench-full.log` `917961a62a74d95951c46daa4ab9634369984cac2738687dba7ecdc6cdeaddff`. Report SHA-256 is recorded in the checkpoint's `report.sha256`.

## Remaining limits

- The Run service pause API outside `SetStatus` must be checked against this row/journal fence before the broader Task is accepted; no ownership amendment was issued for `craft_budget.go` in this Task 2 handoff.
- Task 1's coordinator still has no production model gateway/sandbox caller. No PostgreSQL race/concurrency test was run. Independent Task 2 review and backend validation are pending.
