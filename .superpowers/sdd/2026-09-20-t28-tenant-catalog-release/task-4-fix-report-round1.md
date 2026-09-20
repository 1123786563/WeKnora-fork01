# Task 4 Fix Report — Review Round 1

Status: DONE
Branch: `marketplace/t28-tenant-catalog-release`

## Findings Addressed

1. **Concurrent approval cleanup:** Final bundle publication now uses `os.Link` from a verified same-directory temporary file, which atomically creates the final path only if it does not exist. An `EEXIST` result causes the service to read and SHA-256 verify the winning object; it is considered pre-existing and is never removed by this attempt. A cross-process `flock` on the tenant/submission/digest path serializes the storage-stage, repository-transaction and compensation window. When a transaction fails, the service queries for a Release by Tenant/submission and removes the final path only when the attempt created it and the successful query confirms no matching Release references it. If the lookup fails, the service conservatively retains the bytes.
2. **Pending catalog visibility:** `ListTenantCatalog` now returns only rows in `listed` state with a non-null current Release pointer. A separate tenant-scoped `GetListing` read lets reviewer workflow load the expected pointer for a pending Listing without exposing it to members. Repository and service tests cover pending visibility.

## TDD Evidence

- **RED — pending Listing visibility:** The updated repository tests failed before the query filter, returning pending Listings with a null pointer from `TestAgentMarketplaceSubmissionIgnoresCallerReleasePointer` and `TestAgentMarketplaceTenantIsolationWithPopulatedTenants`.
- **RED — concurrent compensation:** The concurrent approval test initially could not compile because the exclusive-publication seam was absent. After implementing no-replace publication and persisted-Release lookup, a 20-run stress check exposed the remaining ordering race: one run failed because the final bundle path was missing. This showed that checking for a committed Release alone was insufficient while a peer transaction could still be in flight.
- **GREEN — service:** `go test ./internal/application/service -run TestAgentMarketplace -count=1` → pass (`ok`, 1.249s).
- **GREEN — repository:** `go test ./internal/application/repository -run 'TestAgentMarketplace(Submission|Review|Publish|TenantScope)' -count=1` → pass (`ok`, 2.868s).
- **GREEN — concurrency:** `go test ./internal/application/service -run TestAgentMarketplaceConcurrentApprovalFailureKeepsPeerCommittedBundle -count=20` → pass (`ok`, 2.317s).
- **Diff hygiene:** `git diff --check` → pass.

## Test Coverage Added or Updated

- Two simultaneous service approval calls produce one successful Release and one competing transaction failure; the final digest file remains present and matches the Release bytes and digest.
- An `EEXIST` publication verifies the existing object's digest before permitting the transaction to continue; corrupt existing bytes reject approval.
- A transaction failure removes a path created by the failed attempt when no Release references it.
- A pending Listing is absent from the catalog while remaining available through the internal tenant-scoped Listing lookup.
- Tenant-isolation assertions now expect both tenants' pending Listings to be absent from the catalog before approval.
- The repository submission test verifies `GetReleaseBySubmission` returns the committed Release.

## Changed Files

- `internal/application/service/agent_marketplace.go`
- `internal/application/service/agent_marketplace_test.go`
- `internal/application/repository/agent_marketplace.go`
- `internal/application/repository/agent_marketplace_test.go`
- `internal/types/interfaces/agent_marketplace.go`
- `.superpowers/sdd/2026-09-20-t28-tenant-catalog-release/task-4-fix-report-round1.md`

## Remaining Risk

- The storage serialization uses POSIX `flock`; the shared filesystem must honor advisory file locks. Database transactions and pointer CAS remain authoritative for persisted publication state.
- PostgreSQL behavior was not independently exercised in this fix round; focused service and repository verification uses local files and the repository's SQLite-backed tests.
