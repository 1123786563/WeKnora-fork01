# T19 budget-pause service writers: independent scoped review

Date: 2026-09-23. Read-only review of Task 2 in `t19-budget-pause-fix-plan.md`, `t19-budget-pause-service-report.md`, the exact checkpoint delta, the reviewed repository Task 1 seam, and approved Craft Spec #107/#138. Integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No code/test edits, staging, delegation or OCR.

## Checkpoint and verdict

| Full file | SHA-256 |
| --- | --- |
| `internal/application/service/craft_budget.go` | `fe7414ab725b88827969379b0f12cdae6752a15d0bb60ceb4ddd1758e5d56f52` |
| `internal/application/service/craft_budget_t19_test.go` | `187488c8bc03a9a037028d5cc55254f5d62179a8487c14e44818e0a2bcc4f97e` |
| Unchanged verification dependency `internal/application/service/craft_budget_start_test.go` | `02faea72b26b066e959f8b40672706d42d7c30f7780f636fb887543c0f12b67a` |

The live hashes match checkpoint `t19-budget-pause-service-checkpoint-01/after.sha256` and the implementation report. Its `task-delta.patch` changes only `craft_budget.go` and `craft_budget_t19_test.go`; `craft_budget_start_test.go` is byte-identical before/after.

- **Scoped Spec compliance: PASS.** All four former `pauseCraftRunTx` production callers now use the journal-aware repository transition in their existing Run-locked transaction. An unresolved `intent|unknown` start stays `reconciling` with durable pause request, lease, slot and G4 hold; it cannot be claimed as a new execution. A budget denial is returned after the transaction commits. Pending cancel takes precedence and repository failure cannot be reported as a committed budget denial.
- **Scoped code quality: PASS.** The service constructs `AgentRunStore` from the same DB, passes the existing `*gorm.DB` transaction and exact tenant/Run key without nesting, removes the direct SQL helper, and retains reservation cleanup before pause. The eight-case test matrix, cancellation/rollback tests, and B-first ordering test cover the changed boundary. No new scoped finding was identified.
- **Full T19: NOT VERIFIED.** Model gateway and sandbox production callers, PostgreSQL concurrency/row locks, central full-package validation and OCR remain separate gates.

## Evidence

`StartBinding` holds the exact Run write lock in its outer transaction before both max-call and reservation-denial paths (`craft_budget.go:201-205,226-254`). On reservation denial it deletes the fresh call-ledger row before requesting a pause (`:244-253`). Both paths assign the original typed denial only after `pauseCraftRunInTx` succeeds, and `StartBinding` returns it only after the transaction returns successfully (`:272-280`). A repository event/transition error returns from the transaction and rolls back, rather than masquerading as the budget denial.

`PauseRunForBudget` and `pauseOnDenial` also preserve their outer transactions and `lockCraftRun` ordering (`craft_budget.go:330-342,1021-1036`). The shared service helper passes that transaction to `AgentRunStore.PauseForCraftBudgetInTx` with `RunKey{TenantID, RunID}` and the fixed budget wait reason (`:345-348`). `NewCraftBudgetService` assembles the repository adapter from the same `db` (`:414-435`). The old `pauseCraftRunTx` definition and all four direct references are gone from `craft_budget.go`. This satisfies the repository seam's row-lock precondition and introduces no nested transaction.

The new `TestCraftBudgetPauseWritersFenceIntentAndUnknown` covers four writer paths × two journal states. It asserts a committed pause event, no callback, retained G4 hold, `reconciling`/lease/slot and scan/claim exclusion before resolution, then `waiting_user` only after journal resolution (`craft_budget_t19_test.go:368-470`). Further cases assert a pending cancel is not downgraded and a rejected event append returns a repository error while rolling back the Run transition (`:473-537`). The B-first test now waits for the journal's started outcome and an explicit reconciliation claim before expecting `waiting_user` (`:264-344`). The repository tests independently cover repeated pause and cancel precedence.

I independently ran `go test ./internal/application/service -run 'TestCraftBudget|TestCraftCharge' -count=1` (passed), `go test ./internal/application/repository -run 'TestCraftCharge|TestPauseForCraftBudgetInTx|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1` (passed), `go test ./internal/modules/workbench/service/workbench -run '^TestGormCancelPortDefersUnresolvedCraftStart$' -count=1` (passed), and `git diff --check` on the two changed files (passed). `TRPC_TEST_POSTGRES_DSN` is unset, so PostgreSQL row-lock behavior is not established. A central full repository rerun was still pending at this scoped checkpoint.
