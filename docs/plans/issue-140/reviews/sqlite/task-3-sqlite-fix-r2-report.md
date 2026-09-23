# T03 SQLite startup fix round 2 report

**Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t03-sqlite-fix/WeKnora-fork01`
**Starting commit:** `3ac1b3adf7ffda75436a90783a712ec3d2ddd2ed`
**Scope:** Round-two review finding R1-F1: reject partial unique indexes at the Career SQLite startup guard.

## Changes

`requireSQLiteUniqueConstraint` now reads SQLite's `partial` index-list flag and ignores partial unique indexes when matching a required composite key. The malformed schema regression replaces the full fact uniqueness constraint with an otherwise matching partial unique index and successfully inserts two duplicate-key rows excluded by that index predicate; `NewOffice` must reject this schema. The clean migration startup regression now closes the original SQLite pool, opens a new GORM connection to the same fully migrated file, and initializes Career again.

The migration helper still only runs all pending migrations via `RunMigrationsWithOptions` and offers no version target. Thus a test paused at 000112 and continued to 000113 would require a custom migration harness outside this scoped change; this remains documented as a limitation.

## TDD and verification evidence

- RED: `go test ./internal/database -run 'TestCareerOfficeRejectsMalformedVersionedSQLiteSchema/partial_fact_uniqueness' -count=1` failed as expected because a matching partial index was accepted.
- GREEN: `go test ./internal/database -run 'TestCareerOffice|TestCareerMigrationCreatesPersonalEvidenceSchema' -count=1` passed, covering full-migration reopen, malformed missing-column/missing-constraint/partial-index cases, partial table rejection, and migration schema.
- `go test ./internal/modules/career/... ./internal/database/... ./internal/router/...` passed.
- `git diff --check` passed.

## Changed files

- `internal/modules/career/office.go`
- `internal/database/career_migration_test.go`

## Remaining limits

The full migration stream and a fresh connection/reopen are covered. The migration stream is not paused specifically between 000112 and 000113 because the production helper has no target-version API. PostgreSQL behavior remains unchanged; the SQLite-specific verification is selected by dialect name.
