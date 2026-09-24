# T10 Graduation Block Fix Independent Validation

- **Result:** DONE
- **Fix under test:** `d70792a3e1e5b5d29ca2386452489089322205af`
- **Worktree HEAD during validation:** `222f78a7bb8a8152c61f8b6e09e84f55c0fe07e5` (report-only commit after the requested code SHA)
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-evaluation/WeKnora-fork01`
- **Scope:** Multi-line/CRLF graduation requirement classification, separately labeled skill sections, replay/concurrency regression, broad backend regression and SQLite migration. No source or test files modified during validation.

## Commands and results

1. `git rev-parse HEAD` → `222f78a7bb8a8152c61f8b6e09e84f55c0fe07e5`; `git status --short` → clean before writing this ignored `.superpowers` report.
2. `go test -count=10 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` → PASS, all 10 repetitions.
3. `go test -race -count=1 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` → PASS, no race reported.
4. `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` → PASS across all packages. Existing linker warning: `ignoring duplicate libraries: '-lc++'`.
5. `go test -count=1 ./internal/database -run '^TestCareerEvaluationSQLiteMigrationUpDownUp$'` → PASS; SQLite migration 116 down/up cycle preserves prior Opportunity tables and restores evaluation schema.
6. `git diff --check d70792a3e1e5b5d29ca2386452489089322205af^ d70792a3e1e5b5d29ca2386452489089322205af` → PASS.

## Acceptance evidence

- Focused cases cover wrapped alternatives before and after the recognized clause, a second graduation year on a following line, negation on the preceding line and directly before the clause, punctuation-separated year lists, and CRLF-wrapped alternatives. These all remain `unknown` without profile evidence.
- The standalone recognized graduation condition still produces the correct mismatch result when followed by a clearly labeled skill section on a separate LF or CRLF line. The focused case asserts the cited raw-text range and profile evidence.
- Replay after a profile edit continues to return the stored original receipt/evaluation; a new request captures the new profile revision and the old evaluation remains immutable.
- A synchronized concurrent changed-intent request race continues to return HTTP 409 `idempotency_conflict` to the losing intent.
- No acceptance gap was found in this repair scope.

## Risks / limitations

- The parser deliberately recognizes a small set of labeled skill/project section boundaries. Other unrecognized neighboring text is conservatively treated as part of the graduation requirement block and yields `unknown`.
- PostgreSQL runtime testing remains unavailable from the prior validation because `TRPC_TEST_POSTGRES_DSN` was unset; this repair does not modify migration SQL or persistence behavior.
- This report validates this parser fix only; it does not replace independent code review or complete T10/#150 gates.
