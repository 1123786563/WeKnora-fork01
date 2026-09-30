# T08 backend error classification fix report

## Scope and decision

Implemented the approved plan `2026-09-24-issue-140-t08-backend-error-classification.md` in the T08 isolated worktree. A database error on the initial idempotency receipt SELECT happens before any persistence write is attempted, so `ImportJD` now returns that original error. After the first possible insert, transaction errors continue through bounded, scoped receipt reconciliation and retain the existing `outcome_unknown` behavior when the commit outcome cannot be established.

No wire or schema contract changed. The concurrent same-request regression permits SQLite's definite pre-write `SQLITE_BUSY` error in addition to the existing ambiguous outcome; retrying the same exact request still produces one persisted snapshot.

## RED / GREEN evidence

RED, before the implementation change:

```text
go test -count=1 ./internal/modules/career -run '^TestCareerOpportunityFirstReceiptReadFailureReturnsOriginalDatabaseError$'
```

Failed as expected: the injected first receipt SELECT error was reported as `career request outcome unknown` instead of the injected database error.

GREEN and regression checks, after the change:

```text
go test -count=1 ./internal/modules/career -run 'TestCareerOpportunity(FirstReceiptReadFailureReturnsOriginalDatabaseError|HTTPRejectsOversizedBodyBeforeBinding|ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|ImportReplaysImmutableJDAndDoesNotChangeProfileRevision)'
go test -count=5 ./internal/modules/career -run '^TestCareerOpportunityConcurrentSameRequestReconcilesOneReceipt$'
go test -race -count=1 ./internal/modules/career -run 'TestCareerOpportunity(ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|FirstReceiptReadFailureReturnsOriginalDatabaseError)'
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
git diff --check
```

All commands passed. The broader suite emitted a linker warning that duplicate `-lc++` libraries were ignored; it did not fail the tests.

## Changed files and commits

Code commit: `8e54ac44c6379077a641aacd563c0f11decbbb0a` (`Classify Career JD pre-write failures`). It changes only:

- `internal/modules/career/opportunity.go`
- `internal/modules/career/opportunity_test.go`

This is an additive fix after the prior T08 commits; no prior commit was amended. This report is a separate evidence artifact and is not included in the code commit.

## Limits

Classification intentionally switches at the first opportunity insert attempt. Errors before that point are returned directly because no persistence write could have occurred. Errors at or after that point retain the existing bounded reconciliation behavior. Verification used the repository's SQLite-backed Career test setup; no PostgreSQL-specific behavior was changed or newly claimed.
