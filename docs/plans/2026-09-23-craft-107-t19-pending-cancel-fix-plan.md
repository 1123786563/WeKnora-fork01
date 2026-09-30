# T19 Pending Pause → Cancel Fix Plan

> **For Codex:** SDD RED → GREEN → REFACTOR, exact checkpoint and independent Review. No commit.

**Goal:** Close Task 2 review's High in-scope finding: `CancelRun` returns success without recording cancel when the Run is already `reconciling/craft_charge_start_pending` after a pause request, so journal resolution chooses the older pause and leaves a resumable Run.

**Sources:** Approved Spec #107/#138, `t19-run-transition-report.md` and pending Review, `agent_run_lifecycle.go:cancelRunTx`, `agent_run.go:requestCraftChargeCancel/resolveCraftChargePendingTransition`, T19 journal migration. Separate full-T19 bypass in `CraftBudgetService.pauseCraftRunTx` is not this Task; it needs its own plan after this fix.

**Global Constraints:** backend_implementer owns only `internal/application/repository/agent_run_lifecycle.go`, `agent_run.go` if helper must change, and focused `agent_run_craft_charge_test.go`; report. Do not edit T08 craft_session, T05 knowledge, RunView store/SQL, Workbench, budget service, gateway/sandbox, commit or spawn agents. Other agents share integration Worktree. Maintain exact T01 input admission changes in agent_run.go.

**Review Focus:** unresolved start + pause request + authenticated cancel request must persist a later cancel intent; after journal resolves the Run is canceled, session slot cleared, hold not discarded prematurely, no new worker claim. Repeated cancel is idempotent and cannot append unbounded duplicate intent/events. Concurrent late pause cannot override a recorded cancel. Wrong tenant/owner/revision still fails. No external send on cancel.

## Task 1 — persist cancel over pending pause

1. RED: test pause requested while intent unresolved; call CancelRun and Workbench fenced cancel on the pending Run; resolve journal; currently final state is waiting_user. Add repeated cancel and pause-after-cancel cases.
2. GREEN: let `cancelRunTx` record cancel even when already reconciling. Adjust helper CAS/event logic so latest durable intent is cancel and retry is idempotent. `resolveCraftChargePendingTransition` must prioritize cancel over an earlier pause; document why timestamp/sequence order cannot lose cancel. Preserve hold until journal resolution and session slot until final cancel.
3. Run focused repo/Workbench tests (coordinate test resource with RunView agent), full repository if stable, diff check and exact source/hash/checkpoint/report. Independent re-review before T19 Task 2 accepted.
