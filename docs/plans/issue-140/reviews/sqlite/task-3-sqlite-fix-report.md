# T03 SQLite startup integration fix report

**Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t03-sqlite-fix/WeKnora-fork01`
**Branch / base:** `codex/issue-140-t03-sqlite-fix` / `a4bb32ed3`
**Scope:** #140 T03/#141 SQLite startup integration only.

## Finding and change

The SQLite versioned migration creates `career_facts`, `career_changes`, and `career_receipts` with table-level `UNIQUE (...)` constraints. `career.NewOffice` then called GORM `AutoMigrate` during container/router initialization. GORM's SQLite schema introspection misread the table-level clause as a column while rebuilding `career_facts`, producing `table career_facts__temp has no column named UNIQUE`.

`NewOffice` now detects a complete set of the seven versioned SQLite Career tables and skips the redundant rebuild on SQLite. It retains AutoMigrate when the schema is absent and retains AutoMigrate on non-SQLite dialects. The regression test applies the actual repository SQLite migration stream, initializes Career twice (startup/restart), claims a scoped space, and asserts uniqueness remains enforced for fact scope/key, change revision, and receipt scope/request ID.

## TDD and verification evidence

- RED: `go test ./internal/database -run TestCareerOfficeOpensAfterVersionedSQLiteMigration -count=1` failed as expected with `table career_facts__temp has no column named UNIQUE` at `career.NewOffice`.
- GREEN: the same targeted test passed after the initialization change.
- `go test ./internal/modules/career/... ./internal/database/... ./internal/router/...` passed (`career`, `database`, and `router`).
- `git diff --check` passed.
- A bounded actual startup was attempted with a fresh `/tmp` path and private port: `DB_DRIVER=sqlite DB_PATH=/tmp/weknora-t03-startup-proof-retry.db APP_PORT=18089 GIN_MODE=release timeout 25s go run ./cmd/server`. Startup proceeded past SQLite migration and container/module construction without the Career `UNIQUE` error, then exited status 1 before serving. The captured output has no explicit fatal cause; the final startup milestone was Craft lifecycle startup. Therefore actual HTTP startup/authenticated probes remain unverified and require parent integration/runtime configuration.

## Changed files

- `internal/modules/career/office.go`
- `internal/database/career_migration_test.go`

## Risks / remaining checks

- The complete-schema check currently verifies the presence of all seven Career tables, not every column/index. A partially migrated hand-managed SQLite schema continues down AutoMigrate and may still encounter the SQLite parser issue; versioned migration databases are the supported path tested here.
- The actual server did not remain available for HTTP validation. Follow-up should resolve its separate startup exit and verify startup plus authenticated `/api/v1/career/open` against a disposable DB.
