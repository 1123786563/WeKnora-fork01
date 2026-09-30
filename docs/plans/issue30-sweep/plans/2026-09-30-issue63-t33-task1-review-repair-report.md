# Issue #63 T33 Task 1 Review Repair Report

## Scope and findings

Repaired independent review findings F1 (High), F2 (High), and F3 (Medium) from `2026-09-30-issue63-t33-task1-independent-review.md`.

- Tenant Adoption now gates `AdoptListing` against the tenant-owned Listing in `listed` state and verifies that the accepted tenant Release belongs to that Listing and tenant.
- Variant creation and Adoption reconciliation use a tenant-scoped conditional no-op update on the active Adoption row. `EndAdoption` updates that same parent row, serializing create/end and reconcile/end. Reconciliation rejects ended rows even when the Release ID is unchanged; pointer advancement is an active-state CAS with an affected-row check.
- Public introduction first conditionally updates the platform Listing in `listed` state, validates the persisted Release belongs to that Listing, then accesses the adopter Adoption and introduction ledger. `UnlistPublicListing` updates the same Listing row. Lock order is Listing → Adoption.
- No migration or API changes were needed. SQLite required no retry wrapper in the tested paths.

## Tests and evidence

RED evidence: before repository changes, the new regression tests failed because `CreateVariant` accepted an ended Adoption and `IntroduceRelease` succeeded after public Unlist. After the gates were added:

- `go test ./internal/application/repository -run 'Test(AgentAdoption|AgentLifecycle|PublicMarketplace).*Lifecycle|Test.*Unlist.*Adopt|Test.*End.*Variant' -count=1 -v` — PASS.
- `go test ./internal/application/repository -run 'TestAgentAdoptionRepositoryRejectsAdoptionAfterTenantUnlist|TestAgentAdoptionRepositoryRejectsWritesAfterEnd|TestPublicMarketplaceRepositoryRejectsIntroductionAfterUnlist' -count=1 -v` — PASS. Asserts no tenant Adoption is created after tenant Unlist, no public Introduction or Adoption is created after public Unlist, direct Variant creation after End fails, and same/different Release re-introduction after End fails.
- `go test ./internal/database -run 'TestAgentMarketplaceLifecycleMigrationSQLiteUpDownUp|TestMigrationVersionsUniquePerTrack' -count=1 -v` — PASS.
- `git diff --check` — PASS.
- `TRPC_TEST_POSTGRES_DSN` — unavailable. PostgreSQL row-lock winner-order behavior was not exercised; SQLite results are not claimed as proof of PostgreSQL locking.
- `go test ./internal/application/repository -count=1` was also invoked; the command wrapper returned after 30.4s without captured output or an exit code, so this is not counted as a pass.

## Remaining limits

The tests cover the end-first and unlist-first sequential rejection cases and verify final rows. This environment did not permit an actual PostgreSQL contention test, so production PostgreSQL lock behavior still requires validation where the configured DSN exists. No SQLite busy error occurred, so no transaction retry was added.

## Checkpoint

- Implementation commit: `868c8da752494db8d8477292e67a9a1ed09f9b06`.
- SHA-256 of the binary patch for the four repository implementation/test files in that commit: `6d6214cca8196dadb06157567dc38a88a6e060ca9c7d147de6777ce56b5e62aa`.
- Review status: awaiting independent review and backend validation; downstream T33 tasks remain pending.
