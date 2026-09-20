# Task 4 Report: Author and Reviewer Service Transitions

Status: DONE
Branch: `marketplace/t28-tenant-catalog-release`

## Implementation

- Added `AgentMarketplaceService` for release submission, review queue reads, review decisions, Tenant catalog reads and tenant-scoped Release lookup.
- Submission reads the tenant-scoped immutable `AgentVersionSnapshot`, resolves dependencies from that snapshot, builds the canonical sanitized bundle once, verifies that every payload Skill and subagent reference has a matching typed Dependency Lock entry, and persists the fixed Manifest, lock, bundle bytes and digest through `CreateSubmission`.
- Review decisions replace any caller-provided reviewer ID with the authenticated actor ID, require a reason for rejection or changes requested, compare the submitted digest, and verify the bundle bytes against that digest before storage or transaction work.
- Approval writes and re-reads a temporary file, verifies its SHA-256, then atomically renames it to `tenant-{id}/releases/{submission-id}/{digest}` beneath the configured bundle root. The repository transaction then appends the Review and Release and advances the expected Listing pointer. If the transaction fails, the service removes only the final object created by this attempt. Rejection does not stage a bundle.
- Added tenant-scoped `GetSubmission` and `GetRelease` methods to the repository contract/implementation so the service can bind review to immutable persisted submission bytes and expose all five service operations.
- No legacy `PublishedExpert` row or `PublishedSnapshotDir` is used.

## TDD Evidence

- **RED:** The first `go test ./internal/application/service -run TestAgentMarketplace -count=1` failed on the missing `NewAgentMarketplaceService` API and transition methods, as expected before implementation. Compiler output also exposed two fixture symbol mistakes, corrected before writing production behavior.
- **GREEN:** `go test ./internal/application/service -run TestAgentMarketplace -count=1` → pass (`ok`, 1.182s).
- **Repository regression check:** `go test ./internal/application/repository -run 'TestAgentMarketplace(Submission|Review|Publish|TenantScope)' -count=1` → pass (`ok`, 2.539s).
- **Service package suite:** `go test ./internal/application/service` → pass (`ok`, 55.639s).
- **Diff hygiene:** `git diff --check` → pass.

## Tests Added

- Submission uses the frozen Agent Version and persists Manifest, license and both locked payload references.
- A missing subagent Dependency Lock entry causes submission to fail. This specifically proves the lock covers every payload Skill and subagent reference.
- Review requires a rejection reason and rejects a stale digest before any decision is persisted.
- Approval stages verified bundle bytes, returns the immutable Release, and an exact retry returns the same Review/Release without changing the bytes.
- Database transaction failure removes the newly staged object.

## Files Changed

- `internal/application/service/agent_marketplace.go`
- `internal/application/service/agent_marketplace_test.go`
- `internal/types/interfaces/agent_marketplace.go`
- `internal/application/repository/agent_marketplace.go`
- `.superpowers/sdd/2026-09-20-t28-tenant-catalog-release/task-4-report.md`

## Self-Review and Remaining Risk

- Reviewer identity is taken from the service actor argument; Admin/Owner role enforcement and author edit permission remain at the authenticated route boundary per the approved plan.
- The repository transaction is the database atomicity boundary. The service computes the Listing's expected current Release before staging; the transaction's pointer compare-and-swap rejects a concurrent advance and triggers staged-file compensation.
- Staged file paths are deterministic from Tenant, Submission and digest, while Release bundle bytes also remain persisted in the database. The current API does not expose filesystem paths as public data.
- PostgreSQL transaction behavior and concurrent filesystem staging were not independently exercised in this Task; repository integration evidence remains the Task 3 SQLite suite.
