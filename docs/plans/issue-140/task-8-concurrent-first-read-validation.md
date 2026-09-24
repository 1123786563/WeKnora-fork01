# T08 Concurrent First-Read Fix Validation

- **Result:** PASS
- **Validated code commit:** `e37dcb2e860dd2d8be6d402fb7ab98531064f07f`
- **Checkout HEAD:** `34f5e85fe1dbe59d3e8dd59dbd37db28b0b84e4a`
- **Revision relationship:** code commit is an ancestor of HEAD. `git diff --name-status e37dcb2e860dd2d8be6d402fb7ab98531064f07f..HEAD` contains only `A .superpowers/sdd/2026-09-24-issue-140-implementation/task-8-concurrent-first-read-report.md`; no business or test source differs from the target code. Initial worktree status was clean. This validation report is the only file written.

## Results

- Concurrent same-ID recovery: PASS, `-count=10`. The regression fails callers on any raw database error; callers may return a receipt or typed `OutcomeUnknownError` with the request ID. Exact-intent retry converges on one stored snapshot. The code recognizes only SQLite BUSY/LOCKED first receipt read errors as potentially transient and reconciles through the bounded, scoped receipt lookup.
- Permanent initial receipt query failure: PASS in focused and race runs. The injected non-BUSY database error is returned as the original error before any opportunity, snapshot, or receipt is created; it is not mislabeled outcome unknown.
- Post-write ambiguity: PASS. Cancellation/rollback and simulated lost commit acknowledgement tests preserve typed recovery and return the persisted receipt when present.
- HTTP 413: PASS. Oversized body is rejected before binding and no opportunity or receipt is persisted.
- Broader Career/router/database/handler/container suites: PASS. Container produced the existing non-failing duplicate `-lc++` linker warning.
- `git diff --check` across code SHA to HEAD: PASS.

No acceptance gap was found in this fix scope. Verification uses SQLite-backed tests; this change does not alter PostgreSQL or schema behavior.

## Exact commands

```text
git rev-parse HEAD
git status --short
git show -s --format='%H %s' e37dcb2e860dd2d8be6d402fb7ab98531064f07f
git log --oneline --decorate -10
cat .superpowers/sdd/2026-09-24-issue-140-implementation/task-8-concurrent-first-read-report.md
go test -count=10 ./internal/modules/career -run '^TestCareerOpportunityConcurrentSameRequestReconcilesOneReceipt$'
go test -count=1 ./internal/modules/career -run 'TestCareerOpportunity(FirstReceiptReadFailureReturnsOriginalDatabaseError|ConcurrentSameRequestReconcilesOneReceipt|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt|ImportReplaysImmutableJDAndDoesNotChangeProfileRevision|HTTPRejectsOversizedBodyBeforeBinding)'
go test -race -count=1 ./internal/modules/career -run 'TestCareerOpportunity(ConcurrentSameRequestReconcilesOneReceipt|FirstReceiptReadFailureReturnsOriginalDatabaseError|AmbiguousCancelledWriteReturnsRecoverableOutcome|AmbiguousCommitReturnsPersistedReceipt)'
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
git diff --check e37dcb2e860dd2d8be6d402fb7ab98531064f07f..HEAD
git diff --name-status e37dcb2e860dd2d8be6d402fb7ab98531064f07f..HEAD
git status --short
```
