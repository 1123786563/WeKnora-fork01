# T19 Budget Pause Service Task 2 Report

Date: 2026-09-23. Implements Task 2 only from `2026-09-23-craft-107-t19-budget-pause-fix-plan.md`. No commit was created.

## Result

Replaced all four direct `pauseCraftRunTx` writers in `CraftBudgetService` with `pauseCraftRunInTx`, which passes the existing locked transaction and `RunKey{TenantID, RunID}` to the reviewed `AgentRunStore.PauseForCraftBudgetInTx` seam. The service constructor creates the repository adapter from the same DB; there is no public constructor or container signature change. There are no remaining direct `pauseCraftRunTx` calls or helper in `craft_budget.go`.

The two `StartBinding` denial paths still use the original transaction and Run row lock. Reservation denial still removes the fresh call ledger row before requesting a pause. The service sets `preparation.denial` only after the repository pause method succeeds, so the outer transaction commits the pause request before `StartBinding` returns the original typed denial. Repository failures return directly and roll back the transaction. `PauseRunForBudget` and `pauseOnDenial` also retain their existing outer transactions and locks. Pending cancellation errors out rather than being downgraded by a later budget pause.

## Changed files

- `internal/application/service/craft_budget.go`
- `internal/application/service/craft_budget_t19_test.go`

`craft_budget_start_test.go` and `craft_budget_test.go` were not changed.

## RED → GREEN and verification

- RED command against the pre-Task-2 service: `go test ./internal/application/service -run '^TestCraftBudgetPauseWritersFenceIntentAndUnknown$' -count=1`. All eight combinations (four writers × `intent|unknown`) failed because service writers exposed `waiting_user` instead of `reconciling`. Full output is `red-service.log`.
- GREEN same matrix after routing through the repository seam passed (`ok github.com/Tencent/WeKnora/internal/application/service 18.162s`). It verifies pause event persisted before return, denial identity, no start callback, no new reservation, existing dispatched G4 hold retention, active Run lease and session slot retention, scan/claim exclusion before resolution, and `waiting_user` only after journal resolution.
- `go test ./internal/application/service -run '^TestCraftBudgetPauseDoesNotDowngradePendingCancellation$|^TestCraftBudgetStartDenialReturnsRepositoryFailureInsteadOfDenial$' -count=1` passed. This checks cancel precedence and that a failed durable event append returns the repository failure, not a budget denial.
- Updated the existing `TestCraftChargeFenceBFirstStartOrdersBeforePause`: while callback/journal outcome is unresolved it now expects `reconciling`; after the started outcome is recorded, the reconciliation claim finalizes `waiting_user`. `go test ./internal/application/service -run '^TestCraftChargeFenceBFirstStartOrdersBeforePause$' -count=1` passed.
- Final broad focused budget/service command: `go test ./internal/application/service -run 'TestCraftBudget|TestCraftCharge' -count=1` passed (`ok .../internal/application/service 102.660s`). Full output is `service-focused.log`.
- Repository journal/cancel selection: `go test ./internal/application/repository -run 'TestCraftCharge|TestPauseForCraftBudgetInTx|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1` passed (`27.823s`).
- Workbench cancel adapter: `go test ./internal/modules/workbench/service/workbench -run '^TestGormCancelPortDefersUnresolvedCraftStart$' -count=1` passed (`1.369s`).
- `git diff --check -- internal/application/service/craft_budget.go internal/application/service/craft_budget_t19_test.go internal/application/service/craft_budget_start_test.go internal/application/service/craft_budget_test.go` passed.
- PostgreSQL tests were not available because `TRPC_TEST_POSTGRES_DSN` is unset. The focused SQLite tests do not establish PostgreSQL row-lock concurrency behavior.

## Prior repository package report correction

Task 1 report `2026-09-23-craft-107-t19-budget-pause-repository-report.md` was amended per independent review to list all five failures from its full package run, not just the last two shown by the tail: `TestWorkbenchOwnedRunIsTenantScoped`, `TestCancelRunReleasesSlotAndDeleteFences`, `TestArtifactVersionCrossTenantIsolation`, `TestWorkbenchHTTPOwnershipUsesRealStoreAndProjection`, and `TestWorkbenchHTTPSourceEventToSnapshotAndSSEUsesRealRepository`. The three `agent runtime conflict` results are consistent with concurrently tightened active-membership test fixtures, while the cancellation event count mismatch reflects behavior requiring separate triage. These observations do not establish that every failure is unrelated. The correction's report SHA-256 is `c160faba3b31cd46ffdd5ff6e85dda8e9b6c314008a8dfb76e7c6579ee021248`; review and full-package evidence remain in the Task 1 checkpoint.

## Exact uncommitted checkpoint and limits

Checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-budget-pause-service-checkpoint-01/`.

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Live SHA-256: `craft_budget.go` `fe7414ab725b88827969379b0f12cdae6752a15d0bb60ceb4ddd1758e5d56f52`; `craft_budget_t19_test.go` `187488c8bc03a9a037028d5cc55254f5d62179a8487c14e44818e0a2bcc4f97e`.
- `craft_budget_start_test.go` is byte-identical before/after at `02faea72b26b066e959f8b40672706d42d7c30f7780f636fb887543c0f12b67a`.
- Checkpoint contains exact before/after full files, SHA-256 manifests, task delta, RED/GREEN focused logs, HEAD, and complete shared-worktree status.
- Independent review is pending. T19 remains incomplete until this checkpoint is reviewed/integrated and production gateway/sandbox initiation, PostgreSQL locking evidence, full integration validation and OCR are handled. No overall repository-suite pass is claimed here.
