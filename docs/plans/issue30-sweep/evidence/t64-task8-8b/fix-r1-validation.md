# Task 8B R1 backend validation report

## Validation target

- Exact HEAD: `ebb439454823f746734b0ce14184cc413e21877b`
- Checkout: `/Users/wuyongjun/.codex/worktrees/t64-task8-8b/WeKnora-fork01`
- Validation was performed serially; no test processes overlapped.
- No source or test files were edited during validation.

## Commands and results

| # | Exact command | Exit | Result / duration |
|---|---|---:|---|
| 1 | `go test ./internal/application/repository -run '^(TestAgentChatTurnClaimEntityUsesTenantScopedKeysAndRevocationIndex|TestAgentAdoptionRepositoryLifecycle|TestAgentChatTurnClaimMissingPlaceholder.*|TestAgentChatTurnClaimMissingExpiredPlaceholderRollsBackAdmission|TestAgentChatTurnClaimRevocationMissingPlaceholderRollsBackTogether)$' -count=1` | 0 | Passed; `ok github.com/Tencent/WeKnora/internal/application/repository 6.285s`. |
| 2 | `go test ./internal/application/repository -run '^(TestAgentChatTurnClaim|TestRunCancellationReconciliation|TestCancelRunsBySecurityPins|TestAgentAdoption.*Variant|TestAgentSecurityStoreAppendReleaseAndAuditRollsBackTogether)' -count=1` | 0 | Passed; `ok github.com/Tencent/WeKnora/internal/application/repository 19.302s`. |
| 3 | `go test ./internal/application/service -run '^(TestRevokeReleaseReturnsCommittedPendingResultWhenImmediateReconcileFails|TestRevokeDependencyReturnsCommittedPendingResultWhenImmediateReconcileFails)$' -count=1` | 0 | Passed; `ok github.com/Tencent/WeKnora/internal/application/service 5.794s`. |
| 4 | `go test ./internal/application/service -run '^(TestResolvePublishedAgentVersion|TestAgentSecurity.*)$' -count=1` | 0 | Passed; `ok github.com/Tencent/WeKnora/internal/application/service 23.786s`. |
| 5 | `go test ./internal/database -run '^(TestSQLiteMigrationsCreateVersionedSchema|TestTask8ClaimMigrationEmptyDownUpAndPopulatedDownRefusal|TestTask8RunPinsAndPublishedVariantIdentityAreImmutable|TestTask8PostgresMigrationDeclaresTransactionalSecurityGuards)$' -count=1` | 0 | Passed; `ok github.com/Tencent/WeKnora/internal/database 4.550s`. Includes SQLite runtime migration checks and PostgreSQL migration source-contract checks. |
| 6 | `go build ./...` | 0 | Passed. Duration was not captured. Linker warned `ignoring duplicate libraries: '-lc++'` for `cmd/server` and `cmd/desktop`. |
| 7 | `git diff --check cbdb08e627b3466cc419815e3108fe18bb0b3bfc..ebb439454823f746734b0ce14184cc413e21877b` | 0 | Passed; no whitespace errors. |

## Checkout and ownership evidence

After validation, `git rev-parse HEAD` returned `ebb439454823f746734b0ce14184cc413e21877b`. `git status --short --untracked-files=all` returned no entries, so the checkout was clean and HEAD had not moved.

The changed paths in `cbdb08e627b3466cc419815e3108fe18bb0b3bfc..HEAD` were reviewed against the corrected Task 8B R1 Brief. They consist of its Task 8B repository/service/types changes, focused tests, paired migration files, migration-contract tests, and the Task 8B implementation report. No 8C/8D/8E paths were changed. The report path added by this validation is validation evidence and is not a source/test change.

## PostgreSQL limitation

PostgreSQL migration validation here is static SQL source-contract testing only. No PostgreSQL server was available or exercised; this is not evidence of PostgreSQL runtime migration behavior.
