# #140 Task 3 reminder deletion race fix

- Finding: `docs/plans/issue-140/task-backend-ocr-r4-review.md` reports that `reconcileReminderSource` can read a live reminder, race with `DeleteCareer`'s final purge, and insert a deduplicated receipt after the deletion receipt claims completion.
- Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`.
- BASE / starting HEAD: `50932d609f3a81666ec6efd5d6ae8ba8ebefae64`.
- Integration HEAD when this task's commit was prepared: `8c58a9a27d3dbd04aa6b41836a121c4f17669eb9`; unrelated integration commits landed during the task, and the owned diff was rebased by sharing the same worktree.
- Scope: `internal/modules/career/reminder.go`, direct tests in `internal/modules/career/reminder_test.go`, and this report. `career_export_test.go` unchanged.

## Change

`reconcileReminderSource` now selects the scoped `career_profiles` row with `FOR UPDATE` at the start of its transaction, matching `attemptReminderWrite` and the final deletion purge lock. Only after acquiring that lock does it check for an existing request receipt or read the source reminder. If the profile no longer exists, it returns `found=false` without creating a receipt. Thus, after deletion has finalized, reconciliation cannot return a live deduplicated reminder or recreate its receipt.

Added two tests:

- `TestReconcileReminderSourceDoesNotRecreateReceiptAfterDeletion` deletes a space and then attempts reconciliation; it asserts no reminder or reminder receipt remains and no live receipt is returned.
- `TestDeleteFinalizingDuringReminderReconciliationLeavesNoRows` pauses reconciliation immediately after its source lookup using a GORM callback/barrier, runs deletion concurrently, resumes reconciliation, ensures deletion reaches `deleted`, and asserts both reminder tables are empty.

## Verification

- `go test -race ./internal/modules/career -run 'Test(ReconcileReminderSourceDoesNotRecreateReceiptAfterDeletion|DeleteFinalizingDuringReminderReconciliationLeavesNoRows|ConcurrentSetReminderDifferentRequestIDsConvergeOnOneTodo)$' -count=1` — PASS.
- `go test ./internal/modules/career -count=1` — PASS (`ok ... 24.123s`).
- `git diff --check` — PASS.

The barrier test uses file-backed SQLite in WAL mode. SQLite ignores GORM's `FOR UPDATE` locking clause and may reject a write from the stale reconciliation snapshot with `SQLITE_BUSY`; the test accepts that database outcome and verifies the final deletion invariant. It does not establish PostgreSQL row-lock behavior experimentally. In PostgreSQL, the new profile-row lock makes reconciliation and the final purge serialize: whichever acquires the lock first commits first, and the final purge removes any receipt reconciliation committed before deletion.

## Review scope and remaining risk

No schema, API contract, authentication scope, or cancellation behavior changed. Queries remain tenant/user scoped. No PostgreSQL test DSN was supplied or exercised, so live PostgreSQL concurrency behavior remains unverified by this task; the lock order is aligned with the existing deletion and normal reminder-write paths.
