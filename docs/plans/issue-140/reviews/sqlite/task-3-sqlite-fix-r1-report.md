# T03 SQLite startup fix round 1 report

**Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t03-sqlite-fix/WeKnora-fork01`
**Starting commit:** `21d45eb799f1d66e73504220835289f566aeb7e1`
**Scope:** Round-one review findings F1 (schema verification) and F2 (partial schema handling).

## Changes

`career.NewOffice` now handles SQLite schema state in three explicit cases:

- No Career tables: retain the AutoMigrate compatibility path for a fresh database.
- Some but not all seven Career tables: fail startup with an actionable `incomplete Career SQLite schema` error; do not run AutoMigrate over the mixed schema.
- All seven tables: verify the required columns in every Career table and verify the ordered composite unique constraints `(tenant_id,user_id,key)`, `(tenant_id,user_id,revision)`, `(tenant_id,user_id,request_id)`. Missing schema details fail initialization with table/column or constraint details.

PostgreSQL retains its original AutoMigrate path. The SQLite migration API used by tests (`RunMigrationsWithOptions`) applies the full pending stream and does not expose a target-version option, so it cannot directly test a paused-at-000112 then 000113 sequence without building a separate migration harness. Existing integration test applies the actual full SQLite migration stream through current head before opening Career.

## TDD and verification evidence

- RED: `go test ./internal/database -run 'TestCareerOfficeRejects(Malformed|Partial)VersionedSQLiteSchema' -count=1` failed as expected: malformed seven-table schemas returned nil; a partial set re-entered AutoMigrate and emitted the known `career_facts__temp has no column named UNIQUE` failure.
- GREEN: `go test ./internal/database -run 'TestCareerOffice(Rejects(Malformed|Partial)VersionedSQLiteSchema|OpensAfterVersionedSQLiteMigration)|TestCareerMigrationCreatesPersonalEvidenceSchema' -count=1` passed.
- `go test ./internal/modules/career/... ./internal/database/... ./internal/router/...` passed.
- `git diff --check` passed.

## Changed files

- `internal/modules/career/office.go`
- `internal/database/career_migration_test.go`

## Remaining limits

The initialization guard validates schema shape and the three required composite unique constraints directly; it does not read a migration version number because that table name can be customized by the migration DSN and the migration runner owns migration-history validation. This ensures a complete-looking schema cannot bypass required Career data-integrity guarantees.
