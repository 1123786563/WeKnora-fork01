# T19 budget pause through the charge-start transition fence

**Read-only design, 2026-09-23.** Integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`; moving-tree SHA-256 at inspection: `internal/application/service/craft_budget.go` `ed033c9a66fa29893d7fb81345237cdbc7a745442b646aeb57f2ae4ae06dcea1`, `internal/application/repository/agent_run.go` `f15bd6b76189d47f4ddca7732942cc79d40dfd1613a378aa5d3099a5507c53e1`. Recheck checkpoint before edits. Sources: approved Craft spec, T19 durable-start plan, `2026-09-23-craft-107-t19-run-transition-review.md`; pending-cancel correction is already reported PASS and is not reopened here.

## Gap and invariant

Four service call sites still use `pauseCraftRunTx`: `StartBinding` budget count exhaustion (`craft_budget.go:225`), `StartBinding` reservation denial (`:248`), `PauseRunForBudget` (`:339`), and `pauseOnDenial` (`:1036`). That helper directly updates queued/running/recovering to `waiting_user`, clears lease, and increments revision. It never reads `craft_charge_start_journal`. The repository's `SetStatus(waiting_user)` checks unresolved `intent|unknown` under the Run lock and, if present, writes `reconciling/craft_charge_start_pending` plus `craft_charge_pause_requested`; it later confirms waiting only after journal reconciliation. Budget service must use this same transition policy, while keeping its current denial return value and the StartBinding budget transaction atomic.

## Smallest interface

Add a repository method on `AgentRunStore`, conceptually:

```go
PauseForCraftBudgetInTx(ctx context.Context, tx *gorm.DB, key agentruntime.RunKey, reason string) error
```

This is an infrastructure transaction seam, not a new public runtime command or a second policy implementation. It accepts the **existing transaction** after `lockCraftRun` has acquired the Run-row write lock. It must not start `db.Transaction`, call `SetStatus`, or acquire a second connection. It checks current status and unresolved journal using the same repository helpers as `SetStatus`; on `intent|unknown`, call the existing `requestCraftChargePause` on `tx`; otherwise perform the same fenced direct `waiting_user` update as `SetStatus`, with reason `craftBudgetWaitReason`. It does not clear the active session slot. Pending cancel has precedence: if state is already `reconciling` with a cancel request, do not append a pause that could downgrade it; return a typed nonchargeable/conflict result. A duplicate budget pause already waiting for budget may be idempotent only if the original service contract allows it; the four original call sites' current queued/running/recovering preconditions otherwise remain intact.

Implement one private repository helper for `SetStatus(waiting_user)` and `PauseForCraftBudgetInTx` to share the journal decision and transition. This prevents future divergence. `SetStatus` retains its lease-fence check; the budget method relies on the Run-row lock already held by `lockCraftRun` and must verify the row/key/status itself. The service passes `RunKey{TenantID: grant.TenantID, RunID: grant.RunID}` and exact budget reason. Never let the service inspect journal rows or append transition events directly.

## Denial and transaction ordering

In the two `StartBinding` denial branches, invoke the new method **inside the existing transaction**, set `preparation.denial` and return `nil` so the pause request commits before returning `ErrGrantExhausted` or mapped reservation denial. Preserve reservation cleanup before pause on reserve denial. A repository failure returns an error and rolls the transaction back; do not return a budget denial unless its durable pause/request committed. `PauseRunForBudget` and `pauseOnDenial` retain their current outer transactions and `lockCraftRun`, then call the new method; `pauseOnDenial` returns its original denial after successful commit. This avoids nested transactions and preserves the write-order fence against external StartBinding. Budget denial can coexist with Run status `reconciling` while charge-start outcome is unknown; UI should read the pending stop state, not assume `waiting_user` is confirmed.

## Exact ownership and tests

1. **Repository owner:** `internal/application/repository/agent_run.go`, `internal/application/repository/agent_run_craft_charge_test.go`. Add the transaction method and shared transition helper. Test unresolved `intent` and `unknown`, no journal, resolved journal, pending cancel precedence, repeated pause, and rollback on event write error. Check lease remains held logically through `reconciling`, claim is excluded, and reconciliation yields `waiting_user` only when safe.
2. **Budget service owner (after repository checkpoint integrated):** `internal/application/service/craft_budget.go`, `internal/application/service/craft_budget_t19_test.go`, `internal/application/service/craft_budget_start_test.go`, and `internal/application/service/craft_budget_test.go` only for affected assertions. Replace exactly four calls, inject/construct the repository adapter without changing public Craft budget behavior. Test all four entry points with an unresolved journal and assert denial identity, committed `craft_charge_pause_requested`, no early `waiting_user`, and final reconciliation. Include SQLite and PostgreSQL row-lock coverage where configured.
3. **Central wiring owner only if injection is needed:** `internal/container/craft_model_gateway.go`. Prefer constructing `AgentRunStore` from the already supplied `db` inside the budget service to avoid a container signature change; if repository configuration requires a shared instance, inject it explicitly and keep the existing tests' constructor path. Do not overlap live T19/container edits.

## Review gate

The method's contract is transaction-scoped and lock-required. Tests must prove it never opens a nested transaction and that all four service paths share the same journal-aware transition. Re-run focused repository/budget tests and transaction race tests against PostgreSQL where available. The existing pending-cancel PASS remains necessary but does not cover these service writers.
