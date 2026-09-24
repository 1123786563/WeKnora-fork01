# T19 Budget Pause Repository Transition Report

Date: 2026-09-23. Implements Task 1 only from `2026-09-23-craft-107-t19-budget-pause-fix-plan.md`. No commit was created.

## Result

- Added `AgentRunStore.PauseForCraftBudgetInTx(ctx, tx, key, reason)`. It uses the caller's transaction and Run key, reads and validates the current Run state, and does not create a nested transaction or touch the session's active Run slot.
- Shared `SetStatus(waiting_user)` and budget pause behavior through `transitionCraftBudgetPauseTx`, keeping the existing lease fence on `SetStatus` and requiring the budget caller to hold the Run row lock.
- For unresolved `intent|unknown`, writes the pending pause status and journal request atomically, retains the active Run slot and lease, and keeps the Run out of scan/claim eligibility until journal resolution. The reconciliation transition clears the lease only after resolution and then confirms `waiting_user`.
- Replayed pending pause is idempotent. A pending cancellation takes precedence and causes a budget pause attempt to return `agentruntime.ErrConflict`. Resolved or absent journals transition directly to the requested budget wait state.
- Event insertion failure returns an error; with the caller's transaction it rolls back the status, revision, event, and lease mutation.

## Changed files

- `internal/application/repository/agent_run.go`
- `internal/application/repository/agent_run_craft_charge_test.go`

No service or other agent-owned files were edited.

## RED → GREEN and verification

- RED: `go test ./internal/application/repository -run '^TestPauseForCraftBudgetInTx' -count=1` failed to compile because the new repository method did not yet exist at the new test call sites. The exact red record is `red.log` in the checkpoint.
- Initial implementation test run exposed a wrong assertion in the new test: unresolved charge starts must remain absent from `Scan`, not be returned for reconciliation while unresolved. The assertion was corrected to verify scan exclusion, matching the journal hold invariant.
- GREEN: `go test ./internal/application/repository -run '^TestPauseForCraftBudgetInTx' -count=1` passed (`ok .../internal/application/repository 11.529s`).
- GREEN: `go test ./internal/application/repository -run 'TestCraftCharge|TestPauseForCraftBudgetInTx|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1` passed (`ok .../internal/application/repository 20.452s`).
- GREEN final focused run: `go test ./internal/application/repository -run 'TestAgentRunAdmission|TestCraftRunAdmission|TestGenericRunAdmission|TestAdmissionUsesActiveMembership|TestCraftCharge|TestPauseForCraftBudgetInTx|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1` passed (`ok .../internal/application/repository 23.929s`). This includes active-membership admission regressions from T08.
- `git diff --check -- internal/application/repository/agent_run.go internal/application/repository/agent_run_craft_charge_test.go` passed.
- PostgreSQL repository cases were not exercised: `TRPC_TEST_POSTGRES_DSN` is unset. SQLite tests do not establish PostgreSQL row-lock concurrency behavior.

## Full package run and unrelated integration failures

`go test ./internal/application/repository -count=1` completed with exit code 1 after 175.794s. The complete command output is checkpointed as `full-package.log`. Independent review found five failing tests:

- `TestWorkbenchOwnedRunIsTenantScoped` (`agent_run_driver_test.go:122`) — unexpected `agent runtime conflict`.
- `TestCancelRunReleasesSlotAndDeleteFences` (`agent_run_lifecycle_test.go:37`) — expected two `cancellation_requested` events after session deletion, observed one.
- `TestArtifactVersionCrossTenantIsolation` (`artifact_version_test.go:261`) — unexpected `agent runtime conflict`.
- `TestWorkbenchHTTPOwnershipUsesRealStoreAndProjection` (`workbench_http_integration_test.go:65`) — unexpected `agent runtime conflict`.
- `TestWorkbenchHTTPSourceEventToSnapshotAndSSEUsesRealRepository` (`workbench_http_integration_test.go:90`) — unexpected `agent runtime conflict`.

The three conflict errors are consistent with tightened active-membership admission fixtures; for example `openWorkbenchHTTPDB` inserts tenant, user, and session but no `tenant_members`. The cancellation count differs from the prior pending-cancel implementation's idempotent behavior. These are observations, not a ruling that all failures are unrelated. They are outside this Task's owned files and need separate triage before any overall T19 or integration completion claim. Full output, including logged expected database errors, is retained untruncated in the checkpoint.

## Exact uncommitted checkpoint

Checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-budget-pause-repository-checkpoint-01/`.

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Live SHA-256: `agent_run.go` `920ce165ef5c5c79a6e27cceeeca002151868905297c344d8bfb07f914883cae`; `agent_run_craft_charge_test.go` `2be37987027d24854ae0fda56ec6f2881cf85a0059bee9f69824637d21e5bc31`.
- Checkpoint contains exact before/after full files, SHA-256 manifests, task delta, RED/focused/full-package logs, HEAD, and full shared-worktree status. The `agent_run.go` before image is the accepted T08 actor-membership checkpoint; the charge test before image is the accepted T19 pending-cancel checkpoint. This isolates this task's incremental delta without removing prior approved edits.
- Independent review is required before service Task 2. T19, production budget callers, gateway/sandbox initiation, PostgreSQL concurrency, and full-repository completion are not claimed here.
