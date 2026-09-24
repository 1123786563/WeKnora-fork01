# T19 pending pause → cancel fix: independent review

Date: 2026-09-23. Read-only review of the exact delta in `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-pending-cancel-fix-checkpoint-01/task-delta.patch`, against the approved Craft Spec, `CONTEXT.md`, ADR-0004, T19 durable-start Task 2 brief, the earlier run-transition review, and the pending-cancel fix plan/report. HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No OCR was invoked.

## Verdict

- **Scoped Spec compliance: PASS.** An authenticated, revision-fenced cancel following an unresolved-start budget pause now records a durable cancel request. Resolution selects cancel over pause and clears the session slot only after the charge-start journal resolves. Repeated cancel keeps one request and one revision advance. The implementation preserves the unresolved journal/hold and prevents the stale worker pause from downgrading cancel.
- **Scoped code quality: PASS, with one low test-coverage finding below.** The Run-row lock serializes the check for an existing cancel request with its insertion. Both request events are written in the same transaction as the status/revision change. The resolver explicitly gives cancel precedence, avoiding reliance on which request has the latest sequence.
- **Full T19: NOT VERIFIED.** The earlier review's direct `CraftBudgetService.pauseCraftRunTx` journal-fence bypass remains outside this patch. Model/sandbox production initiation, PostgreSQL locking evidence, and full validation are also still open.

## Low 1 — user-facing and bulk-delete ordering lacks a focused regression

**Evidence / affected tests:** The new `TestCraftChargePendingPauseThenCancelWinsAndRepeatsIdempotently` calls `CancelRunOwnedAtRevision` directly (`internal/application/repository/agent_run_craft_charge_test.go:103-148`). The Workbench test `TestGormCancelPortDefersUnresolvedCraftStart` cancels a `running` Run, not one already carrying a pending pause (`internal/modules/workbench/service/workbench/interaction_test.go:198-240`). No new test sends `DeleteSessionRuns` after that pause, although both paths were explicit in the fix plan and prior finding.

**Impact:** The shared repository transition is tested, and the Workbench adapter delegates to it, so there is no demonstrated product defect. The exact user-facing and bulk-delete orderings could regress without a targeted test.

**Smallest correction:** Add one Workbench adapter test for pause → revision-fenced cancel and one session-deletion test for pause → delete → journal resolution, asserting final canceled state, active-slot release, and retained journal evidence. This is follow-up test coverage, not a reason to reject the scoped code fix.

## Evidence and limits

Live SHA-256 values matched all three `after.sha256` checkpoint values: `agent_run.go` `f15bd6b76189d47f4ddca7732942cc79d40dfd1613a378aa5d3099a5507c53e1`, `agent_run_lifecycle.go` `637bb9f132f1524024f5f525f6ede4e58191ade536f1ed82781fe55bdfd4d8d2`, and `agent_run_craft_charge_test.go` `b4cabc02166c53971ac29912c2c008a38e63cbcdf9be6a6ca5fcfef169f3d60a`. The delta changes only these files. Other concurrent integration-worktree edits were not attributed to this fix.

I independently ran the two new repository tests and the focused Workbench cancel test; both package commands passed. `git diff --check` on the three scoped files passed. These SQLite tests establish the focused behavior but do not establish PostgreSQL concurrency safety. I did not run the full repository suite or OCR.
