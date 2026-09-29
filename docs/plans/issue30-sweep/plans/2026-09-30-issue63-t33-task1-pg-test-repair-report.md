# Issue #63 T33 Task 1 PostgreSQL Test Harness Repair Report

## Result

Repaired only the tagged concurrency test harness. The test body now runs under `t.Run("postgres", ...)`, causing the existing `openRunTestDB` helper to select PostgreSQL, and immediately asserts `db.Name() == "postgres"`. The End connection is pinned to one `*sql.Conn`; its `pg_backend_pid()` is captured and the wait poll only accepts that PID with `wait_event_type = 'Lock'` and a query targeting `agent_adoptions`. A `sync.Once` release function is registered with `t.Cleanup` to unblock CreateVariant on all exits.

## Verification evidence

- `TRPC_TEST_POSTGRES_DSN`: unset.
- `go test -tags semantic_integration ./internal/application/repository -run '^TestAgentAdoptionEndWaitsThenSeesCommittedVariant$' -count=1 -v` — passed with explicit skip: `TRPC_TEST_POSTGRES_DSN unset: PostgreSQL READ COMMITTED race evidence blocked-env`. The PostgreSQL dialect assertion compiled but could not execute without the DSN.
- `go test ./internal/application/repository -run '^TestAgentAdoptionRepository(EndRequiresRetiredVariants|Lifecycle|RejectsWritesAfterEnd)$' -count=1` — passed.
- `go test ./internal/database -run '^(TestAgentMarketplaceLifecycleMigrationSQLiteUpDownUp|TestMigrationVersionsUniquePerTrack)$' -count=1` — passed.
- `git diff --check` — passed.

## Remaining limitation

The corrected test now selects PostgreSQL and checks the precise End backend when `TRPC_TEST_POSTGRES_DSN` is configured, but this environment did not provide the DSN. PostgreSQL concurrency acceptance remains unverified pending execution in a configured environment.
