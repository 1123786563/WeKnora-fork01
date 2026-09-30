# T10 Backend Review Fix Independent Validation

- **Result:** DONE
- **Fix under test:** `ec8f0f2f60e0041b47ea1113f12d5729f3c8c299`
- **Worktree HEAD during validation:** `e8b4cdb851177ca2deef6a35cefecf6fb362b739` (report-only child commit; source tree contains the exact fix SHA)
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-evaluation/WeKnora-fork01`
- **Scope:** Alternative and negated graduation clauses, unsupported/no-citation cases, short Latin token boundaries (Go versus Google), concurrent changed-intent requests, regression suites and migration. No production/test source edits were made during validation.

## Commands and results

1. `git rev-parse HEAD` → `e8b4cdb851177ca2deef6a35cefecf6fb362b739`; `git status --short` → clean before this report (the `.superpowers` report directory is ignored by Git).
2. `go test -count=20 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationShortLatinSoftMatchRequiresASCIIWordBoundary|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` → PASS, all 20 repetitions.
3. `go test -race -count=1 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationShortLatinSoftMatchRequiresASCIIWordBoundary|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` → PASS, no race reported.
4. `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` → PASS for all packages. Existing linker warning: `ignoring duplicate libraries: '-lc++'`.
5. `go test -count=1 ./internal/database -run '^TestCareerEvaluationSQLiteMigrationUpDownUp$'` → PASS; version 116 down to 115 and back to 116 with evaluation columns restored.
6. `git diff --check ec8f0f2f60e0041b47ea1113f12d5729f3c8c299^ ec8f0f2f60e0041b47ea1113f12d5729f3c8c299` → PASS.

## Findings

- The focused regression table exercises positive and negative 2027-only rules, alternative graduation years, same-line unparsed requirements, a standalone requirement line with unrelated text on a separate line, missing graduation criteria, malformed/missing facts, and unsupported citations. Outcomes stay `unknown` when the parser cannot safely establish a single cited criterion; the explicit isolated mismatch remains `ineligible`.
- Evidence tests require citation spans to match the raw JD byte slice exactly. Ambiguous/no-citation cases return no fabricated `jobEvidence`; known rules retain profile revision, fact revision, confirmation time, source and confirmer.
- Soft evidence tests show `Go` is not matched as a substring of `Google`, while standalone `Go` and `Go` before a non-ASCII Chinese suffix are cited accurately.
- `TestConcurrentChangedEvaluationIntentReturnsHTTPConflict` uses an explicit barrier to ensure a different-intent request passes its initial receipt miss before the competing intent commits. It then verifies the loser returns HTTP 409 `idempotency_conflict`. Repeated 20 times and under Go's race detector, it passed.
- No acceptance gap was found in the requested repair scope.

## Remaining limitations

- PostgreSQL runtime/migration testing remains unavailable (`TRPC_TEST_POSTGRES_DSN` was unset in the prior validation; this repair does not change migration SQL or persistence schema). This is an environment limitation, not a failure in the requested SQLite/backend regression scope.
- This validates the backend repair only; it does not replace independent code review, Web validation or the full T10/#150 completion gate.
