# Issue #63 T33-1 Task Report

## Result

Implemented durable lifecycle metadata and repository primitives for tenant Variant retirement, Adoption end, tenant/public Listing unlisting, and tenant/public Release deprecation. Added the tenant-scoped `AgentTaskAdmission` lookup (`IsRetiredMarketplaceAgent`), which returns false for ordinary local Agents without marketplace Variant rows.

Lifecycle writes use compare-and-set predicates and persist actor, UTC time, and reason. Ending an Adoption additionally requires that every Variant is retired. Release replacement IDs must refer to a different Release in the same lane Listing. Tenant and Public Release lifecycle fields are separate; public Releases remain keyed by their platform Release ID and do not acquire a tenant ID. The transition updates lifecycle columns only, leaving bundle bytes/digest untouched.

## Checkpoint

- Worktree/branch: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t63`, `codex/issue30-b6-t63`.
- Base: `c7c3580eeb6f33107fdd37576b74f36c8fbe7341`.
- Implementation commit: `24c19ee1c476e9eeaa98df1796ed85dd91b8baa7`.
- Review package: `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-review-package.patch`.
- Review package SHA-256: `eb3ab370aa840b7572746af312aa1b24a936100006fedd01613fd215b88583da`.
- Migration versions selected after checking the active tracks: SQLite `000124`; versioned `000203`.
- Independent task review remains pending; T33-1 is not yet marked verified.

## RED / GREEN and verification evidence

- RED: `go test ./internal/application/repository -run '^TestAgentLifecycle' -count=1` initially failed at compile time because the lifecycle methods and fields did not exist.
- GREEN: `go test ./internal/application/repository -run '^TestAgentLifecycle' -count=1` — PASS.
- `go test ./internal/application/repository -run 'TestAgentLifecycle|TestAgentAdoptionRepository|TestAgentMarketplace|TestPublicMarketplaceRepository' -count=1` — PASS (`9.216s`). Includes tenant and public CAS, tenant isolation/not-found behavior, invalid/repeated transitions, concurrent retirement CAS, Agent admission lookup, release byte/digest preservation, and existing marketplace/adoption repository cases.
- `go test ./internal/database -run 'TestAgentMarketplaceLifecycleMigrationSQLiteUpDownUp|TestMigrationVersionsUniquePerTrack|TestSQLiteMigrationsCreateVersionedSchema' -count=1` — PASS (`1.457s`). SQLite lifecycle columns pass up/down/up; the versioned migration is checked for the matching column set; migration version uniqueness and full SQLite schema creation pass.
- `git diff --cached --check -- . ':!docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-review-package.patch'` — PASS. The package artifact is a unified diff, whose context lines trigger `git diff --check` whitespace diagnostics when checked as a source file.
- A broader `go test ./internal/application/repository -count=1` attempt was stopped after running over two minutes because the package includes lengthy unrelated integration cases. It did not complete and is not counted as passing evidence.

## Changed files

- `internal/types/agent_adoption_persistence.go`
- `internal/types/agent_marketplace_persistence.go`
- `internal/types/public_marketplace_persistence.go`
- `internal/application/repository/agent_adoption.go`
- `internal/application/repository/agent_marketplace.go`
- `internal/application/repository/public_marketplace.go`
- `internal/application/repository/agent_lifecycle_test.go`
- `internal/database/agent_marketplace_lifecycle_migration_test.go`
- `migrations/sqlite/000124_agent_marketplace_lifecycle.{up,down}.sql`
- `migrations/versioned/000203_agent_marketplace_lifecycle.{up,down}.sql`

## Limits

- PostgreSQL migration execution was not run because no `TRPC_TEST_POSTGRES_DSN` was provided. Versioned-track parity is covered by matching DDL column assertions; SQLite up/down/up is exercised against the real migration stream.
- No service, handler, router, or Task admission wiring was changed. Those remain downstream T33 tasks.
