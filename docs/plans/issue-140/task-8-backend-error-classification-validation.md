# T08 Backend Error Classification Fix Validation

- **Result:** PASS
- **Requested code commit:** `8e54ac44c6379077a641aacd563c0f11decbbb0a`
- **Validated HEAD:** `76b367f571b942716bcf5bc2fe81bf7131740484`
- **Revision relationship:** requested commit is an ancestor. `git diff --name-status 8e54ac44c6379077a641aacd563c0f11decbbb0a..HEAD` contains only `A .superpowers/sdd/2026-09-24-issue-140-implementation/task-8-backend-error-classification-report.md`. No implementation or test source changed after the requested commit. Initial worktree status was clean; this validation report is the only file written.

## Findings and checks

- Injected initial scoped receipt SELECT failure is returned as the original database error, not `outcome_unknown`; assertions confirm the failure occurs before any opportunity, snapshot, or receipt is written. Classification changes to conservative bounded receipt reconciliation immediately before the first INSERT attempt. Errors after that point, and simulated post-commit acknowledgement loss, remain recoverable via the scoped receipt and same-ID retry.
- Oversized HTTP body returns 413 before JSON binding and writes no rows.
- Eight synchronized same-ID callers followed by exact-intent retry converge on one persisted snapshot. Repeated five times successfully.
- Cancellation after receipt insertion with simulated transaction rollback returns typed outcome unknown and request ID; receipt lookup confirms absence. Simulated lost commit acknowledgement returns/reconciles the stored receipt and exactly one snapshot.
- No acceptance gap found in the assigned error-classification fix. The prior PostgreSQL caveat remains: this fix does not change migrations or schema; no PostgreSQL execution is claimed.

## Verification

All commands ran on HEAD whose code/test source matches the requested SHA.

```text
go test -count=1 ./internal/modules/career -run 'TestCareerOpportunity(FirstReceiptReadFailureReturnsOriginalDatabaseError|HTTPRejectsOversizedBodyBeforeBinding|ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|ImportReplaysImmutableJDAndDoesNotChangeProfileRevision)'
go test -race -count=1 ./internal/modules/career -run 'TestCareerOpportunity(ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|FirstReceiptReadFailureReturnsOriginalDatabaseError)'
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
go test -count=5 ./internal/modules/career -run '^TestCareerOpportunityConcurrentSameRequestReconcilesOneReceipt$'
git diff --check 8e54ac44c6379077a641aacd563c0f11decbbb0a..HEAD
git diff --name-status 8e54ac44c6379077a641aacd563c0f11decbbb0a..HEAD
git status --short
```

Focused, race, repeated concurrency, and broader Career/router/database/handler/container suites all passed. The broader run emitted the existing linker warning `ignoring duplicate libraries: '-lc++'`; container tests passed.
