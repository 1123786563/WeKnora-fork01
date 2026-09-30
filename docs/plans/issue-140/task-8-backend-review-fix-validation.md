# T08 Backend Review Fix Validation

- **Result:** PASS with environment note
- **Requested code SHA:** `b1eba2892bcca03b0737258034dd492ade177630`
- **Checkout HEAD:** `0f1fa8d4fde3dca554767026c3e56008c342285b`
- **HEAD relationship:** requested SHA is an ancestor. `git diff --name-status b1eba2892bcca03b0737258034dd492ade177630..HEAD` contains only added `.superpowers/sdd/2026-09-24-issue-140-implementation/task-8-backend-review-fix-report.md`; no code or test source differs from the requested SHA. Initial worktree status was clean. This validation report is the only file written.

## Results

All checks below were run on HEAD with the source tree equal to requested code SHA.

1. **Oversized request before binding:** PASS. `TestCareerOpportunityHTTPRejectsOversizedBodyBeforeBinding` returned HTTP 413 with `request_too_large` and verified no opportunity or receipt row was created. Inspection confirms `MaxBytesReader` wraps the request body before Gin JSON binding, with a bounded envelope allowance; Office separately enforces decoded raw text size.
2. **Same request ID under concurrency:** PASS. Eight synchronized SQLite callers either obtain the stored receipt or typed `outcome_unknown`; re-POST of identical intent converges to one receipt and exactly one snapshot. The test passed five repeated runs.
3. **Cancellation and ambiguous commit acknowledgement:** PASS. Cancellation after receipt insertion followed by a simulated transaction error returns typed `OutcomeUnknownError` with request ID; scoped lookup confirms that rolled-back receipt is absent, allowing safe retry. Simulated lost acknowledgement after commit reconciles to the exact persisted receipt and one snapshot. Reconciliation uses a bounded timeout context detached from request cancellation but retains scope values, checks the scoped fingerprint, and returns `outcome_unknown` when it cannot establish an outcome. HTTP maps that to 504 and includes request ID.
4. **Focused behavior suite:** PASS.
5. **Broader requested suites:** PASS for Career, router, database, handler, and container packages. Container emitted linker warning `ignoring duplicate libraries: '-lc++'` and passed.
6. **Diff whitespace:** `git diff --check b1eba2892bcca03b0737258034dd492ade177630..HEAD` passed.

## Exact commands

```text
git rev-parse HEAD
git status --short
git show -s --format='%H %s' b1eba2892bcca03b0737258034dd492ade177630
git diff --name-status b1eba2892bcca03b0737258034dd492ade177630..HEAD
cat .superpowers/sdd/2026-09-24-issue-140-implementation/task-8-backend-review-fix-report.md
rg -n 'MaxBytes|request_too_large|OutcomeUnknown|WithoutCancel|reconcil|ConcurrentSameRequest|AmbiguousCancelled|AmbiguousCommit|oversized' internal/modules/career/handler.go internal/modules/career/opportunity.go internal/modules/career/opportunity_test.go
go test -count=1 ./internal/modules/career -run 'TestCareerOpportunity(HTTPRejectsOversizedBodyBeforeBinding|ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|ImportReplaysImmutableJDAndDoesNotChangeProfileRevision)'
go test -count=5 ./internal/modules/career -run '^TestCareerOpportunityConcurrentSameRequestReconcilesOneReceipt$'
go test -race -count=1 ./internal/modules/career -run 'TestCareerOpportunity(ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt)'
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
git diff --check b1eba2892bcca03b0737258034dd492ade177630..HEAD
git status --short
```

PostgreSQL was not rechecked for this fix because no PostgreSQL migration or schema changed; the earlier report records that its DSN was unavailable. No backend acceptance gap was found in the assigned fix scope.
