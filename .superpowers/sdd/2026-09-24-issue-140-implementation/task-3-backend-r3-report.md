# T03 backend fix round 3 report

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t03/WeKnora-fork01`
- Base: `4ec5aafd4a11a115bde955ad7387221f873280bc`
- Finding: concurrent calls with the same scoped request ID could both miss the receipt and return a revision/SQLite lock failure rather than replaying the committed receipt.

## Changes

`Office.mutate` now resolves receipts after the failed transaction has rolled back for revision conflicts, duplicate-key/unique receipt races, and SQLite lock races. It compares the stored fingerprint before replay, so a changed payload returns `ErrIdempotencyConflict`. A short bounded retry handles the interval between a losing transaction rolling back and the winning transaction committing. When SQLite reports a lock for a distinct request ID, the committed profile revision is checked and returned as `RevisionConflictError`.

A private nil-by-default synchronization hook lets the test force both transactions to miss the receipt before either continues. The test opens two GORM/SQLite handles to one WAL database and checks identical request replay, changed payload conflict, and distinct-ID revision conflict. It also checks a single revision and history row after identical requests.

## Verification

- RED evidence: before bounded post-rollback resolution, the synchronized two-connection test intermittently failed with `database is locked`; the immediate receipt lookup ran before the winner committed and found no receipt.
- `go test ./internal/modules/career -run 'TestConcurrentSameRequestIDReplaysCommittedReceiptAcrossConnections|TestRevisionConflictHasCurrentValueAndIdempotency' -count=10` — pass.
- `go test ./internal/modules/career ./internal/database -count=1` — pass.
- `go test -race ./internal/modules/career -run TestConcurrentSameRequestIDReplaysCommittedReceiptAcrossConnections -count=3` — pass.
- `git diff --check` — pass.
- `pnpm exec tsx --test packages/career-core/src/contracts.test.ts` — not run: `tsx` executable is unavailable in this worktree. No TypeScript contract files changed in this round.

## Scope and remaining risk

Changed Go files only: `internal/modules/career/office.go` and `internal/modules/career/office_test.go`. Migrations and the TS contract are unchanged. The overlap test uses two SQLite connections and deterministic synchronization; PostgreSQL runtime behavior was not exercised in this environment. The production retry window is bounded at 200 ms and respects request cancellation.
