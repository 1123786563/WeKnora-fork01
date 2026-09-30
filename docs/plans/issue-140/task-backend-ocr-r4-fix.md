# #140 Career 后端 OCR round-4 修复报告

- Assigned baseline: `76df0cee0bf3ae23c14411c345b151ad518077ee`。
- Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`。
- Initial HEAD check: `76df0cee0bf3ae23c14411c345b151ad518077ee`。
- Current HEAD at report time: `bc19b9fd36d473ce3e3afd8a0c11f0067634a2af`; concurrent commits `f35ed2427` and `bc19b9fd3` landed during this task. Backend changes listed below remain uncommitted on top of that HEAD.
- Scope: only the assigned Career backend source, corresponding tests, and this report.

## Changes

- Corrected the search-rule deletion boundary: search runs and discovery todos are explicitly described as excluded from export and unrecoverable after deletion.
- Sanitized only unmapped `500/internal` responses to the fixed public message `internal career office error`; the original error is logged server-side.
- When clearing a failed upload resource loses its claim, reloads the owner-scoped latest source and returns it as superseded, matching the earlier claim-loss path.
- Serializes same-source reminder dedupe checks after the profile head lock. For concurrent SQLite lock or unique-key races, performs bounded owner-scoped source reconciliation and persists a deduplicated receipt for the losing request ID.

## TDD and verification evidence

- RED command: `go test ./internal/modules/career -run 'TestCareerDeletionBoundaryDoesNotPromise|TestHTTPUnknownErrorDoesNotExpose|TestFinishClaimFailureTreatsCleanupClaimLoss|TestConcurrentSetReminderDifferentRequestIDs' -count=1` — failed as expected before implementation: boundary text still promised inclusion, unknown 500 exposed storage endpoint details, cleanup claim loss returned `ErrUploadClaimLost`, and concurrent SQLite requests returned `outcome_unknown` after lock errors.
- GREEN command: same focused command after changes — PASS (`ok github.com/Tencent/WeKnora/internal/modules/career 0.835s`).
- `go test ./internal/modules/career -count=1` — PASS (`ok ... 20.587s`). Includes handler contract and career module tests.
- `go test -race ./internal/modules/career -run 'TestConcurrentSetReminderDifferentRequestIDsConvergeOnOneTodo|TestFinishClaimFailureTreatsCleanupClaimLossAsSuperseded' -count=1` — PASS (`ok ... 4.413s`).
- `git diff --check` — PASS (exit 0).
- Concurrency integration evidence is SQLite-backed. No PostgreSQL service was started, so PostgreSQL lock behavior was not separately exercised; the profile row lock ordering and source-key reconciliation are covered by code path and SQLite concurrent regression.

## Review state

- Awaiting parent incremental review. No commit created.
