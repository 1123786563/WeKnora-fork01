# Task 3 Fix Report — Review Round 1/5

Status: DONE (all five Important findings addressed)
Branch: `marketplace/t28-tenant-catalog-release`
Commit: `fix(marketplace): close Task 3 review findings` (SHA reported to controller)

## Findings addressed

1. **Release number contention and bounded retry.** `ReviewAndPublishTx` now retries the whole transaction up to five attempts when it sees the named per-Listing release-number unique collision or transient lock/serialization contention. Each attempt rereads the Submission and `MAX(release_number)` in the transaction; the existing expected-prior pointer CAS remains unchanged. Retries use bounded incremental backoff and stop on context cancellation. Other integrity violations and stale-pointer CAS errors are returned without retry.
2. **Caller supplied Listing pointer.** First Listing creation now copies only the display name and summary. ID, Tenant, source Agent, state, timestamps and `CurrentReleaseID` are server supplied; a new Listing always starts with a nil pointer. A regression test passes a forged pointer and verifies the new Listing is empty.
3. **AgentVersion/source Agent binding.** Submission creation now requires a version matching `(tenant_id, id, agent_id)` before making a Listing or Submission. Both dialect migrations add the referenced unique key and a composite Submission foreign key. Releases also persist and constrain the same source Agent binding. A same-Tenant mismatched AgentVersion test fails closed.
4. **Populated SQLite rollback.** SQLite 000100 down first clears Listing pointers, then drops review, release, submission and listing tables, and finally removes the source-binding index. A migration-backed test publishes a Release with `PRAGMA foreign_keys=1`, runs the down SQL, and checks all four Marketplace tables are removed while `agent_versions` remains.
5. **Populated Tenant isolation.** The new isolation test creates versions, listings, submissions and Releases for two Tenants with the same source Agent ID. Before and after review it verifies each Tenant sees only its own queue/catalog entries; `GetRelease` returns nil for cross-Tenant IDs and resolves the owned Release.

The deferred Minor finding about nonempty reviewer ID / caller-chosen initial Submission status was not changed, per controller direction.

## RED → GREEN evidence

- **RED:** `go test ./internal/application/repository -run 'TestAgentMarketplace(SubmissionIgnores|SubmissionRequires|TenantIsolationWith|SQLiteDownWith|ConcurrentPublishing)' -count=1` → failed as expected: forged Listing pointer hit the FK, mismatched AgentVersion/source Agent was accepted, populated down failed with FK enforcement, and two-connection publication surfaced `database is locked`.
- **GREEN:** After the fixes, the covering focused command passed:
  `go test ./internal/application/repository -run 'TestAgentMarketplace(Submission|Review|Publish|TenantScope|TenantIsolationWith|SQLiteDownWith|ConcurrentPublishing)' -count=1`
  Result: `ok github.com/Tencent/WeKnora/internal/application/repository`.
- **Concurrency stability:**
  `go test ./internal/application/repository -run TestAgentMarketplaceConcurrentPublishingUsesUniqueReleaseNumbers -count=10`
  Result: `ok github.com/Tencent/WeKnora/internal/application/repository`.
  The two-connection test verifies exactly one contender advances the initial CAS, the other can publish using the new pointer, final release numbers are exactly 1/2/3 with no duplicate, the Listing points to Release 3, and Release 1 bytes remain unchanged.
- **Migration gate:**
  `go test ./internal/database -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1`
  Result: `ok github.com/Tencent/WeKnora/internal/database`.
- `git diff --check` passed.

## Files changed

- `internal/application/repository/agent_marketplace.go`
- `internal/application/repository/agent_marketplace_test.go`
- `internal/database/migration_sqlite_versioned_schema_test.go`
- `internal/types/agent_marketplace_persistence.go`
- `migrations/versioned/000179_tenant_agent_marketplace.up.sql`
- `migrations/versioned/000179_tenant_agent_marketplace.down.sql`
- `migrations/sqlite/000100_tenant_agent_marketplace.up.sql`
- `migrations/sqlite/000100_tenant_agent_marketplace.down.sql`

## Residual concern

The concurrency coverage uses two SQLite connections and exercises lock contention plus the transaction retry/CAS path. It does not force a PostgreSQL unique-index collision deterministically; PostgreSQL migration execution and that exact collision path remain unverified locally. The retry classifier handles the named PostgreSQL constraint and SQLite’s release-number unique-violation signature, and the database unique index remains the correctness boundary.
