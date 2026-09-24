# T19 session-delete cancellation event tests: independent scoped review

Date: 2026-09-23. Read-only review of `t19-session-delete-event-triage.md`, the test plan/report/checkpoint, approved Craft Spec #107/#138, and the one-file test delta in the integration worktree. HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No source/test edits, staging, delegation or OCR.

## Exact checkpoint and verdict

| Evidence | SHA-256 |
| --- | --- |
| `internal/application/repository/agent_run_lifecycle_test.go` | `ed754e9a235438d3976c96304f19321920fee613874fe32ddced6e6dc2e34727` |
| `docs/plans/2026-09-23-craft-107-t19-session-delete-event-test-checkpoint.patch` | `4adda33b6f55d2e19aaa3d81d0503608a0800548dfa70b72a0015e33102245d3` |
| `docs/plans/2026-09-23-craft-107-t19-session-delete-event-test-checkpoint.json` | `404012f47e9fb52ba574f53df07ccb87184bd2949e160ca05c92da007be2a2a1` |

The live test hash matches the checkpoint manifest and implementation report. The diff changes only this test file; production cancellation code and existing test assertions outside these two cases are unchanged.

- **Scoped Spec compliance: PASS.** A Run already terminally canceled keeps its one durable `cancellation_requested` event when its Session is deleted; deletion still tombstones the Session, leaves the Run canceled, preserves the stale worker fence, and releases the active slot. The new pending-pause test verifies deletion requests cancellation while a charge start remains unresolved, retains the slot/journal, and resolves to canceled after the start outcome becomes known.
- **Scoped code quality: PASS.** The changed expectation reflects the existing idempotent `cancelRunTx` terminal path, rather than weakening production behavior to create a redundant event. The added regression asserts status, revision, event counts, wait reason, slot timing, journal state, lease clearance, and tombstone. No scoped finding was identified.
- **Full T19/repository suite: NOT VERIFIED by this checkpoint.** The focused tests pass; a coordinated full repository package run and PostgreSQL journal/lock evidence remain separate gates.

## Evidence and tests

`cancelRunTx` returns immediately when it sees `status='canceled'` (`agent_run_lifecycle.go:23-36`), while `DeleteSessionRuns` writes the deletion barrier and then routes each Run through that same cancellation transition (`:95-131`). The corrected test keeps the original cancellation payload assertion, expects one event after deletion, and verifies terminal revision unchanged, stale `SetStatus` denial, empty active slot and tombstone revision (`agent_run_lifecycle_test.go:21-51`). This aligns the event count with the reviewed idempotent terminal behavior.

The new case creates an unresolved charge-start journal and budget pause, then deletes the Session (`agent_run_lifecycle_test.go:54-78`). It requires exactly one `craft_charge_pause_requested`, one `craft_charge_cancel_requested`, and one `cancellation_requested`; the Run remains `reconciling`, the slot remains held and the journal remains `intent` (`:79-90`). After the test marks the journal `started`, `Claim` must finalize rather than lease the Run; the Run becomes canceled with `session_deleted`, lease cleared, slot released, journal retained as started, and cleanup tombstoned (`:92-108`). This meaningfully covers the pending-pause → delete → resolve order left open in the previous T19 review. It does not assert a second deletion is idempotent; the triage/report note the deletion barrier revision may advance on replay.

I independently ran `go test ./internal/application/repository -run 'TestCancelRunReleasesSlotAndDeleteFences|TestDeleteSessionRunAfterPendingPausePersistsCancelUntilChargeStartResolves' -count=1` and `go test ./internal/application/repository -run 'Test(CancelRun|DeleteSession|CraftCharge)' -count=1`; both passed. `git diff --check` on the one test file passed. These SQLite tests do not establish PostgreSQL concurrency or full application cancellation behavior.
