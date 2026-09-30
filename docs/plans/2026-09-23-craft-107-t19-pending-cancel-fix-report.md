# T19 Pending Pause → Cancel Fix Report

Date: 2026-09-23. This implements the scoped Task 2 review correction described in `2026-09-23-craft-107-t19-pending-cancel-fix-plan.md`. No commit was created. This report does not close Task 2 review or T19/#138 until independent re-review and remaining T19 gates pass.

## Change

- `CancelRun` now records cancellation when the Run is already `reconciling/craft_charge_start_pending` due to an earlier pause. The transition increments revision once, records the cancel-specific and generic request events, and retains the unresolved journal/hold and active session slot.
- Repeated cancel is idempotent: the existing durable cancel request is detected and does not add a duplicate event or revision.
- Pending transition resolution gives any durable cancel request precedence over pause requests, even if a later stale pause event appears in the event stream. A stale worker's fenced pause write still fails after cancel clears its lease.
- The focused regression exercises pause→owner/revision-cancel, wrong owner and stale revision rejection, repeated cancel, resolution to confirmed cancellation, evidence retention, cancel→late pause, and cancel-over-later-pause precedence.
- `agent_run.go` includes concurrent T01 admission changes from the integrated worktree. They were preserved; the checkpoint contains full bytes and a task delta against the prior Task 2 checkpoint.

## TDD and verification

- RED: before implementation, `go test ./internal/application/repository -run '^TestCraftChargePendingPauseThenCancelWinsAndRepeatsIdempotently$' -count=1` failed because the acknowledged cancel did not advance the pending Run revision (expected 3, actual 2); it was silently treated as an idempotent request. The later-resolution assertion would therefore resolve the earlier pause.
- GREEN: `go test ./internal/application/repository -run 'TestCraftCharge|TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun' -count=1` passed after the fix. Output: `repository-focused.log`.
- GREEN: `go test ./internal/modules/workbench/service/workbench -run '^TestGormCancelPortDefersUnresolvedCraftStart$' -count=1` passed. Output: `workbench-focused.log`.
- `git diff --check -- internal/application/repository/agent_run.go internal/application/repository/agent_run_lifecycle.go internal/application/repository/agent_run_craft_charge_test.go` passed.
- No full repository suite was rerun: the prior Task 2 full package attempt was interrupted after 145.487 seconds during concurrent repository validation and is not a passing baseline. PostgreSQL remains untested because `TRPC_TEST_POSTGRES_DSN` is unset.

## Files and checkpoint

Changed source paths:

- `internal/application/repository/agent_run.go`
- `internal/application/repository/agent_run_lifecycle.go`
- `internal/application/repository/agent_run_craft_charge_test.go`

Checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t19-pending-cancel-fix-checkpoint-01/`. It contains pre-fix full source copies from the preceding Task 2 checkpoint, post-fix source bytes, before/after checksums, exact task delta patch, HEAD and complete worktree status, and targeted test logs. No Workbench, Task 1 budget service, T08, T05, gateway, or sandbox files were changed.

## Remaining review scope

Independent re-review is pending. The separate direct budget pause bypass identified by review remains outside this correction and requires its own approved follow-up. Production model and sandbox callers and PostgreSQL locking evidence also remain open; T19 is not complete.
