# Issue #63 T33 PostgreSQL PID Binding Repair Report

## Changes

Updated only the tagged PostgreSQL concurrency test. Inspection of installed `gorm.io/driver/postgres@v1.6.0/postgres.go` confirmed `Initialize` assigns `db.ConnPool = dialector.Conn`; `gorm.Config.ConnPool` alone is overwritten when the dialector's `Conn` is nil. The test now opens the observer handle with `postgres.New(postgres.Config{Conn: conn})`, queries `pg_backend_pid()` through that GORM handle, and fails unless it equals the PID captured from the pinned `*sql.Conn` before the repository call. The lock wait remains scoped to this PID and checks Lock wait plus the Adoption table query.

The test also registers cleanup that idempotently releases the paused CreateVariant callback and waits up to 15 seconds for both CreateVariant and EndAdoption goroutines. This cleanup runs before the registered connection and callback cleanup due to `t.Cleanup`'s LIFO order.

## Verification

- `TRPC_TEST_POSTGRES_DSN`: unset.
- `go test -tags semantic_integration ./internal/application/repository -run '^TestAgentAdoptionEndWaitsThenSeesCommittedVariant$' -count=1 -v` — passed with explicit skip because DSN is unset. The dialect/PID assertions compile but could not execute here.
- `go test ./internal/application/repository -run '^TestAgentAdoptionRepository(EndRequiresRetiredVariants|Lifecycle|RejectsWritesAfterEnd)$' -count=1` — passed.
- `go test ./internal/database -run '^(TestAgentMarketplaceLifecycleMigrationSQLiteUpDownUp|TestMigrationVersionsUniquePerTrack)$' -count=1` — passed.
- `git diff --check` — passed.

PostgreSQL contention evidence remains pending an environment with `TRPC_TEST_POSTGRES_DSN` configured.
