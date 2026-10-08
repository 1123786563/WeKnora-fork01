# Issue 140 Task 13 — Durable Career Lifecycle Gate

## Scope and baseline

- Task: Task 13 from `docs/superpowers/plans/2026-09-30-issue140-career-review-repairs-r2.md`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue140-task13-gate`.
- BASE: `1ff972596d8a3ce619698b96911fe444c3bf574a`.
- Migration mapping: append SQLite 143 and versioned/PostgreSQL 222. Existing #30 and #140 IDs remain unchanged.
- Task 1's same-Office mutex was not present at this BASE; Task 13 adds the database-shared gate needed across Office instances. Integration should keep Task 1's local synchronization as an additional optimization and rerun these tests after integration.
- The legacy `search_rule.go` is unchanged. `admitLifecycleClaimTx` is the transaction seam for Task 17 to atomically add lifecycle admission alongside its rule-period claim.

## Implementation evidence

- Added retained per-scope lifecycle phase with a bound deletion request ID and fingerprint. A deletion request records its intent while draining; unresolved claims return retryable `ErrCareerOperationsBusy`, stop new external operations, and allow only original-claim recovery. Active claims have no time-based expiry.
- Added durable operation claims keyed by scope, operation and request ID, with intent fingerprint checks. Application link creation and reconciliation keep claims until a ready/failed durable receipt is recorded. Unknown linker results retain the claim for original-request reconciliation.
- Material publish admits before its first object write and derives stable export IDs from scope, request ID and fingerprint. Unknown object-store outcomes retain the claim; replay writes the same object names. A committed export receipt resolves a stale claim on replay. Claim release errors are returned as outcome-unknown.
- Career deletion transitions to `deleting` only after claims drain, retains that phase and the original deletion identity through the terminal receipt, and rejects other deletion requests. It removes revoked as well as active export objects and fails closed when an object/source cleanup adapter is missing.
- Full career export now carries each stored preparation receipt, including draft body, sources and submitted-version anchor. Deletion-boundary count query failures now return an error instead of reporting zero.
- Added paired migration files at `migrations/sqlite/000143_career_lifecycle_gate.{up,down}.sql` and `migrations/versioned/000222_career_lifecycle_gate.{up,down}.sql`. The repository ignores `migrations/`; these four task-owned files must be force-added to Git.

## Tests and commands

- `go test -timeout=90s -count=1 ./internal/career -run 'TestLifecycleGate|TestLifecycleClaim|TestTwoOfficesDeletionWaits|TestReconcileApplicationLinkKeepsClaim|TestMaterialPublishReplaysSameObjects|TestDeleteCareerRemovesPreviouslyRevokedExportObjects|TestDeleteCareerFailsClosedWhenExportStorageUnavailable|TestCareerDeletionBoundaryReturnsCountQueryErrors|TestExportCareerArchiveCarriesPreparationsSearchRulesAndReminders'` — PASS.
- `go test -count=1 ./internal/database -run 'TestMigrationVersionsUniquePerTrack|TestCareerLifecycleGateMigrationOpensPersistedSQLiteSchema|TestCareerExportMigrationUpAndDown|TestCareerOfficeOpensAfterVersionedSQLiteMigration'` — PASS. The full SQLite migration chain reaches version 143; the migrated database opens through `career.NewOffice` with gate columns present.
- `go test -count=1 ./internal/career ./internal/workbench/service/workbench ./internal/container ./internal/database` — PASS for all four packages.
- `go test -race -count=1 ./internal/career -run 'TestLifecycleGate|TestLifecycleClaim|TestTwoOfficesDeletionWaits|TestReconcileApplicationLinkKeepsClaim|TestMaterialPublishReplaysSameObjectsAfterUnknownStorageOutcome'` — PASS.
- `git diff --check` — PASS.
- Two-Office tests pause the Workbench linker, application reconciliation lookup and material storage write while a second Office attempts deletion. Deletion remains busy until the original operation reaches a durable outcome; recovered publish reuses stable object keys; a stale post-receipt claim is resolved by exact replay.

## Limits

- PostgreSQL migration/runtime integration was not run in this worktree. The versioned migration is append-only at 222 and uses a short transaction/row lock over the same gate row, but this still needs the configured PostgreSQL integration job.
- Task 17 must call `admitLifecycleClaimTx(tx, scope, operation, requestID[, fingerprint])` inside its rule-period claim transaction; this stale BASE did not include Task 11's started-period protocol, and its `search_rule.go` was deliberately left unchanged.
- Final integration must include Task 1 before asserting end-to-end deletion closure, then rerun the Career, Workbench and container checks against the combined tree.
