# T19 Task 2 Run transitions: independent scoped review

Date: 2026-09-23. Read-only review of Task 2 in `2026-09-23-craft-107-t19-durable-start-plan.md`, implementation report and `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-run-transition-checkpoint-01/` in the integration worktree. HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec/#138, charge-start journal migration and Task 1 review, current Run store and Workbench cancel seam. No source/test edits, staging, OCR or PostgreSQL run.

## Checkpoint and verdict

The seven live full-file SHA-256 values matched `source.sha256` in the saved checkpoint:

| Checkpoint file | SHA-256 |
| --- | --- |
| `internal/application/repository/agent_run.go` | `36a9ae5b4612c040e2e6e58fc70ddfecf0abf5803b09252b4e2aa4b29d9de3ff` |
| `internal/application/repository/agent_run_events.go` (inspected, unchanged) | `171b4ef95ef54ac5126e78b3392beb8df4e666be5536424993bef062e1e75cfd` |
| `internal/application/repository/agent_run_decisions.go` | `909d25bd57266a8145b7ceed7164263ae5956f68ae3b3a1696d22b4c470740f6` |
| `internal/application/repository/agent_run_lifecycle.go` | `9b809dcd5b2922a2fd3b522facd12c6ec70a8907c03f7dbc2aa5052384ed58e7` |
| `internal/application/repository/agent_run_craft_charge_test.go` | `e1d06eb9221fefe30f1b39c40ad97ce8c47ddaf1e77f341ced4dd35b76b60851` |
| `internal/modules/workbench/service/workbench/interaction.go` | `5013342afee1a679c15923797e5e0963eaef8e4b1b2593ecebd0af76d8e9b7b9` |
| `internal/modules/workbench/service/workbench/interaction_test.go` | `3fefafa69935c24cd36e04e89b0f6579f40e511d9b27deccf568c5e9792dd8f6` |

`agent_run.go` contains parallel T01 input-admission changes already in the shared tree; this review does not attribute them to Task 2. The source checkpoint and patch isolate the reviewed bytes despite other concurrent work.

- **Scoped Spec compliance: FAIL (High finding 1).** Claim/recovery, waiting, decision and cancellation paths generally retain unresolved intent/hold and prevent a new worker. But a cancel arriving after a pending budget pause is silently acknowledged without recording cancel, then resolves to `waiting_user` rather than `canceled`.
- **Scoped code quality: FAIL (same finding).** The pending transition uses only a common `reconciling` marker and latest request event. The cancellation code treats every existing pending marker as an idempotent cancel, without checking whether its recorded event is pause or cancel. The tests cover pause and cancel separately, not pause→cancel.
- **Full T19: NOT VERIFIED.** The direct budget-pause writer outside Task 2 still bypasses this journal fence (High integration gate below). Model gateway and sandbox initiation call sites, full repository suite and PostgreSQL concurrency evidence remain open.

## High 1 — cancel after pending pause is lost

**Evidence / affected symbols:** With unresolved journal state, `SetStatus(waiting_user)` calls `requestCraftChargePause`, which writes `reconciling/craft_charge_start_pending` and appends `craft_charge_pause_requested` (`agent_run.go:785-803, 507-523`). If `CancelRun`, `DeleteSessionRuns` or Workbench revision cancel then reaches `cancelRunTx`, it returns nil for *any* such pending state without appending a cancel request (`agent_run_lifecycle.go:40-52, 69-84, 104-113`; Workbench delegates at `interaction.go:343-350`). After journal resolution, `resolveCraftChargePendingTransition` reads the latest request event, still the pause, and commits `waiting_user` (`agent_run.go:543-587`). The focused tests exercise pause resolution and cancel resolution independently (`agent_run_craft_charge_test.go:70-147`) but not the ordering pause→cancel.

**Impact:** An acknowledged user/session-deletion cancellation can become a resumable budget pause. The active Run slot remains held; the client or cleanup path can believe cancellation was requested while the Run later allows a decision/resume. This violates the distinct requested-versus-confirmed stop semantics and Task 2's transition fence.

**Smallest defensible correction:** When a pending marker already exists, inspect its durable request kind. If it is a pause and cancellation arrives, append a fenced `craft_charge_cancel_requested` event (and cancellation-requested projection) under the same Run-row lock, so cancel takes precedence after reconciliation. Repeated cancel should be idempotent; cancellation should never be downgraded back to pause. Test pause→cancel, cancel→pause, repeated cancel and session deletion under unresolved intent, including Workbench revision behavior.

## High integration gate — direct budget pause bypasses the journal fence

This file was explicitly outside Task 2 ownership, and the implementer report correctly left it open. `CraftBudgetService.PauseRunForBudget` and `pauseOnDenial` call `pauseCraftRunTx` (`internal/application/service/craft_budget.go:328-340, 1026-1041`). That helper updates a queued/running/recovering Run directly to `waiting_user`, clears its lease and does not query unresolved `intent`/`unknown` (`:375-386`). Therefore an unresolved external start can be exposed as a resumable budget pause through a production writer despite Task 2's `SetStatus` fence. The smallest central follow-up is to route these writers through the same locked journal-aware pause transition and test an unresolved start on both entry points. This is a full T19 blocker, not a defect attributed to Task 2's owned patch.

## Confirmed behavior and verification limits

`claimableSQL` groups the queued/expired predicate before excluding unresolved journal rows; `ScanDriver` uses it and `ClaimDriver` also locks the Run then checks unresolved state (`agent_run.go:478-493, 599-691, 722-737`). `SetStatus(waiting_user)` records a pending pause without releasing the active slot (`:785-823`); decision application locks and refuses unresolved starts (`agent_run_decisions.go:87-150`). `Finalize` is fenced and retains journal evidence while completing a terminal Run (`agent_run_events.go:111-174`). `CancelRun` and Workbench use the same journal-aware cancellation implementation, and `DeleteSessionRuns` retains rows and a pending active slot while unresolved (`agent_run_lifecycle.go:14-123`). These mechanisms do not erase the two gaps above.

I independently ran `go test ./internal/application/repository -run 'TestCraftCharge|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1` and the focused Workbench cancel test; both passed. `git diff --check` on Task 2 tracked files passed. The report's full repository package run was interrupted after 145 seconds and is not a pass; no independent full-suite rerun was performed. `TRPC_TEST_POSTGRES_DSN` was unset in this environment, so SQLite behavior does not establish PostgreSQL row-lock/race behavior. Current tests cover a stale worker claim but do not cover the pause→cancel race or all production transition permutations. No T01 input-admission regression is implied by this review; its shared `agent_run.go` bytes were preserved at the recorded hash, and separate T01 gates remain authoritative.
