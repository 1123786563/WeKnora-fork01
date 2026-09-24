# T08/#146 Backend Review Fix Report

- **Plan:** `docs/plans/2026-09-24-issue-140-t08-backend-review-fixes.md` in the integration worktree.
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t08-jd/WeKnora-fork01`
- **Original T08 implementation:** `7a29b3336068f10d82d850e9f793db1d14a0970e` (preserved).
- **Review-fix code commit:** `b1eba2892bcca03b0737258034dd492ade177630`.
- **Scope:** both assigned medium findings; no migration, router, Web, TypeScript, or architectureguard changes.

## Fixes

1. **Bound import JSON before binding.** `Handler.ImportJD` wraps the request body with `http.MaxBytesReader` before Gin JSON binding. The cap is `6 * 1 MiB + 64 KiB`: enough for worst-case JSON string escaping of the existing 1 MiB raw-text limit and bounded source metadata/envelope. Oversized bodies return HTTP 413 with `request_too_large`; the Office still independently enforces the exact 1 MiB decoded raw-text limit.
2. **Make uncertain writes recoverable.** After any transaction error other than an already-known idempotency conflict, the Office checks the scoped receipt using a timeout context derived with `context.WithoutCancel`. It makes at most four receipt reads over a 350 ms window, with 20/40/80 ms backoff. A matching fingerprint returns the stored receipt; a changed fingerprint returns `idempotency_conflict`; a missing or unreadable receipt returns `OutcomeUnknownError` with the original request ID. The existing HTTP mapping exposes this as 504 `outcome_unknown`, including `requestId`. Retrying the same POST after an unknown result returns the receipt if committed, or safely creates one if the failed transaction rolled back.

## RED / GREEN evidence

- **RED:** `go test ./internal/modules/career -run 'TestCareerOpportunity(HTTPRejectsOversizedBodyBeforeBinding|ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome)' -count=1` — failed as intended: oversized body returned generic 400 instead of 413; concurrent writes exposed raw SQLite `database table is locked`; a canceled write returned the injected raw transaction error instead of a request-ID recovery result.
- **GREEN focused repeat:** `go test -count=5 ./internal/modules/career -run '^TestCareerOpportunityConcurrentSameRequestReconcilesOneReceipt$'` — PASS. Test synchronizes all callers after scope verification, exercises concurrent SQLite transaction contention, accepts successful receipts or typed `outcome_unknown`, retries the same request ID, and asserts one persisted snapshot.
- **GREEN race checks:** `go test -race -count=1 ./internal/modules/career -run 'TestCareerOpportunity(ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt)'` — PASS. Includes cancellation after receipt insertion and simulated lost commit acknowledgement; the latter confirms reconciliation returns the exact persisted receipt.
- **Focused behavior:** `go test -count=1 ./internal/modules/career -run 'TestCareerOpportunity(HTTPRejectsOversizedBodyBeforeBinding|ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|ImportReplaysImmutableJDAndDoesNotChangeProfileRevision)'` — PASS.
- **Requested Go suites:** `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — PASS.
- **Diff validation:** `git diff --check` and `git diff --cached --check` — PASS.
- The container test emitted the macOS linker warning `ignoring duplicate libraries: '-lc++'`; the package passed.

## SQLite contention and recovery behavior

With eight concurrent callers against SQLite shared-cache memory, SQLite may roll back every overlapping write transaction with `SQLITE_LOCKED`. Such calls now return `outcome_unknown` instead of a raw database error. The same-ID re-POST after contention is tested to converge on one receipt and exactly one snapshot. This retains the plan’s finite reconciliation bound; callers should use the returned request ID for receipt lookup and retry the exact intent if no receipt is yet visible.

PostgreSQL integration was not run for this review-fix turn. The changes do not alter the versioned SQL migration or transaction schema.
