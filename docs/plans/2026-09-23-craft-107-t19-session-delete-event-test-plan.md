# T19 Session Deletion Event Contract Test Plan

> **For Codex:** Execute with SDD exact uncommitted checkpoint and independent narrow Review. No commit.

**Goal:** Reconcile one repository test's obsolete expectation of a second cancellation event after a Run is already terminal, and add meaningful coverage for deletion while a charge start is unresolved.

**Sources:** `t19-session-delete-event-triage.md`, T19 pending-cancel PASS review, `TestCancelRunReleasesSlotAndDeleteFences` failure in full repository log, approved #138 cancellation semantics.

**Global Constraints:** Test source only. Do not alter the reviewed idempotent production cancellation path merely to recreate a duplicate event. Preserve durable tombstone, Run row, slot fencing and pending journal semantics. The three active-member fixture files are separately owned by a mechanical worker; this task owns only `agent_run_lifecycle_test.go` and, for a new focused case if needed, `agent_run_craft_charge_test.go` after no other worker owns it.

**Review Focus:** already canceled Run has one cancellation_requested event; session deletion still records tombstone and leaves Run canceled/stale worker fenced. Unresolved pause/start followed by session deletion records a durable cancel request, retains slot until reconciliation, and resolves terminal canceled without a second provider start.

## Task 1 — update and extend observable cancellation tests

**Depends on:** read-only lifecycle triage and T19 budget-pause repository Task1 review PASS. **Role:** backend_implementer for the pending-journal behavioral case; mechanical_worker only if changing the one assertion. **Owned files:** `internal/application/repository/agent_run_lifecycle_test.go` and focused new test file only with separate ownership amendment. **Produces:** tests aligned to idempotent terminal cancellation.

1. RED: existing test fails because it expects two events after terminal cancel then DeleteSessionRuns; separately write unresolved-start pause→delete→resolve test to expose any missing cancellation request.
2. GREEN: expect one event for already terminal Run, assert tombstone/revision/slot/stale worker fence. Add the unresolved-start case without changing production code unless a real behavior defect is found, which requires a new fix plan and review.
3. Run focused lifecycle/charge tests, `git diff --check`, save exact full-content checkpoint and independent review; then central reruns full repository package after active-member fixtures settle.

**Failure handling:** If the new pending-journal case fails, report exact production behavior and keep T19 unverified; do not weaken the test to green.
