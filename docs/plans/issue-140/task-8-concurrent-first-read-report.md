# T08 concurrent first-read recovery report

## Scope and result

Implemented `2026-09-24-issue-140-t08-concurrent-first-read.md` in the isolated T08 worktree. When the initial scoped receipt SELECT returns SQLite BUSY/LOCKED, `ImportJD` now routes that failure through the existing bounded (350 ms) scoped receipt reconciliation. A nontransient first-read failure remains an ordinary database error. No retry loop, schema, migration, route, or wire changes were added.

The concurrency test now rejects raw database errors and accepts only success or typed `OutcomeUnknownError` with the request ID. The permanent injected first-receipt-query failure test remains and passes.

## RED / GREEN and verification

RED, after tightening the concurrency assertion and before the classification change:

```text
go test -count=10 ./internal/modules/career -run '^TestCareerOpportunityConcurrentSameRequestReconcilesOneReceipt$'
```

Failed as expected on a concurrent run with raw `database table is locked` reaching the caller.

GREEN repeat:

```text
go test -count=10 ./internal/modules/career -run '^TestCareerOpportunityConcurrentSameRequestReconcilesOneReceipt$'
```

Passed.

Additional checks, all passed:

```text
go test -count=1 ./internal/modules/career -run 'TestCareerOpportunity(FirstReceiptReadFailureReturnsOriginalDatabaseError|ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|ImportReplaysImmutableJDAndDoesNotChangeProfileRevision|HTTPRejectsOversizedBodyBeforeBinding)'
go test -race -count=1 ./internal/modules/career -run 'TestCareerOpportunity(ConcurrentSameRequestReconcilesOneReceipt|FirstReceiptReadFailureReturnsOriginalDatabaseError|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt)'
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
git diff --check
```

The full focused suite passed, including permanent first-read error handling and post-write ambiguity recovery. The broader Go suites passed. Container linking emitted the existing non-failing duplicate `-lc++` warning.

## Changed files and commit

Code commit: `e37dcb2e8e860dd2d8be6d402fb7ab98531064f07` (`Recover Career imports after transient receipt locks`). It changes only:

- `internal/modules/career/opportunity.go`
- `internal/modules/career/opportunity_test.go`

The fix reuses `isSQLiteBusy` and the already bounded `reconcileOpportunityReceipt` path. Permanent injected query failures do not match that category and are returned directly.

## Limits

Race and integration verification used the repository's SQLite-backed Career tests. No PostgreSQL-specific behavior was modified or newly claimed. If bounded reconciliation cannot observe a committed receipt, existing behavior returns typed `outcome_unknown`, preserving request-ID recovery semantics.
