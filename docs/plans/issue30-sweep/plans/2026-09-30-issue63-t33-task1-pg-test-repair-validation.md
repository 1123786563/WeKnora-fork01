# Issue #63 T33 Task 1 PostgreSQL Test Harness Repair Validation

## Revision and scope

Initial validation of commit `3131aaef37e562813e5efb710f087bdddb904ac6` (`3131aaef3`) found that the PostgreSQL DSN was unset and the integration subtest skipped. Follow-up validation on 2026-09-30 used the existing local `WeKnora-postgres-dev` service with the repository's supported per-test schema harness. No application tables or shared test data were used.

## Harness selection review

The tagged race test now wraps its body in `t.Run("postgres", ...)` and asserts `db.Name() == "postgres"`. The shared `openRunTestDB` helper selects PostgreSQL only when `strings.Contains(t.Name(), "/postgres")`; otherwise it creates and migrates a temporary SQLite database. This pins selection to the PostgreSQL harness and makes accidental SQLite selection fail the assertion. With the DSN unset, the outer test exits via an explicit `t.Skip` before calling the helper; it does not silently run SQLite.

## Commands and results

- `go test -tags semantic_integration ./internal/application/repository -run '^TestAgentAdoptionEndWaitsThenSeesCommittedVariant$' -count=1 -v` — **PASS as a command; test explicitly skipped**. Output: `TRPC_TEST_POSTGRES_DSN unset: PostgreSQL READ COMMITTED race evidence blocked-env`.
- `TRPC_TEST_POSTGRES_DSN='postgres://postgres:<URL-encoded local development password>@127.0.0.1:5432/WeKnora?sslmode=disable' go test -tags semantic_integration ./internal/application/repository -run '^TestAgentAdoptionEndWaitsThenSeesCommittedVariant$' -count=1 -v` — **PASS**, including `TestAgentAdoptionEndWaitsThenSeesCommittedVariant/postgres` (2.535s package time). The test entered the PostgreSQL harness, asserted `db.Name() == "postgres"`, observed End's backend PID waiting on the Adoption parent gate, released CreateVariant, and verified End rejected the now-present non-retired Variant. The DSN value is intentionally not persisted.
- `go test ./internal/application/repository -run '^TestAgentAdoptionRepository(EndRequiresRetiredVariants|Lifecycle|RejectsWritesAfterEnd)$' -count=1` — **PASS** (`ok`, 3.418s).
- `go test ./internal/database -run '^(TestAgentMarketplaceLifecycleMigrationSQLiteUpDownUp|TestMigrationVersionsUniquePerTrack)$' -count=1` — **PASS** (`ok`, 1.309s). Covers SQLite migration up/down/up and migration-version uniqueness.
- `git diff --check HEAD^ HEAD` — **PASS** (no whitespace errors).

## Assessment and limitation

The follow-up run selected PostgreSQL and checked the precise End backend, closing the prior environment limitation for this regression. T33-1 still requires independent source review and the full validator/review gates recorded in the lifecycle ledger; this report does not promote the whole task to verified.

Validation status: **PASS for PostgreSQL race harness behavior**; broader T33-1 status remains running pending its independent review and task-level closure.
