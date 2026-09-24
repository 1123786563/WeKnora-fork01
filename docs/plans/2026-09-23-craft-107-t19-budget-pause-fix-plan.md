# T19 Budget Pause Journal Fence Fix Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact checkpoints, independent Spec/quality Review. No commit.

**Goal:** Route all four production Craft budget-pause writers through the same durable charge-start journal fence as Run transitions, so an unresolved external start cannot be exposed as a confirmed resumable pause.

**Sources:** Spec #107/#138, `t19-run-transition-review.md`, `t19-pending-cancel-fix-review.md`, `t19-budget-pause-seam-design.md`, existing T19 durable-start plan and journal migration.

**Global Constraints:** T08 actor Task 1 currently owns `agent_run.go`; Task 1 below waits its exact checkpoint and independent review. No shared nested transaction, no lease release/slot reuse while journal is `intent|unknown`, and pending cancel must outrank a later budget pause. Existing budget denial must be returned only after the pause request commits. Preserve owner/actor fields and T01 admission edits in shared files.

**Review Focus:** four call sites, transaction ordering, unknown outcomes, cancel precedence, retry idempotency, claim exclusion, SQLite and PostgreSQL row-lock semantics, no false `waiting_user` before resolution.

## Task 1 — repository transition seam

**Depends on:** T08 actor Task 1 reviewed/integrated and `agent_run.go` released. **Role:** backend_implementer. **Owned files:** `internal/application/repository/agent_run.go`, `internal/application/repository/agent_run_craft_charge_test.go`. **Consumes:** existing locked `*gorm.DB` transaction and exact `RunKey`; **Produces:** transaction-scoped `PauseForCraftBudgetInTx` sharing the journal-aware waiting transition with `SetStatus`.

1. RED tests for unresolved intent/unknown, clean/resolved journal, pending cancel precedence, duplicate pause, and event-write rollback. Assert unresolved start retains active slot/lease and claim is excluded.
2. GREEN via one private shared helper. The new method uses the caller's transaction and validates row/key/status; it never opens its own transaction or calls `SetStatus`. Return typed conflict for a pending cancel. Preserve lease fencing in `SetStatus` and current budget reason.
3. Focused repository tests, PostgreSQL tests when DSN exists, diff check and full-file hash checkpoint; independent reviewer must pass before Task 2.

## Task 2 — budget service routes all writers

**Depends on:** Task 1 reviewed/integrated. **Role:** backend_implementer. **Owned files:** `internal/application/service/craft_budget.go`, `craft_budget_t19_test.go`, `craft_budget_start_test.go`, `craft_budget_test.go` only for affected assertions; `container.go` only with a separate ownership amendment. **Consumes:** transaction-scoped repository method. **Produces:** four journal-aware budget pause paths.

1. RED tests for `StartBinding` count exhaustion, reservation denial, `PauseRunForBudget`, and `pauseOnDenial` with unresolved start. Assert committed pause-request event, `reconciling` state, no early `waiting_user`, original denial identity after commit, and resolution semantics.
2. Replace exactly four direct `pauseCraftRunTx` calls. Keep existing transaction and Run-row lock; no nested transaction. Reservation cleanup remains before pause. Repository errors roll back and do not masquerade as budget denials.
3. Run focused budget/repository/Workbench tests, configured PG race tests, diff check and exact checkpoint; independent Review then joined validation. If any other direct budget pause writer appears, update the plan and cover it before claiming T19.

**Failure handling:** A missing repository transaction interface or absent PostgreSQL DSN is reported as an evidence limit; do not route through an unfenced direct SQL update. T19 remains unverified until gateway/sandbox callers and full OCR are also complete.
