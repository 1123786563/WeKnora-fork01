# T19 Session Deletion Event Contract Test Report

**Status:** Updated terminal-cancel event expectation and added pending-start deletion coverage. No production changes were needed.

**Worktree / baseline:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`, HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No commit or staging. Only `internal/application/repository/agent_run_lifecycle_test.go` was changed in source/tests.

## Changes

- `TestCancelRunReleasesSlotAndDeleteFences` now expects one `cancellation_requested` event after cancel then session deletion. It checks that the terminal Run revision is unchanged, the canceled Run and stale-worker fence remain, the session slot is clear, and the tombstone is recorded at `terminal Run revision + 1`.
- Added `TestDeleteSessionRunAfterPendingPausePersistsCancelUntilChargeStartResolves`: creates an unresolved journal, pauses the Run, deletes its session, and verifies the durable pause/cancel/cancellation events, retained slot and journal while unresolved. After marking the journal `started`, recovery finalizes to canceled with `session_deleted`, releases the slot, and retains the resolved journal and tombstone.
- Tombstones intentionally advance `deletion_revision` on each deletion call, so the new assertions cover the first deletion only and do not characterize repeat deletion as idempotent.

## RED/GREEN and verification

The triage report documented the existing full-package assertion failure: the old test expected a second cancellation event after a Run was already terminal. That is the RED evidence for the obsolete expectation. After changing it to one, both focused scenarios passed. The newly added pending-start path also passed against current production code, confirming the durable cancel precedence behavior is already implemented; no production defect was found, so no code change or broader fix plan was warranted.

Commands and results:

```text
gofmt -w internal/application/repository/agent_run_lifecycle_test.go
PASS

go test ./internal/application/repository -run 'TestCancelRunReleasesSlotAndDeleteFences|TestDeleteSessionRunAfterPendingPausePersistsCancelUntilChargeStartResolves' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/repository 10.798s

go test ./internal/application/repository -run 'Test(CancelRun|DeleteSession|CraftCharge)' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/repository 25.443s

git diff --check -- internal/application/repository/agent_run_lifecycle_test.go
PASS
```

## Checkpoint and limits

Exact full-content patch and hashes are recorded in `2026-09-23-craft-107-t19-session-delete-event-test-checkpoint.json`; its source/test delta is relative to the recorded HEAD. A full repository package rerun remains with the parent after separately owned active-member fixtures settle. No independent review is claimed here.
