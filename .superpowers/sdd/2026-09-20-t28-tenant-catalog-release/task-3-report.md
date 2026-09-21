# Task 3 Report: Persist Submissions, Reviews, Releases and Tenant Listings

Status: DONE
Branch: `marketplace/t28-tenant-catalog-release`
Commit: `feat(marketplace): persist submissions reviews releases and listings` (SHA reported by the task runner)

## What was implemented

- Added persistence entities for Tenant Marketplace Listing, immutable Release Submission, append-only Review, and immutable Release, with explicit table names and fields for source AgentVersion, fixed Manifest/Dependency Lock JSON, bundle bytes/digest, review metadata, and current Release pointer.
- Added the application-facing `AgentMarketplaceRepository` contract and the GORM repository implementation:
  - `CreateSubmission` finds or creates one Listing per Tenant/source Agent and appends a Submission in a transaction.
  - `ListReviewQueue` is tenant-scoped and excludes already-reviewed submissions.
  - `ReviewAndPublishTx` reloads the tenant-scoped Submission and compares the fixed digest, appends a Review, and on approval allocates the next per-Listing Release number and inserts a Release. A pointer compare-and-swap advances only the expected Listing; any mismatch rolls the transaction back. Rejection appends only a Review.
  - Exact decision retries return the already persisted Review/Release. A different decision for an already-reviewed Submission is rejected.
  - Submission, Review and Release rows are never updated or deleted by this repository. Only the Listing pointer changes during publication; no keyed mutex is used.
  - `ListTenantCatalog` and `GetRelease` apply Tenant predicates.
- Added PostgreSQL migration 000179 and SQLite migration 000100, with corresponding downs. Both schemas use tenant-composite foreign keys for AgentVersion, Listing, Submission and Release links; indexes enforce one Listing per Tenant/source Agent, unique reviewer/digest decisions, and unique per-Listing Release number, semantic version and digest. The Listing pointer references a Release.
- Extended the SQLite migration schema parity matrix for the four new tables and the shared named uniqueness indexes.
- Added repository tests for digest mismatch, pointer CAS conflict, append-only review retry behavior, rejected review without Release, monotonically increasing per-Listing Release numbers, preservation of older Release bytes, and tenant-scoped queue/catalog/release reads.

## TDD evidence

- **RED (initial API absence):**
  `go test ./internal/application/repository -run 'TestAgentMarketplace(Submission|Review|Publish|TenantScope)' -count=1` → exit 1; compile errors identified the missing `NewAgentMarketplaceRepository`, persistence entities, decision type, and error values.
- **RED (append-only behavior):**
  After adding an assertion that review must leave Submission status unchanged, the same command failed with expected `submitted`, actual `released`. This exposed an implementation that mutated Submission state.
- **GREEN:** Removed the Submission update, made queue visibility derive from appended Review records, and reran the same command → exit 0.
- **Migration gate:**
  `go test ./internal/database -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1` → exit 0.
- **Final focused verification:** Both exact commands above passed after the final repository changes; `git diff --check` passed.
- **SQLite down smoke check:** concatenated `000100` up/down scripts piped to `sqlite3 :memory:` → exit 0.

## Files changed

- `internal/types/agent_marketplace.go` — corrected persistence entity ownership comment.
- `internal/types/agent_marketplace_persistence.go` — new persistence entities and review decision value.
- `internal/types/interfaces/agent_marketplace.go` — new repository contract.
- `internal/application/repository/agent_marketplace.go` — repository transaction and scoped reads.
- `internal/application/repository/agent_marketplace_test.go` — migration-backed repository tests.
- `migrations/versioned/000179_tenant_agent_marketplace.up.sql`
- `migrations/versioned/000179_tenant_agent_marketplace.down.sql`
- `migrations/sqlite/000100_tenant_agent_marketplace.up.sql`
- `migrations/sqlite/000100_tenant_agent_marketplace.down.sql`
- `internal/database/migration_sqlite_versioned_schema_test.go` — SQLite schema parity assertions.

## Self-review and remaining risk

- Publication inserts Review and Release before the pointer CAS inside the same database transaction. A digest mismatch, unique constraint violation, or stale pointer therefore leaves no partial Review/Release/pointer state.
- Release number allocation uses a transactional `MAX + 1` read, with database uniqueness as the concurrency guard; no process-local lock establishes correctness. Under a true concurrent approval race, one transaction may receive a unique-constraint error instead of transparently retrying with the next number. No multiprocess concurrency test was added in this task.
- SQLite migration up/down files are paired and up/parity/upgrade paths passed the specified migration gate. PostgreSQL migration execution was not available in this task environment; PostgreSQL SQL was reviewed against the corresponding SQLite schema and existing 000178 composite-key layout.
