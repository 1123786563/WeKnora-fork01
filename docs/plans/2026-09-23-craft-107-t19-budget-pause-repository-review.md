# T19 budget-pause repository seam: independent scoped review

Date: 2026-09-23. Read-only review of Task 1 in `t19-budget-pause-fix-plan.md`, `t19-budget-pause-repository-report.md`, the checkpoint task delta, approved Craft Spec #107/#138, and the prior pending-cancel PASS review. Integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no code/test edits, staging, delegation or OCR.

## Exact checkpoint and verdict

| Full source file | SHA-256 |
| --- | --- |
| `internal/application/repository/agent_run.go` | `920ce165ef5c5c79a6e27cceeeca002151868905297c344d8bfb07f914883cae` |
| `internal/application/repository/agent_run_craft_charge_test.go` | `2be37987027d24854ae0fda56ec6f2881cf85a0059bee9f69824637d21e5bc31` |

Both hashes match `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-budget-pause-repository-checkpoint-01/after.sha256` and the implementation report. I reviewed that checkpoint's `task-delta.patch` so earlier T08 actor, T01 admission and T19 journal edits in the shared files are not attributed to this Task.

- **Scoped Spec compliance: PASS.** The transaction-scoped method routes unresolved `intent|unknown` starts to a durable pause request and `reconciling` state, retains the Run lease and session slot, excludes claim/scan, and confirms `waiting_user` only after journal resolution. Pending cancel outranks a repeated budget pause. Resolved/absent journals can enter the budget wait state directly.
- **Scoped code quality: PASS.** The new method uses the caller's `*gorm.DB` transaction without opening another; the helper is shared with fenced `SetStatus(waiting_user)`. Run-row serialization, event insert and status/revision change occur in one transaction. The focused tests cover journal states, retry idempotency, cancel precedence and rollback on event failure. No new scoped finding was identified.
- **Full T19: NOT VERIFIED.** Task 2 has not yet routed the four `CraftBudgetService` writers through this method; current direct `pauseCraftRunTx` calls remain. PostgreSQL row-lock/concurrency evidence and production gateway/sandbox integration remain open.

## Evidence

The budget service's existing `lockCraftRun` obtains a no-op write lock on the exact tenant/Run row before its current pause call sites (`craft_budget.go:199-202,335-339,348-363,1032-1036`); Task 2 must keep that ordering when calling the new seam. `PauseForCraftBudgetInTx` validates its transaction/key/reason and delegates to `transitionCraftBudgetPauseTx` on the passed transaction (`agent_run.go:638-646`). The helper checks the exact Run state and journal (`:572-615`), writes a pending pause event with the reconciling status for unresolved starts (`:551-567`), and preserves lease fields there. It returns conflict if a pending cancel exists, and replays an existing pause without a second revision/event (`:581-599`). Direct wait after a resolved/absent journal clears the lease with the status/revision change (`:618-635`). The later resolver orders cancel requests ahead of pause, waits for no unresolved journal, then clears the lease and confirms the selected terminal/wait state (`:666-713`). `ClaimDriver` and `Scan` use the unresolved-journal exclusions (`:522-527,735-762`).

`SetStatus` obtains its existing fenced no-op row lock before invoking the shared helper for `waiting_user` (`agent_run.go:911-925`), retaining stale lease/epoch rejection. For a budget caller, the seam assumes the transaction already holds the Run row lock; it does not independently acquire one. This is an explicit interface precondition and should be checked again in the Task 2 call-site review. Tests verify `intent` and `unknown` preserve lease and stay unclaimable, resolved/absent journal goes to `waiting_user`, replay does not advance revision, pending cancel wins, and a rejected event insert rolls back status/lease/revision (`agent_run_craft_charge_test.go:247-369`).

I independently ran the focused repository command covering admission, Craft charge, budget pause and unresolved delete behavior; it passed (`go test ./internal/application/repository -run 'TestAgentRunAdmission|TestCraftRunAdmission|TestGenericRunAdmission|TestAdmissionUsesActiveMembership|TestCraftCharge|TestPauseForCraftBudgetInTx|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1`). `git diff --check` on the tracked reviewed file passed. `TRPC_TEST_POSTGRES_DSN` is unset; the SQLite checks do not establish PostgreSQL lock behavior.

## Broader suite evidence discrepancy

The checkpoint's `full-package.log` reports **five** failing tests, although the implementation report says exactly two. They are `TestWorkbenchOwnedRunIsTenantScoped` (`agent_run_driver_test.go:122`), `TestCancelRunReleasesSlotAndDeleteFences` (`agent_run_lifecycle_test.go:37`), `TestArtifactVersionCrossTenantIsolation` (`artifact_version_test.go:261`), `TestWorkbenchHTTPOwnershipUsesRealStoreAndProjection` (`workbench_http_integration_test.go:65`), and `TestWorkbenchHTTPSourceEventToSnapshotAndSSEUsesRealRepository` (`:90`). The three conflict errors are consistent with concurrently tightened active-membership admission fixtures; the lifecycle test expects a second cancellation event after deletion of an already canceled Run, whereas the earlier pending-cancel change is designed for idempotence. These are observations, not a ruling that all five are unrelated. None is exercised by this Task's delta, but the full repository package is **not passing** and needs separate fixture/behavior triage before an overall T19 or integration completion claim.
